// builder package implements the generic zbuild Builder, which is responsible
// of building specialized images from generic build definitions. It's also
// responsible of other generic operations involving specialized kind handlers.
package builder

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/NiR-/zbuild/pkg/builddef"
	"github.com/NiR-/zbuild/pkg/defloader"
	"github.com/NiR-/zbuild/pkg/image"
	"github.com/NiR-/zbuild/pkg/llbutils"
	"github.com/NiR-/zbuild/pkg/pkgsolver"
	"github.com/NiR-/zbuild/pkg/registry"
	"github.com/NiR-/zbuild/pkg/statesolver"
	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/exporter/containerimage/exptypes"
	"github.com/moby/buildkit/frontend/gateway/client"
	"github.com/twpayne/go-vfs"
	"golang.org/x/xerrors"
	"gopkg.in/yaml.v2"
)

// These consts come from: https://github.com/moby/buildkit/blob/master/frontend/dockerfile/builder/build.go
const (
	keyTarget        = "target"
	keyContext       = "context"
	keyDockerContext = "contextkey"
	keyFilename      = "filename"
)

// Builder takes a KindRegistry, which contains all the specialized handlers
// supported by zbuild. It's used to execute generic operations for specialized
// build definitions, by calling the appropriate kind handlers' methods.
// It also contains a set of PackageSolvers and a filesystem abstraction, used
// during locking.
type Builder struct {
	Registry   *registry.KindRegistry
	PkgSolvers pkgsolver.PackageSolversMap
	Filesystem vfs.FS
}

func (b Builder) findHandler(
	kind string,
	solver statesolver.StateSolver,
	shouldEmbedWebserverDef bool,
) (registry.KindHandler, error) {
	handler, err := b.Registry.FindHandler(kind)
	if err != nil {
		return handler, err
	}
	handler.WithSolver(solver)

	if shouldEmbedWebserverDef && !b.Registry.EmbedWebserverDef(kind) {
		err = xerrors.Errorf("you can't call a webserver stage from a %s kind as it doesn't embed webserver definition", kind)
	}

	return handler, err
}

func buildOptsFromBuildkitOpts(c client.Client) (builddef.BuildOpts, error) {
	sessionID := c.BuildOpts().SessionID
	opts := c.BuildOpts().Opts

	file := "zbuild.yml"
	if v, ok := opts[keyFilename]; ok {
		file = v
	}

	buildOpts, err := builddef.NewBuildOpts(file, "context", "dev", sessionID)
	if err != nil {
		return buildOpts, err
	}

	if v, ok := opts[keyTarget]; ok {
		buildOpts.Stage = v
	}

	if v, ok := opts[keyDockerContext]; ok {
		buildOpts.BuildContext, err = builddef.NewContext(v, "")
	} else if v, ok := opts[keyContext]; ok {
		buildOpts.BuildContext, err = builddef.NewContext(v, "")
	}

	return buildOpts, err
}

func (b Builder) Build(
	ctx context.Context,
	solver statesolver.StateSolver,
	c client.Client,
) (*client.Result, error) {
	buildOpts, err := buildOptsFromBuildkitOpts(c)
	if err != nil {
		return nil, err
	}

	def, err := defloader.Load(ctx, solver, buildOpts)
	if err != nil {
		return nil, err
	}
	buildOpts.Def = def

	// At this point, the defloader loaded both the zbuildfile and its lockfile
	// but it didn't check if the RawLocks are out-of-sync with the generic
	// BuildDef. If it's the case (eg. a property in the zbuildfile has been
	// added/removed/updated), an OutOfSyncLockfileError is returned to let the
	// user know they should run `zbuild update` first.
	if def.Hash() != def.RawLocks.DefHash {
		return nil, OutOfSyncLockfileError{}
	}

	state, img, err := b.build(ctx, solver, buildOpts)
	if err != nil {
		return nil, err
	}

	return solveStateWithImage(ctx, c, state, img)
}

// OutOfSyncLockfileError is returned by Builder.Build() when the hash of the
// original BuildDef used to generate the lockfile does not match the hash of
// the current BuildDef.
type OutOfSyncLockfileError struct{}

func (err OutOfSyncLockfileError) Error() string {
	return "your lockfile is out-of-sync with your definition file, please run `zbuild update`"
}

func (b Builder) build(
	ctx context.Context,
	solver statesolver.StateSolver,
	buildOpts builddef.BuildOpts,
) (llb.State, *image.Image, error) {
	webserverStage := isWebserverStage(buildOpts.Stage)
	if webserverStage {
		buildOpts.Stage = strings.TrimPrefix(buildOpts.Stage, "webserver-")
	}

	handler, err := b.findHandler(buildOpts.Def.Kind, solver, webserverStage)
	if err != nil {
		return llb.State{}, nil, err
	}

	state, img, err := handler.Build(ctx, buildOpts)
	if err != nil {
		return state, img, err
	}

	if webserverStage {
		buildOpts.Def = newBuildDefForWebserver(buildOpts.Def)
		buildOpts.SourceState = &state
		buildOpts.Stage = "webserver"

		return b.build(ctx, solver, buildOpts)
	}

	return state, img, nil
}

func newBuildDefForWebserver(parent *builddef.BuildDef) *builddef.BuildDef {
	return &builddef.BuildDef{
		Kind:      "webserver",
		RawConfig: extractWebserverFromParent(parent.RawConfig),
		RawLocks: builddef.RawLocks{
			Raw: extractWebserverFromParent(parent.RawLocks.Raw),
		},
	}
}

func extractWebserverFromParent(parent map[string]interface{}) map[string]interface{} {
	raw := map[string]interface{}{}
	webserver, ok := parent["webserver"]
	if !ok {
		return raw
	}

	for k, v := range webserver.(map[interface{}]interface{}) {
		raw[k.(string)] = v
	}

	return raw
}

func isWebserverStage(stage string) bool {
	return strings.HasPrefix(stage, "webserver-")
}

func solveStateWithImage(
	ctx context.Context,
	c client.Client,
	state llb.State,
	img *image.Image,
) (*client.Result, error) {
	if img == nil {
		return nil, errors.New("specialized builder returned a nil image")
	}

	res, ref, err := llbutils.SolveState(ctx, c, state)
	if err != nil {
		return nil, err
	}

	config, err := json.Marshal(img)
	if err != nil {
		return nil, xerrors.Errorf("failed to marshal image config: %w", err)
	}

	res.AddMeta(exptypes.ExporterImageConfigKey, config)
	res.SetRef(ref)

	return res, nil
}

func (b Builder) Debug(
	solver statesolver.StateSolver,
	file,
	stage string,
) (llb.State, error) {
	var state llb.State

	buildOpts, err := builddef.NewBuildOpts(file, "", stage, "")
	if err != nil {
		return state, err
	}

	ctx := context.Background()
	def, err := defloader.Load(ctx, solver, buildOpts)
	if err != nil {
		return llb.State{}, err
	}
	buildOpts.Def = def

	state, _, err = b.build(ctx, solver, buildOpts)
	return state, err
}

func (b Builder) DumpConfig(
	solver statesolver.StateSolver,
	file,
	stage string,
) ([]byte, error) {
	buildOpts, err := builddef.NewBuildOpts(file, "", stage, "")
	if err != nil {
		return []byte{}, err
	}

	ctx := context.Background()
	def, err := defloader.Load(ctx, solver, buildOpts)
	if err != nil {
		return nil, err
	}

	webserverStage := isWebserverStage(stage)
	if webserverStage {
		def = newBuildDefForWebserver(def)
		buildOpts.Stage = "webserver"
	}
	buildOpts.Def = def

	handler, err := b.findHandler(def.Kind, solver, webserverStage)
	if err != nil {
		return nil, err
	}

	dumpable, err := handler.DebugConfig(buildOpts)
	if err != nil {
		return nil, err
	}

	return yaml.Marshal(dumpable)
}

func (b Builder) UpdateLockFile(
	solver statesolver.StateSolver,
	buildOpts builddef.BuildOpts,
) error {
	var err error
	ctx := context.Background()

	buildOpts.Def, err = defloader.Load(ctx, solver, buildOpts)
	if err != nil {
		return err
	}

	rawLocks, err := b.updateLocks(ctx, solver, buildOpts)
	if err != nil {
		return err
	}

	// The raw BuildDef (Kind + RawConfig) is hashed and the hash is added to
	// the locked properties to be able to compare the hash of the BuildDef
	// to the locked one later on, when loading both files. This is used to
	// detect any changes on the BuildDef made without re-running
	// `zbuild update`.
	rawLocks["defhash"] = buildOpts.Def.Hash()

	buf, err := yaml.Marshal(rawLocks)
	if err != nil {
		return err
	}

	err = b.Filesystem.WriteFile(buildOpts.LockFile, buf, 0640)
	if err != nil {
		return xerrors.Errorf("could not write %s: %w", buildOpts.LockFile, err)
	}

	return nil
}

func (b Builder) updateLocks(
	ctx context.Context,
	solver statesolver.StateSolver,
	buildOpts builddef.BuildOpts,
) (map[string]interface{}, error) {
	handler, err := b.findHandler(buildOpts.Def.Kind, solver, false)
	if err != nil {
		return nil, err
	}

	locks, err := handler.UpdateLocks(ctx, b.PkgSolvers, buildOpts)
	if err != nil {
		return nil, err
	}

	rawLocks := locks.RawLocks()
	if !b.Registry.EmbedWebserverDef(buildOpts.Def.Kind) {
		return rawLocks, nil
	}

	buildOpts.Def = newBuildDefForWebserver(buildOpts.Def)
	rawLocks["webserver"], err = b.updateLocks(ctx, solver, buildOpts)
	if err != nil {
		return nil, err
	}

	return rawLocks, nil
}
