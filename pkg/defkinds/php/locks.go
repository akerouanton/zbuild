package php

import (
	"context"
	"strings"

	"github.com/NiR-/notpecl/extindex"
	"github.com/NiR-/webdf/pkg/builddef"
	"github.com/NiR-/webdf/pkg/pkgsolver"
	"golang.org/x/xerrors"
	"gopkg.in/yaml.v2"
)

// DefinitionLocks defines version locks for system packages and PHP extensions used
// by each stage.
type DefinitionLocks struct {
	builddef.BaseLocks `yaml:",inline"`
	Stages             map[string]StageLocks `yaml:"stages"`
}

func (l DefinitionLocks) RawLocks() ([]byte, error) {
	lockdata, err := yaml.Marshal(l)
	if err != nil {
		return lockdata, xerrors.Errorf("could not marshal php locks: %v", err)
	}
	return lockdata, nil
}

// StageLocks represents the version locks for a single stage.
type StageLocks struct {
	builddef.BaseStageLocks `yaml:",inline"`

	Extensions map[string]string `yaml:"extensions"`
}

func (h PHPHandler) UpdateLocks(
	genericDef *builddef.BuildDef,
	pkgSolver pkgsolver.PackageSolver,
) (builddef.Locks, error) {
	def, err := NewKind(genericDef)
	if err != nil {
		return nil, err
	}

	// @TODO: support template in base image param instead
	// @TODO: resolve sha256 of the base image and lock it
	baseImageRef := def.BaseImage
	if baseImageRef == "" {
		baseImageRef = defaultBaseImage + ":" + def.Version
		if *def.BaseStage.FPM {
			// @TODO: Add a distro param?
			baseImageRef += "-fpm-buster"
		}
	}
	def.Locks.BaseImage = baseImageRef

	ctx := context.TODO()
	osrelease, err := builddef.ResolveImageOS(ctx, h.fetcher, baseImageRef)
	if err != nil {
		return nil, xerrors.Errorf("could not resolve base image os-release: %v", err)
	}
	def.Locks.OS = osrelease

	solverCfg, err := pkgsolver.GuessSolverConfig(osrelease, "amd64")
	if err != nil {
		return nil, xerrors.Errorf("could not update stage locks: %v", err)
	}
	err = pkgSolver.Configure(solverCfg)
	if err != nil {
		return nil, xerrors.Errorf("could not update stage locks: %v", err)
	}

	stagesLocks, err := h.updateStagesLocks(def, pkgSolver)
	def.Locks.Stages = stagesLocks
	return def.Locks, err
}

func (h PHPHandler) updateStagesLocks(
	def Definition,
	pkgSolver pkgsolver.PackageSolver,
) (map[string]StageLocks, error) {
	locks := map[string]StageLocks{}

	platformReqsLoader := func(stage *StageDefinition) error {
		// @TODO: basedir should be the parent dir of the webdf.yml file
		return LoadPlatformReqsFromFS(stage, "")
	}

	for name := range def.Stages {
		stage, err := def.ResolveStageDefinition(name, platformReqsLoader)
		if err != nil {
			return nil, xerrors.Errorf("could not resolve stage %q: %v", name, err)
		}

		stageLocks := StageLocks{}
		stageLocks.SystemPackages, err = pkgSolver.ResolveVersions(stage.SystemPackages)
		if err != nil {
			return nil, xerrors.Errorf("could not resolve systems package versions: %v", err)
		}

		stageLocks.Extensions, err = h.lockExtensions(stage.Extensions)
		if err != nil {
			return nil, xerrors.Errorf("could not resolve php extension versions: %v", err)
		}

		locks[name] = stageLocks
	}

	return locks, nil
}

func (h PHPHandler) lockExtensions(extensions map[string]string) (map[string]string, error) {
	resolved := map[string]string{}
	ctx := context.Background()

	for extName, constraint := range extensions {
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
