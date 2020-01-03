package nodejs

import (
	"context"

	"github.com/NiR-/zbuild/pkg/builddef"
	"github.com/NiR-/zbuild/pkg/defkinds/webserver"
	"github.com/NiR-/zbuild/pkg/pkgsolver"
	"github.com/NiR-/zbuild/pkg/registry"
	"golang.org/x/xerrors"
	"gopkg.in/yaml.v2"
)

type DefinitionLocks struct {
	BaseImage string                     `yaml:"base"`
	Stages    map[string]StageLocks      `yaml:"stages"`
	Webserver *webserver.DefinitionLocks `yaml:"webserver"`
}

func (l DefinitionLocks) RawLocks() ([]byte, error) {
	lockdata, err := yaml.Marshal(l)
	if err != nil {
		return lockdata, xerrors.Errorf("could not marshal nodejs locks: %w", err)
	}
	return lockdata, nil
}

type StageLocks struct {
	SystemPackages map[string]string `yaml:"system_packages"`
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

	locks := DefinitionLocks{}
	// @TODO: resolve sha256 of the base image and lock it
	locks.BaseImage = def.BaseImage

	osrelease, err := builddef.ResolveImageOS(ctx, h.solver, locks.BaseImage)
	if err != nil {
		return nil, xerrors.Errorf("could not resolve OS details from base image: %w", err)
	}
	if osrelease.Name != "debian" {
		return nil, xerrors.Errorf("unsupported OS %q: only debian-based base images are supported", osrelease.Name)
	}

	stagesLocks, err := h.updateStagesLocks(ctx, pkgSolver, def, locks)
	locks.Stages = stagesLocks

	if def.Webserver != nil {
		webserverLocks, err := h.updateWebserverLocks(ctx, pkgSolver, def.Webserver)
		if err != nil {
			err = xerrors.Errorf("could not update webserver locks: %w", err)
			return nil, err
		}

		webDefLocks := webserverLocks.(webserver.DefinitionLocks)
		locks.Webserver = &webDefLocks
	}

	def.Locks = locks
	return locks, err
}

func (h *NodeJSHandler) updateWebserverLocks(
	ctx context.Context,
	pkgSolver pkgsolver.PackageSolver,
	def *webserver.Definition,
) (builddef.Locks, error) {
	var locks builddef.Locks

	webserverHandler, err := registry.FindHandler("webserver")
	if err != nil {
		return locks, err
	}
	webserverHandler.WithSolver(h.solver)

	return webserverHandler.UpdateLocks(ctx, pkgSolver, &builddef.BuildDef{
		Kind:      "webserver",
		RawConfig: def.RawConfig(),
	})
}

func (h *NodeJSHandler) updateStagesLocks(
	ctx context.Context,
	pkgSolver pkgsolver.PackageSolver,
	def Definition,
	defLocks DefinitionLocks,
) (map[string]StageLocks, error) {
	locks := map[string]StageLocks{}

	for name := range def.Stages {
		stage, err := def.ResolveStageDefinition(name, false)
		if err != nil {
			return nil, xerrors.Errorf("could not resolve stage %q: %w", name, err)
		}

		stageLocks := StageLocks{}
		stageLocks.SystemPackages, err = pkgSolver.ResolveVersions(
			defLocks.BaseImage,
			stage.SystemPackages)
		if err != nil {
			return nil, xerrors.Errorf("could not resolve versions of system packages to install: %w", err)
		}

		locks[name] = stageLocks
	}

	return locks, nil
}
