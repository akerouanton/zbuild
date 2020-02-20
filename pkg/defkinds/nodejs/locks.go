package nodejs

import (
	"context"

	"github.com/NiR-/zbuild/pkg/builddef"
	"github.com/NiR-/zbuild/pkg/pkgsolver"
	"github.com/NiR-/zbuild/pkg/statesolver"
	"golang.org/x/xerrors"
)

type DefinitionLocks struct {
	BaseImage     string                `mapstructure:"base"`
	OSRelease     builddef.OSRelease    `mapstructure:"osrelease"`
	Stages        map[string]StageLocks `mapstructure:"stages"`
	SourceContext *builddef.Context     `mapstructure:"source_context"`
}

// @TODO: add a generic way to transform locks into rawlocks
func (l DefinitionLocks) RawLocks() map[string]interface{} {
	lockdata := map[string]interface{}{
		"base":           l.BaseImage,
		"osrelease":      l.OSRelease,
		"source_context": nil,
	}

	if l.SourceContext != nil {
		lockdata["source_context"] = l.SourceContext.RawLocks()
	}

	stages := map[string]interface{}{}
	for name, stage := range l.Stages {
		stages[name] = stage.RawLocks()
	}
	lockdata["stages"] = stages

	return lockdata
}

type StageLocks struct {
	SystemPackages map[string]string `mapstructure:"system_packages"`
}

func (l StageLocks) RawLocks() map[string]interface{} {
	return map[string]interface{}{
		"system_packages": l.SystemPackages,
	}
}

func (h *NodeJSHandler) UpdateLocks(
	ctx context.Context,
	pkgSolvers pkgsolver.PackageSolversMap,
	buildOpts builddef.BuildOpts,
) (builddef.Locks, error) {
	def, err := NewKind(buildOpts.Def)
	if err != nil {
		return nil, err
	}

	def.Locks = DefinitionLocks{}
	def.Locks.BaseImage, err = h.solver.ResolveImageRef(ctx, def.BaseImage)
	if err != nil {
		return nil, xerrors.Errorf("could not resolve image %q: %w",
			def.BaseImage, err)
	}

	osrelease, err := statesolver.ResolveImageOS(ctx, h.solver, def.Locks.BaseImage)
	if err != nil {
		return nil, xerrors.Errorf("could not resolve OS details from base image: %w", err)
	}
	def.Locks.OSRelease = osrelease

	var pkgSolverType pkgsolver.SolverType
	if osrelease.Name == "debian" {
		pkgSolverType = pkgsolver.APT
	} else if osrelease.Name == "alpine" {
		pkgSolverType = pkgsolver.APK
	} else {
		return nil, xerrors.Errorf("unsupported OS %q: only debian-based and alpine-based base images are supported", osrelease.Name)
	}

	pkgSolver := pkgSolvers.New(pkgSolverType, h.solver)
	def.Locks.Stages, err = h.updateStagesLocks(ctx, pkgSolver, def)
	if err != nil {
		return nil, xerrors.Errorf("failed to update stages locks: %w", err)
	}

	def.Locks.SourceContext, err = h.lockSourceContext(ctx, def.SourceContext)
	if err != nil {
		return nil, xerrors.Errorf("failed to lock source context: %w", err)
	}

	return def.Locks, err
}

func (h *NodeJSHandler) updateStagesLocks(
	ctx context.Context,
	pkgSolver pkgsolver.PackageSolver,
	def Definition,
) (map[string]StageLocks, error) {
	locks := map[string]StageLocks{}

	for name := range def.Stages {
		stage, err := def.ResolveStageDefinition(name, false)
		if err != nil {
			return nil, xerrors.Errorf("could not resolve stage %q: %w", name, err)
		}

		stageLocks := StageLocks{}
		stageLocks.SystemPackages, err = pkgSolver.ResolveVersions(ctx,
			def.Locks.BaseImage, stage.SystemPackages.Map())
		if err != nil {
			return nil, xerrors.Errorf("could not resolve versions of system packages to install: %w", err)
		}

		locks[name] = stageLocks
	}

	return locks, nil
}

func (h *NodeJSHandler) lockSourceContext(ctx context.Context, c *builddef.Context) (*builddef.Context, error) {
	locked, err := statesolver.LockContext(ctx, h.solver, c)
	if err != nil {
		return nil, err
	}
	return locked, nil
}
