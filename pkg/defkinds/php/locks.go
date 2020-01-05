package php

import (
	"context"
	"strings"

	"github.com/NiR-/notpecl/extindex"
	"github.com/NiR-/zbuild/pkg/builddef"
	"github.com/NiR-/zbuild/pkg/defkinds/webserver"
	"github.com/NiR-/zbuild/pkg/pkgsolver"
	"github.com/NiR-/zbuild/pkg/registry"
	"golang.org/x/xerrors"
)

var defaultBaseImages = map[string]struct {
	FPM string
	CLI string
}{
	"7.2": {
		FPM: "docker.io/library/php:7.2-fpm-buster",
		CLI: "docker.io/library/php:7.2-cli-buster",
	},
	"7.3": {
		FPM: "docker.io/library/php:7.3-fpm-buster",
		CLI: "docker.io/library/php:7.3-cli-buster",
	},
	"7.4": {
		FPM: "docker.io/library/php:7.4-fpm-buster",
		CLI: "docker.io/library/php:7.4-cli-buster",
	},
}

// DefinitionLocks defines version locks for system packages and PHP extensions used
// by each stage.
type DefinitionLocks struct {
	BaseImage string                     `mapstructure:"base_image"`
	Stages    map[string]StageLocks      `mapstructure:"stages"`
	Webserver *webserver.DefinitionLocks `mapstructure:"webserver"`
}

func (l DefinitionLocks) RawLocks() map[string]interface{} {
	lockdata := map[string]interface{}{
		"base_image": l.BaseImage,
	}

	stages := map[string]interface{}{}
	for name, stage := range l.Stages {
		stages[name] = stage.RawLocks()
	}
	lockdata["stages"] = stages

	return lockdata
}

// StageLocks represents the version locks for a single stage.
type StageLocks struct {
	SystemPackages map[string]string `mapstructure:"system_packages"`
	Extensions     map[string]string `mapstructure:"extensions"`
}

func (l StageLocks) RawLocks() map[string]interface{} {
	return map[string]interface{}{
		"system_packages": l.SystemPackages,
		"extensions":      l.Extensions,
	}
}

func (h *PHPHandler) UpdateLocks(
	ctx context.Context,
	pkgSolver pkgsolver.PackageSolver,
	genericDef *builddef.BuildDef,
) (builddef.Locks, error) {
	def, err := NewKind(genericDef)
	if err != nil {
		return nil, err
	}

	if def.Webserver != nil {
		webserverLocks, err := h.updateWebserverLocks(ctx, pkgSolver, def.Webserver)
		if err != nil {
			err = xerrors.Errorf("could not update webserver locks: %w", err)
			return nil, err
		}

		locks := webserverLocks.(webserver.DefinitionLocks)
		def.Locks.Webserver = &locks
	}
	// @TODO: resolve sha256 of the base image and lock it
	def.Locks.BaseImage = def.BaseImage

	osrelease, err := builddef.ResolveImageOS(ctx, h.solver, def.Locks.BaseImage)
	if err != nil {
		return nil, xerrors.Errorf("could not resolve OS details from base image: %w", err)
	}
	if osrelease.Name != "debian" {
		return nil, xerrors.Errorf("unsupported OS %q: only debian-based base images are supported", osrelease.Name)
	}

	stagesLocks, err := h.updateStagesLocks(ctx, pkgSolver, def, def.Locks)
	def.Locks.Stages = stagesLocks
	return def.Locks, err
}

func (h *PHPHandler) updateWebserverLocks(
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

// @TODO: remove pkgSolver?
func (h *PHPHandler) updateStagesLocks(
	ctx context.Context,
	pkgSolver pkgsolver.PackageSolver,
	def Definition,
	defLocks DefinitionLocks,
) (map[string]StageLocks, error) {
	locks := map[string]StageLocks{}
	composerLockLoader := func(stageDef *StageDefinition) error {
		return LoadComposerLock(ctx, h.solver, stageDef)
	}

	for name := range def.Stages {
		stage, err := def.ResolveStageDefinition(name, composerLockLoader, false)
		if err != nil {
			return nil, xerrors.Errorf("could not resolve stage %q: %w", name, err)
		}

		stageLocks := StageLocks{}
		stageLocks.SystemPackages, err = pkgSolver.ResolveVersions(
			defLocks.BaseImage,
			stage.SystemPackages.Map())
		if err != nil {
			return nil, xerrors.Errorf("could not resolve systems package versions: %w", err)
		}

		stageLocks.Extensions, err = h.lockExtensions(stage.Extensions)
		if err != nil {
			return nil, xerrors.Errorf("could not resolve php extension versions: %w", err)
		}

		// @TODO: lock global extensions?

		locks[name] = stageLocks
	}

	return locks, nil
}

func (h *PHPHandler) lockExtensions(extensions *builddef.VersionMap) (map[string]string, error) {
	resolved := map[string]string{}
	ctx := context.Background()

	// Remove extensions installed by default as this would result in a build
	// error otherwise.
	for _, name := range extensions.Names() {
		if _, ok := preinstalledExtensions[name]; ok {
			extensions.Remove(name)
		}
	}

	for extName, constraint := range extensions.Map() {
		if isCoreExtension(extName) {
			resolved[extName] = constraint
			continue
		}

		segments := strings.SplitN(constraint, "@", 2)
		stability := extindex.Stable
		if len(segments) == 2 {
			stability = extindex.StabilityFromString(segments[1])
		}

		extVer, err := h.NotPecl.ResolveConstraint(ctx, extName, segments[0], stability)
		if err != nil {
			return resolved, err
		}

		resolved[extName] = extVer
	}

	return resolved, nil
}
