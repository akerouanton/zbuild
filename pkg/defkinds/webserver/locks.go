package webserver

import (
	"context"

	"github.com/NiR-/zbuild/pkg/builddef"
	"github.com/NiR-/zbuild/pkg/pkgsolver"
	"golang.org/x/xerrors"
	"gopkg.in/yaml.v2"
)

type DefinitionLocks struct {
	BaseImage      string            `yaml:"base_image"`
	SystemPackages map[string]string `yaml:"system_packages"`
}

func (l DefinitionLocks) RawLocks() ([]byte, error) {
	lockdata, err := yaml.Marshal(l)
	if err != nil {
		return lockdata, xerrors.Errorf("could not marshal webserver locks: %w", err)
	}
	return lockdata, nil
}

func (h *WebserverHandler) UpdateLocks(
	ctx context.Context,
	pkgSolver pkgsolver.PackageSolver,
	genericDef *builddef.BuildDef,
) (builddef.Locks, error) {
	def, err := NewKind(genericDef)
	if err != nil {
		return nil, err
	}

	// @TODO: resolve the sha256 of the base image
	def.Locks.BaseImage = def.Type.BaseImage()

	osrelease, err := builddef.ResolveImageOS(ctx, h.solver, def.Locks.BaseImage)
	if err != nil {
		return nil, xerrors.Errorf("could not resolve OS details from base image: %w", err)
	}
	if osrelease.Name != "debian" {
		return nil, xerrors.Errorf("unsupported OS %s: only debian-based images are supported", osrelease.Name)
	}

	pkgSolverCfg, err := pkgsolver.GuessSolverConfig(osrelease, "amd64")
	if err != nil {
		return nil, xerrors.Errorf("could not update locks: %w", err)
	}
	err = pkgSolver.Configure(pkgSolverCfg)
	if err != nil {
		return nil, xerrors.Errorf("could not update locks: %w", err)
	}

	def.Locks.SystemPackages, err = pkgSolver.ResolveVersions(def.SystemPackages)

	return def.Locks, nil
}
