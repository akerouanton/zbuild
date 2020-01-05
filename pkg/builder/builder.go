package builder

import (
	"context"
	"encoding/json"
	"errors"
	"io/ioutil"
	"strings"

	"github.com/NiR-/zbuild/pkg/builddef"
	"github.com/NiR-/zbuild/pkg/image"
	"github.com/NiR-/zbuild/pkg/llbutils"
	"github.com/NiR-/zbuild/pkg/pkgsolver"
	"github.com/NiR-/zbuild/pkg/registry"
	"github.com/NiR-/zbuild/pkg/statesolver"
	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/exporter/containerimage/exptypes"
	"github.com/moby/buildkit/frontend/gateway/client"
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

type Builder struct {
	Registry  *registry.KindRegistry
	PkgSolver pkgsolver.PackageSolver
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

func buildOptsFromBuildkitOpts(c client.Client) builddef.BuildOpts {
	sessionID := c.BuildOpts().SessionID
	opts := c.BuildOpts().Opts

	file := "zbuild.yml"
	if v, ok := opts[keyFilename]; ok {
		file = v
	}

	stage := "dev"
	if v, ok := opts[keyTarget]; ok {
		stage = v
	}

	contextName := "context"
	if v, ok := opts[keyDockerContext]; ok {
		contextName = v
	} else if v, ok := opts[keyContext]; ok {
		contextName = v
	}

	buildOpts := builddef.NewBuildOpts(file)
	buildOpts.Stage = stage
	buildOpts.SessionID = sessionID
	buildOpts.ContextName = contextName

	return buildOpts
}

func (b Builder) Build(
	ctx context.Context,
	solver statesolver.StateSolver,
	c client.Client,
) (*client.Result, error) {
	buildOpts := buildOptsFromBuildkitOpts(c)
	def, err := builddef.Load(ctx, solver, buildOpts)
	if err != nil {
		return nil, err
	}
	buildOpts.Def = def

	state, img, err := b.build(ctx, solver, buildOpts)
	if err != nil {
		return nil, err
	}

	return solveStateWithImage(ctx, c, state, img)
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
		buildOpts.Source = &state
		buildOpts.Stage = "webserver"

		return b.build(ctx, solver, buildOpts)
	}

	return state, img, nil
}

func newBuildDefForWebserver(parent *builddef.BuildDef) *builddef.BuildDef {
	return &builddef.BuildDef{
		Kind:      "webserver",
		RawConfig: extractWebserverFromParent(parent.RawConfig),
		RawLocks:  extractWebserverFromParent(parent.RawLocks),
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
	buildOpts := builddef.NewBuildOpts(file)
	buildOpts.Stage = stage
	buildOpts.SessionID = "<SESSION-ID>"

	ctx := context.Background()
	def, err := builddef.Load(ctx, solver, buildOpts)
	if err != nil {
		return llb.State{}, err
	}
	buildOpts.Def = def

	state, _, err := b.build(ctx, solver, buildOpts)
	return state, err
}

func (b Builder) DumpConfig(
	solver statesolver.StateSolver,
	file,
	stage string,
) ([]byte, error) {
	buildOpts := builddef.NewBuildOpts(file)
	buildOpts.Stage = stage

	ctx := context.Background()
	def, err := builddef.Load(ctx, solver, buildOpts)
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
	file string,
) error {
	ctx := context.Background()
	buildOpts := builddef.NewBuildOpts(file)
	def, err := builddef.Load(ctx, solver, buildOpts)
	if err != nil {
		return err
	}

	rawLocks, err := b.updateLocks(ctx, solver, def)
	if err != nil {
		return err
	}

	buf, err := yaml.Marshal(rawLocks)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(buildOpts.LockFile, buf, 0640)
	if err != nil {
		return xerrors.Errorf("could not write %s: %w", buildOpts.LockFile, err)
	}

	return nil
}

func (b Builder) updateLocks(
	ctx context.Context,
	solver statesolver.StateSolver,
	def *builddef.BuildDef,
) (map[string]interface{}, error) {
	handler, err := b.findHandler(def.Kind, solver, false)
	if err != nil {
		return nil, err
	}

	locks, err := handler.UpdateLocks(ctx, b.PkgSolver, def)
	if err != nil {
		return nil, err
	}

	rawLocks := locks.RawLocks()
	if !b.Registry.EmbedWebserverDef(def.Kind) {
		return rawLocks, nil
	}

	webserverDef := newBuildDefForWebserver(def)
	rawLocks["webserver"], err = b.updateLocks(ctx, solver, webserverDef)
	if err != nil {
		return nil, err
	}

	return rawLocks, nil
}
