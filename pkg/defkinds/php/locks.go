package php

import (
	"context"
	"strings"

	"github.com/NiR-/notpecl/extindex"
	"github.com/NiR-/zbuild/pkg/builddef"
	"github.com/NiR-/zbuild/pkg/pkgsolver"
	"github.com/NiR-/zbuild/pkg/statesolver"
	"golang.org/x/xerrors"
)

// DefinitionLocks defines version locks for system packages and PHP extensions used
// by each stage.
type DefinitionLocks struct {
	BaseImage     string                `mapstructure:"base_image"`
	ExtensionDir  string                `mapstructure:"extension_dir"`
	Stages        map[string]StageLocks `mapstructure:"stages"`
	SourceContext *builddef.Context     `mapstructure:"source_context"`
}

func (l DefinitionLocks) RawLocks() map[string]interface{} {
	lockdata := map[string]interface{}{
		"base_image":     l.BaseImage,
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
	buildOpts builddef.BuildOpts,
) (builddef.Locks, error) {
	def, err := NewKind(buildOpts.Def)
	if err != nil {
		return nil, err
	}

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

	def.Locks.ExtensionDir, err = h.resolveExtensionDir(ctx, def.Locks.BaseImage)
	if err != nil {
		return nil, err
	}

	def.Locks.SourceContext, err = h.lockSourceContext(ctx, def.SourceContext)
	if err != nil {
		return nil, xerrors.Errorf("failed to lock source context: %w", err)
	}

	def.Locks.Stages, err = h.updateStagesLocks(ctx, pkgSolver, def, buildOpts)
	if err != nil {
		return nil, xerrors.Errorf("failed to update stages locks: %w", err)
	}

	return def.Locks, err
}

func (h *PHPHandler) resolveExtensionDir(ctx context.Context, image string) (string, error) {
	buf, err := h.solver.ExecImage(ctx, image, []string{
		"php", "-r", "echo ini_get('extension_dir');",
	})
	if err != nil {
		return "", xerrors.Errorf("fail to resolve extension dir from base image: %w", err)
	}

	return buf.String(), nil
}

// @TODO: remove pkgSolver?
func (h *PHPHandler) updateStagesLocks(
	ctx context.Context,
	pkgSolver pkgsolver.PackageSolver,
	def Definition,
	buildOpts builddef.BuildOpts,
) (map[string]StageLocks, error) {
	locks := map[string]StageLocks{}
	composerLockLoader := func(stageDef *StageDefinition) error {
		return LoadComposerLock(ctx, h.solver, stageDef, buildOpts.BuildContext)
	}

	for name := range def.Stages {
		stage, err := def.ResolveStageDefinition(name, composerLockLoader, false)
		if err != nil {
			return nil, xerrors.Errorf("could not resolve stage %q: %w", name, err)
		}

		stageLocks := StageLocks{}
		stageLocks.SystemPackages, err = pkgSolver.ResolveVersions(
			def.Locks.BaseImage,
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

func (h *PHPHandler) lockSourceContext(ctx context.Context, c *builddef.Context) (*builddef.Context, error) {
	locked, err := statesolver.LockContext(ctx, h.solver, c)
	if err != nil {
		return nil, err
	}
	return locked, nil
}
