package builder

import (
	"context"
	"encoding/json"
	"errors"
	"io/ioutil"

	"github.com/NiR-/zbuild/pkg/builddef"
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

type frontOptions struct {
	file  string
	stage string
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

	handler, err := b.Registry.FindHandler(def.Kind)
	if err != nil {
		return nil, err
	}
	handler.WithSolver(solver)

	buildOpts.Def = def
	state, img, err := handler.Build(ctx, buildOpts)
	if err != nil {
		return nil, err
	}

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

	buildOpts := builddef.NewBuildOpts(file)
	buildOpts.Stage = stage
	buildOpts.SessionID = "<SESSION-ID>"

	ctx := context.Background()
	def, err := builddef.Load(ctx, solver, buildOpts)
	if err != nil {
		return state, err
	}

	handler, err := b.Registry.FindHandler(def.Kind)
	if err != nil {
		return state, err
	}
	handler.WithSolver(solver)

	buildOpts.Def = def
	state, _, err = handler.Build(ctx, buildOpts)
	if err != nil {
		return state, err
	}

	return state, nil
}

func (b Builder) DumpConfig(
	solver statesolver.StateSolver,
	file,
	stage string,
) ([]byte, error) {
	buildOpts := builddef.NewBuildOpts(file)
	buildOpts.Stage = stage
	// @TODO: remove?
	buildOpts.SessionID = "<SESSION-ID>"

	ctx := context.Background()
	def, err := builddef.Load(ctx, solver, buildOpts)
	if err != nil {
		return nil, err
	}
	buildOpts.Def = def

	handler, err := b.Registry.FindHandler(def.Kind)
	if err != nil {
		return nil, err
	}
	handler.WithSolver(solver)

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

	handler, err := b.Registry.FindHandler(def.Kind)
	if err != nil {
		return err
	}
	handler.WithSolver(solver)

	locks, err := handler.UpdateLocks(ctx, b.PkgSolver, def)
	if err != nil {
		return err
	}

	rawLock, err := yaml.Marshal(locks.RawLocks())
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(buildOpts.LockFile, rawLock, 0640)
	if err != nil {
		return xerrors.Errorf("could not write %s: %w", buildOpts.LockFile, err)
	}

	return nil
}
