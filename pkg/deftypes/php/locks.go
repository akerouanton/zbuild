package php

import (
	"github.com/NiR-/webdf/pkg/builddef"
	"github.com/NiR-/webdf/pkg/pkgsolver"
	"github.com/mitchellh/mapstructure"
	"golang.org/x/xerrors"
	"gopkg.in/yaml.v2"
)

// DefinitionLocks defines version locks for system packages and PHP extensions used
// by each stage.
type DefinitionLocks struct {
	Stages map[string]StageLocks `yaml:"stages"`
}

// StageLocks represents the version locks for a single stage.
type StageLocks struct {
	builddef.BaseLocks `yaml:",inline"`

	Extensions map[string]string `yaml:"extensions"`
}

func (l DefinitionLocks) RawLocks() ([]byte, error) {
	lockdata, err := yaml.Marshal(l)
	if err != nil {
		return lockdata, xerrors.Errorf("could not marshal php locks: %v", err)
	}
	return lockdata, nil
}

func (h PHPHandler) UpdateLocks(
	genericDef *builddef.BuildDef,
	stages []string,
	pkgSolver pkgsolver.PackageSolver,
) (builddef.Locks, error) {
	def := defaultDefinition()
	if err := mapstructure.Decode(genericDef.RawConfig, &def); err != nil {
		return nil, xerrors.Errorf("could not decode manifest: %v", err)
	}

	if err := mapstructure.Decode(genericDef.RawLocks, &def.Locks); err != nil {
		return nil, xerrors.Errorf("could not decode lock manifest: %v", err)
	}

	if len(stages) == 0 {
		stages = []string{"base"}
		for name := range def.Stages {
			stages = append(stages, name)
		}
	}

	for _, name := range stages {
		stage, err := def.ResolveStageDefinition(name)
		if err != nil {
			return nil, xerrors.Errorf("could not resolve stage %q: %+v", name, err)
		}

		addIntegrations(&stage)

		if def.Infer {
			if err := loadPlatformReqsFromFS(&stage); err != nil {
				err := xerrors.Errorf("could not load platform-reqs from composer.lock: %v", err)
				return nil, err
			}

			inferExtensions(&stage)
			inferSystemPackages(&stage)
		}

		if _, ok := def.Locks.Stages[name]; !ok {
			def.Locks.Stages[name] = StageLocks{}
		}

		locks := def.Locks.Stages[name]
		if err := updateStageLocks(stage, &locks, pkgSolver); err != nil {
			return nil, err
		}

		def.Locks.Stages[name] = locks
	}

	return def.Locks, nil
}

func updateStageLocks(
	stage StageDefinition,
	locks *StageLocks,
	pkgSolver pkgsolver.PackageSolver,
) error {
	// @TODO: guess dpkg suite/archive to use from the base service image
	err := pkgSolver.WithDpkgSuites([][]string{
		{"http://deb.debian.org/debian", "jessie"},
		{"http://deb.debian.org/debian", "jessie-updates"},
		{"http://security.debian.org", "jessie/updates"},
	})
	if err != nil {
		return xerrors.Errorf("could not update stage locks: %+v", err)
	}

	locks.SystemPackages, err = pkgSolver.ResolveVersions(stage.SystemPackages, "amd64")
	if err != nil {
		return xerrors.Errorf("could not resolve systems package versions: %v", err)
	}

	locks.Extensions, err = findExtensionVersions(stage.Extensions)
	if err != nil {
		return xerrors.Errorf("could not resolve php extension versions: %v", err)
	}

	return nil
}

func findExtensionVersions(extensions map[string]string) (map[string]string, error) {
	return extensions, nil
}
