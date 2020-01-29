package webserver

import (
	"context"

	"github.com/NiR-/zbuild/pkg/builddef"
	"github.com/NiR-/zbuild/pkg/pkgsolver"
	"github.com/NiR-/zbuild/pkg/statesolver"
	"golang.org/x/xerrors"
)

type DefinitionLocks struct {
	BaseImage      string            `mapstructure:"base_image"`
	SystemPackages map[string]string `mapstructure:"system_packages"`
}

func (l DefinitionLocks) RawLocks() map[string]interface{} {
	return map[string]interface{}{
		"base_image":      l.BaseImage,
		"system_packages": l.SystemPackages,
	}
}

func (h *WebserverHandler) UpdateLocks(
	ctx context.Context,
	pkgSolver pkgsolver.PackageSolver,
	buildOpts builddef.BuildOpts,
) (builddef.Locks, error) {
	def, err := NewKind(buildOpts.Def)
	if err != nil {
		return nil, err
	}

	def.Locks = DefinitionLocks{}
	def.Locks.BaseImage, err = h.solver.ResolveImageRef(ctx, def.Type.BaseImage())
	if err != nil {
		return nil, xerrors.Errorf("could not resolve image %q: %w",
			def.Type.BaseImage(), err)
	}

	osrelease, err := statesolver.ResolveImageOS(ctx, h.solver, def.Locks.BaseImage)
	if err != nil {
		return nil, xerrors.Errorf("could not resolve OS details from base image: %w", err)
	}
	if osrelease.Name != "debian" {
		return nil, xerrors.Errorf("unsupported OS %s: only debian-based images are supported", osrelease.Name)
	}

	def.Locks.SystemPackages, err = pkgSolver.ResolveVersions(
		def.Locks.BaseImage,
		def.SystemPackages.Map())
	if err != nil {
		return nil, xerrors.Errorf("could not resolve system packages: %w", err)
	}

	return def.Locks, nil
}
