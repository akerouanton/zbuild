package webserver

import (
	"context"

	"github.com/NiR-/zbuild/pkg/builddef"
	"github.com/NiR-/zbuild/pkg/pkgsolver"
	"github.com/NiR-/zbuild/pkg/statesolver"
	"golang.org/x/xerrors"
)

type DefinitionLocks struct {
	BaseImage      string             `mapstructure:"base_image"`
	OSRelease      builddef.OSRelease `mapstructure:"osrelease"`
	SystemPackages map[string]string  `mapstructure:"system_packages"`
}

func (l DefinitionLocks) RawLocks() map[string]interface{} {
	return map[string]interface{}{
		"base_image":      l.BaseImage,
		"osrelease":       l.OSRelease,
		"system_packages": l.SystemPackages,
	}
}

func (h *WebserverHandler) UpdateLocks(
	ctx context.Context,
	pkgSolvers pkgsolver.PackageSolversMap,
	opts builddef.UpdateLocksOpts,
) (builddef.Locks, error) {
	def, err := NewKind(opts.BuildOpts.Def)
	if err != nil {
		return nil, err
	}

	if opts.UpdateImageRef {
		baseImageRef := def.Type.BaseImage(def.Version, def.Alpine)
		def.Locks.BaseImage, err = h.solver.ResolveImageRef(ctx, baseImageRef)
		if err != nil {
			return nil, xerrors.Errorf("could not resolve image %q: %w",
				baseImageRef, err)
		}

		osrelease, err := statesolver.ResolveImageOS(ctx, h.solver, def.Locks.BaseImage)
		if err != nil {
			return nil, xerrors.Errorf("could not resolve OS details from base image: %w", err)
		}
		def.Locks.OSRelease = osrelease
	}

	var pkgSolverType pkgsolver.SolverType
	if def.Locks.OSRelease.Name == "debian" {
		pkgSolverType = pkgsolver.APT
	} else if def.Locks.OSRelease.Name == "alpine" {
		pkgSolverType = pkgsolver.APK
	} else {
		return nil, xerrors.Errorf("unsupported OS %s: only debian-based and alpine-based base images are supported", def.Locks.OSRelease.Name)
	}

	if opts.UpdateSystemPackages {
		pkgSolver := pkgSolvers.New(pkgSolverType, h.solver)
		def.Locks.SystemPackages, err = pkgSolver.ResolveVersions(ctx,
			def.Locks.BaseImage, def.SystemPackages.Map())
		if err != nil {
			return nil, xerrors.Errorf("could not resolve system packages: %w", err)
		}
	}

	return def.Locks, nil
}
