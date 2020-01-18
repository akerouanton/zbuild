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
	Stages        map[string]StageLocks `mapstructure:"stages"`
	SourceContext *builddef.Context     `mapstructure:"source_context"`
}

func (l DefinitionLocks) RawLocks() map[string]interface{} {
	lockdata := map[string]interface{}{
		"base": l.BaseImage,
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
	pkgSolver pkgsolver.PackageSolver,
	genericDef *builddef.BuildDef,
) (builddef.Locks, error) {
	def, err := NewKind(genericDef)
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
	if osrelease.Name != "debian" {
		return nil, xerrors.Errorf("unsupported OS %q: only debian-based base images are supported", osrelease.Name)
	}

	stagesLocks, err := h.updateStagesLocks(ctx, pkgSolver, def)
	def.Locks.Stages = stagesLocks
	// @TODO
	def.Locks.SourceContext = def.SourceContext

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
		stageLocks.SystemPackages, err = pkgSolver.ResolveVersions(
			def.Locks.BaseImage,
			stage.SystemPackages.Map())
		if err != nil {
			return nil, xerrors.Errorf("could not resolve versions of system packages to install: %w", err)
		}

		locks[name] = stageLocks
	}

	return locks, nil
}
