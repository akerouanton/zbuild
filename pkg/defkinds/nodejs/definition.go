package nodejs

import (
	"github.com/NiR-/zbuild/pkg/builddef"
	"github.com/NiR-/zbuild/pkg/defkinds/webserver"
	"github.com/NiR-/zbuild/pkg/llbutils"
	"github.com/mitchellh/mapstructure"
	"golang.org/x/xerrors"
	"gopkg.in/yaml.v2"
)

var defaultBaseImages = map[string]string{
	"10": "docker.io/library/node:10-buster-slim",
	"12": "docker.io/library/node:12-buster-slim",
	"13": "docker.io/library/node:13-buster-slim",
}
var supportedVersions = "10, 12, 13"

func defaultDefinition() Definition {
	devStageDevMode := true
	prodStageDevMode := false
	healthcheckEnabled := true

	return Definition{
		BaseStage: Stage{
			ExternalFiles:  []llbutils.ExternalFile{},
			SystemPackages: map[string]string{},
			ConfigFiles:    map[string]string{},
			Healthcheck:    &healthcheckEnabled,
			PostInstall:    []string{},
		},
		BaseImage: "",
		Stages: map[string]DerivedStage{
			"dev": {
				DeriveFrom: "base",
				Dev:        &devStageDevMode,
			},
			"prod": {
				DeriveFrom: "base",
				Dev:        &prodStageDevMode,
			},
		},
	}
}

// @TODO: rename into NewDefinition
func NewKind(genericDef *builddef.BuildDef) (Definition, error) {
	def := defaultDefinition()

	decoderConf := mapstructure.DecoderConfig{
		ErrorUnused:      true,
		WeaklyTypedInput: true,
		Result:           &def,
	}
	decoder, err := mapstructure.NewDecoder(&decoderConf)
	if err != nil {
		return def, err
	}

	if err := decoder.Decode(genericDef.RawConfig); err != nil {
		err := xerrors.Errorf("could not decode build manifest: %w", err)
		return def, err
	}

	if err := yaml.Unmarshal(genericDef.RawLocks, &def.Locks); err != nil {
		err := xerrors.Errorf("could not decode lock manifest: %w", err)
		return def, err
	}

	if def.Version != "" && def.BaseImage != "" {
		return def, xerrors.Errorf("you can't provide both version and base image parameters at the same time")
	}

	if def.BaseImage == "" {
		baseImage, ok := defaultBaseImages[def.Version]
		if !ok {
			return def, xerrors.Errorf("no default base image defined for NodeJS v%s, you have to define it by yourself in your zbuildfile or use one of the supported versions: %s", def.Version, supportedVersions)
		}
		def.BaseImage = baseImage
	}

	if devStage, ok := def.Stages["dev"]; ok {
		if devStage.Dev == nil {
			isDev := true
			devStage.Dev = &isDev
			def.Stages["dev"] = devStage
		}
	}
	if prodStage, ok := def.Stages["prod"]; ok {
		if prodStage.Dev == nil {
			isNotDev := false
			prodStage.Dev = &isNotDev
			def.Stages["prod"] = prodStage
		}
	}

	return def, nil
}

type Definition struct {
	BaseStage Stage `mapstructure:",squash"`

	// @TODO: check what happens when base isn't prefixed with docker.io/library/
	BaseImage  string                  `mapstructure:"base"`
	Version    string                  `mapstructure:"version"`
	Stages     map[string]DerivedStage `mapstructure:"stages"`
	IsFrontend bool                    `mapstructure:"frontend"`
	Webserver  *webserver.Definition   `mapstructure:"webserver"`

	Locks DefinitionLocks `mapstructure:"-"`
}

type Stage struct {
	ExternalFiles  []llbutils.ExternalFile `mapstructure:"external_files"`
	SystemPackages map[string]string       `mapstructure:"system_packages"`
	Command        *[]string               `mapstructure:"command"`
	ConfigFiles    map[string]string       `mapstructure:"config_files"`
	// @TODO: rename into sourcecode and accept both dirs and files
	SourceDirs   []string `mapstructure:"source_dirs"`
	StatefulDirs []string `mapstructure:"stateful_dirs"`
	Healthcheck  *bool    `mapstructur:"healthcheck"`
	PostInstall  []string `mapstructure:"post_install"`
}

type DerivedStage struct {
	Stage `mapstructure:",squash"`

	DeriveFrom string `mapstructure:"from"`
	Dev        *bool  `mapstructure:"dev"`
}

type StageDefinition struct {
	Stage
	Name       string
	BaseImage  string
	Version    string
	Dev        *bool
	IsFrontend bool
	Webserver  *webserver.Definition
	Locks      StageLocks
}

func (def *Definition) ResolveStageDefinition(
	name string,
	withLocks bool,
) (StageDefinition, error) {
	var stageDef StageDefinition

	stages := make([]DerivedStage, 0, len(def.Stages)+1)
	resolvedStages := make([]string, 0, len(def.Stages)+1)
	nextStage := name

	for nextStage != "" && nextStage != "base" {
		for _, stageName := range resolvedStages {
			if nextStage == stageName {
				return stageDef, xerrors.Errorf(
					"there's a cyclic dependency between %q and itself",
					stageName,
				)
			}
		}

		stage, ok := def.Stages[nextStage]
		if !ok {
			return StageDefinition{}, xerrors.Errorf("stage %q not found", nextStage)
		}

		stages = append(stages, stage)
		resolvedStages = append(resolvedStages, nextStage)
		nextStage = stage.DeriveFrom
	}

	stageDef = mergeStages(def, stages...)
	stageDef.Name = name

	if !withLocks {
		return stageDef, nil
	}

	locks, ok := def.Locks.Stages[name]
	if !ok {
		return stageDef, xerrors.Errorf(
			"no locks available for stage %q. Please update your lockfile", name)
	}

	stageDef.Locks = locks

	return stageDef, nil
}

func mergeStages(base *Definition, stages ...DerivedStage) StageDefinition {
	devMode := false
	stageDef := StageDefinition{
		BaseImage:  base.BaseImage,
		Version:    base.Version,
		Stage:      base.BaseStage,
		IsFrontend: base.IsFrontend,
		Dev:        &devMode,
	}

	for i := len(stages) - 1; i >= 0; i-- {
		derived := stages[i]

		if len(derived.ExternalFiles) > 0 {
			stageDef.ExternalFiles = append(stageDef.ExternalFiles, derived.ExternalFiles...)
		}
		if len(derived.SystemPackages) > 0 {
			for name, constraint := range derived.SystemPackages {
				stageDef.SystemPackages[name] = constraint
			}
		}
		if derived.Command != nil {
			stageDef.Command = derived.Command
		}
		if len(derived.ConfigFiles) > 0 {
			for from, to := range derived.ConfigFiles {
				stageDef.ConfigFiles[from] = to
			}
		}
		if len(derived.SourceDirs) > 0 {
			stageDef.SourceDirs = append(stageDef.SourceDirs, derived.SourceDirs...)
		}
		if derived.StatefulDirs != nil {
			stageDef.StatefulDirs = append(stageDef.StatefulDirs, derived.StatefulDirs...)
		}
		if derived.Healthcheck != nil {
			stageDef.Healthcheck = derived.Healthcheck
		}
		if len(derived.PostInstall) > 0 {
			stageDef.PostInstall = append(stageDef.PostInstall, derived.PostInstall...)
		}
		if derived.Dev != nil {
			stageDef.Dev = derived.Dev
		}
	}

	if *stageDef.Dev || stageDef.IsFrontend {
		*stageDef.Healthcheck = false
	}

	return stageDef
}
