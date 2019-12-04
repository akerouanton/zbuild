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
	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/exporter/containerimage/exptypes"
	"github.com/moby/buildkit/frontend/gateway/client"
	"golang.org/x/xerrors"
)

// These consts come from: https://github.com/moby/buildkit/blob/master/frontend/dockerfile/builder/build.go
const (
	keyTarget         = "target"
	keyNameContext    = "contextkey"
	keyNameDockerfile = "dockerfilekey"
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
	if v, ok := opts[keyNameDockerfile]; ok {
		file = v
	}

	stage := "dev"
	if v, ok := opts[keyTarget]; ok {
		stage = v
	}

	return builddef.NewBuildOpts(file, stage, sessionID)
}

func (b Builder) Build(ctx context.Context, c client.Client) (*client.Result, error) {
	buildOpts := buildOptsFromBuildkitOpts(c)
	def, err := builddef.LoadFromContext(ctx, c, buildOpts)
	if err != nil {
		return nil, err
	}

	handler, err := b.Registry.FindHandler(def.Kind)
	if err != nil {
		return nil, err
	}

	buildOpts.Def = def
	state, img, err := handler.Build(ctx, c, buildOpts)
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
		return nil, xerrors.Errorf("failed to marshal image config: %v", err)
	}

	res.AddMeta(exptypes.ExporterImageConfigKey, config)
	res.SetRef(ref)

	return res, nil
}

func (b Builder) Debug(file, stage string) (llb.State, error) {
	var state llb.State
	opts := builddef.NewBuildOpts(file, stage, "<SESSION-ID>")

	def, err := builddef.LoadFromFS(opts.File, opts.LockFile)
	if err != nil {
		return state, err
	}

	handler, err := b.Registry.FindHandler(def.Kind)
	if err != nil {
		return state, err
	}

	opts.Def = def
	state, err = handler.DebugLLB(opts)
	if err != nil {
		return state, err
	}

	return state, nil
}

func (b Builder) UpdateLockFile(file string) error {
	lockFile := builddef.LockFilepath(file)

	def, err := builddef.LoadFromFS(file, lockFile)
	if err != nil {
		return err
	}

	handler, err := b.Registry.FindHandler(def.Kind)
	if err != nil {
		return err
	}

	locks, err := handler.UpdateLocks(def, b.PkgSolver)
	if err != nil {
		return err
	}

	lockdata, err := locks.RawLocks()
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(lockFile, lockdata, 0640)
	if err != nil {
		return xerrors.Errorf("could not write %s: %v", lockFile, err)
	}

	return nil
}
