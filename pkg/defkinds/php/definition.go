package php

import (
	"fmt"
	"path"

	"github.com/NiR-/zbuild/pkg/builddef"
	"github.com/NiR-/zbuild/pkg/llbutils"
	version "github.com/hashicorp/go-version"
	"github.com/mitchellh/mapstructure"
	"golang.org/x/xerrors"
	"gopkg.in/yaml.v2"
)

// defaultDefinition returns a Definition with all its fields initialized with
// default values.
func defaultDefinition() Definition {
	fpm := true
	healthcheck := false
	infer := true
	dev := true

	return Definition{
		BaseStage: Stage{
			BaseConfig: builddef.BaseConfig{
				ExternalFiles:  []llbutils.ExternalFile{},
				SystemPackages: map[string]string{},
			},
			FPM:        &fpm,
			Extensions: map[string]string{},
			ComposerDumpFlags: &ComposerDumpFlags{
				ClassmapAuthoritative: true,
			},
			SourceDirs:   []string{},
			ExtraScripts: []string{},
			Integrations: []string{},
			StatefulDirs: []string{},
			Healthcheck:  &healthcheck,
			PostInstall:  []string{},
		},
		BaseImage: "",
		Version:   "7.4",
		Infer:     infer,
		Stages: map[string]DerivedStage{
			"dev": {
				DeriveFrom: "base",
				Dev:        &dev,
			},
		},
	}
}

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

	majMinVersion, err := extractMajMinVersion(def.Version)
	if err != nil {
		return def, err
	}
	def.MajMinVersion = majMinVersion

	if def.BaseImage == "" {
		baseImages, ok := defaultBaseImages[def.MajMinVersion]
		if !ok {
			return def, xerrors.Errorf("no default base image defined for PHP v%s, you have to define it by yourself in your zbuild file", def.MajMinVersion)
		}
		if *def.BaseStage.FPM {
			def.BaseImage = baseImages.FPM
		} else {
			def.BaseImage = baseImages.CLI
		}
	}

	return def, nil
}

// Definition holds the specialized config parameters for php images.
// It represents the "base" stage and as such holds the PHP version (ths is the
// only parameter that can't be overriden by derived stages).
type Definition struct {
	BaseStage Stage `mapstructure:",squash"`

	BaseImage     string                  `mapstructure:"base"`
	Version       string                  `mapstructure:"version"`
	MajMinVersion string                  `mapstructure:"-"`
	Infer         bool                    `mapstructure:"infer"`
	Stages        map[string]DerivedStage `mapstructure:"stages"`

	Locks DefinitionLocks `mapstructure:"-"`
}

// Stage holds all the properties from the base stage that could also be
// overriden by derived stages.
type Stage struct {
	builddef.BaseConfig `mapstructure:",squash"`

	FPM               *bool              `mapstructure:",omitempty"`
	Command           *string            `mapstrture:"command,omitempty"`
	Extensions        map[string]string  `mapstructure:"extensions"`
	ConfigFiles       PHPConfigFiles     `mapstructure:"config_files"`
	ComposerDumpFlags *ComposerDumpFlags `mapstructure:"composer_dump"`
	SourceDirs        []string           `mapstructure:"source_dirs"`
	ExtraScripts      []string           `mapstructure:"extra_scripts"`
	Integrations      []string           `mapstructure:"integrations"`
	StatefulDirs      []string           `mapstructure:"stateful_dirs"`
	Healthcheck       *bool              `mapstructure:"healthcheck"`
	PostInstall       []string           `mapstructure:"post_install"`
}

type DerivedStage struct {
	Stage `mapstructure:",squash"`

	DeriveFrom string `mapstructure:"derive_from"`
	Dev        *bool  `mapstructure:"dev"`
}

type PHPConfigFiles struct {
	IniFile       *string `mapstructure:"php.ini"`
	FPMConfigFile *string `mapstructure:"fpm.conf"`
}

// ComposerDumpFlags represents the optimization flags taken by Composer for
// `composer dump-autoloader`. Only advanced optimizations can be enabled, as
// the --optimize flag is automatically added whenever building images, except
// for dev stage (see cacheWarmup()).
type ComposerDumpFlags struct {
	// APCU enables --apcu flag during composer dump (will use APCu to cache found/not found classes)
	APCU bool `mapstructure:"apcu"`
	// ClassmapAuthoritative enables the matching optimization flag during composer dump.
	ClassmapAuthoritative bool `mapstructure:"classmap_authoritative"`
}

func (fl ComposerDumpFlags) Flags() (string, error) {
	if fl.APCU && fl.ClassmapAuthoritative {
		return "", xerrors.New("you can't use both --apcu and --classmap-authoritative flags. See https://getcomposer.org/doc/articles/autoloader-optimization.md")
	}

	flags := "--no-dev --optimize"
	if fl.APCU {
		flags += " --apcu"
	}
	if fl.ClassmapAuthoritative {
		flags += " --classmap-authoritative"
	}
	return flags, nil
}

// StageDefinition represents the config of stage once it got merged with all
// its ancestors.
// @TODO: rename into FinalStageDefinition
type StageDefinition struct {
	Stage
	Name          string
	BaseImage     string
	Version       string
	MajMinVersion string
	Infer         bool
	Dev           *bool
}

func (def *Definition) ResolveStageDefinition(
	name string,
	platformReqsLoader func(*StageDefinition) error,
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

	if *stageDef.FPM == false && stageDef.Command == nil {
		return stageDef, xerrors.New("FPM mode is disabled but no command was provided")
	}

	if def.Infer {
		if *stageDef.FPM == false {
			stageDef.ConfigFiles.FPMConfigFile = nil
		}
		if *stageDef.Dev == true || *stageDef.FPM == false {
			disabled := false
			stageDef.Healthcheck = &disabled
		}
		if *stageDef.Dev == false {
			stageDef.Extensions["apcu"] = "*"
			stageDef.Extensions["opcache"] = "*"
		}
	}

	addIntegrations(&stageDef)

	if def.Infer {
		if err := platformReqsLoader(&stageDef); err != nil {
			return stageDef, xerrors.Errorf("could not load platform-reqs from composer.lock: %w", err)
		}

		inferExtensions(&stageDef)
		inferSystemPackages(&stageDef)
	}

	return stageDef, nil
}

func extractMajMinVersion(versionString string) (string, error) {
	ver, err := version.NewVersion(versionString)
	if err != nil {
		return "", err
	}

	segments := ver.Segments()
	return fmt.Sprintf("%d.%d", segments[0], segments[1]), nil
}

func mergeStages(base *Definition, stages ...DerivedStage) StageDefinition {
	dev := false
	stageDef := StageDefinition{
		BaseImage:     base.BaseImage,
		Version:       base.Version,
		MajMinVersion: base.MajMinVersion,
		Infer:         base.Infer,
		Stage:         base.BaseStage,
		Dev:           &dev,
	}

	stages = reverseStages(stages)
	for _, stage := range stages {
		// @TODO: merge base configs
		if stage.FPM != nil {
			stageDef.FPM = stage.FPM
		}
		if len(stage.Extensions) > 0 {
			for name, conf := range stage.Extensions {
				stageDef.Extensions[name] = conf
			}
		}
		if stage.ConfigFiles.IniFile != nil {
			stageDef.ConfigFiles.IniFile = stage.ConfigFiles.IniFile
		}
		if stage.ConfigFiles.FPMConfigFile != nil {
			stageDef.ConfigFiles.FPMConfigFile = stage.ConfigFiles.FPMConfigFile
		}
		if stage.ComposerDumpFlags != nil {
			stageDef.ComposerDumpFlags = stage.ComposerDumpFlags
		}
		if stage.SourceDirs != nil {
			stageDef.SourceDirs = append(stageDef.SourceDirs, stage.SourceDirs...)
		}
		if stage.ExtraScripts != nil {
			stageDef.ExtraScripts = append(stageDef.ExtraScripts, stage.ExtraScripts...)
		}
		if stage.Integrations != nil {
			stageDef.Integrations = append(stageDef.Integrations, stage.Integrations...)
		}
		if stage.StatefulDirs != nil {
			stageDef.StatefulDirs = append(stageDef.StatefulDirs, stage.StatefulDirs...)
		}
		if stage.Healthcheck != nil {
			stageDef.Healthcheck = stage.Healthcheck
		}
		if stage.PostInstall != nil {
			stageDef.PostInstall = append(stageDef.PostInstall, stage.PostInstall...)
		}
		if stage.Dev != nil {
			stageDef.Dev = stage.Dev
		}
		if stage.Command != nil {
			stageDef.Command = stage.Command
		}
	}

	return stageDef
}

func reverseStages(stages []DerivedStage) []DerivedStage {
	reversed := make([]DerivedStage, len(stages))
	i := 1

	for _, stage := range stages {
		id := len(stages) - i
		reversed[id] = stage
		i++

	}

	return reversed
}

var phpExtDirs = map[string]string{
	"7.2": "/usr/local/lib/php/extensions/no-debug-non-zts-20170718/",
	"7.3": "/usr/local/lib/php/extensions/no-debug-non-zts-20180731/",
	"7.4": "/usr/local/lib/php/extensions/no-debug-non-zts-20190902/",
}

func addIntegrations(def *StageDefinition) {
	for _, integration := range def.Integrations {
		switch integration {
		case "blackfire":
			if *def.Dev {
				continue
			}

			dest := path.Join(phpExtDirs[def.MajMinVersion], "blackfire.so")
			def.ExternalFiles = append(def.ExternalFiles, llbutils.ExternalFile{
				URL:         "https://blackfire.io/api/v1/releases/probe/php/linux/amd64/72",
				Compressed:  true,
				Pattern:     "blackfire-*.so",
				Destination: dest,
				Mode:        0644,
			})
		// @TODO: check symfony version
		case "symfony":
			postInstall := []string{
				"php -d display_errors=on bin/console cache:warmup --env=prod",
			}
			def.PostInstall = append(postInstall, def.PostInstall...)
			def.SourceDirs = append(def.SourceDirs, "app/", "src/")
			def.ExtraScripts = append(def.ExtraScripts, "bin/console", "web/app.php")
		}
	}

	if *def.Healthcheck {
		def.ExternalFiles = append(def.ExternalFiles, llbutils.ExternalFile{
			URL:         "https://github.com/NiR-/fcgi-client/releases/download/v0.1.0/fcgi-client.phar",
			Destination: "/usr/local/bin/fcgi-client",
			Mode:        0750,
			Owner:       "1000:1000",
		})
	}
}

func inferExtensions(def *StageDefinition) {
	// soap extension needs sockets extension to work properly
	if _, ok := def.Extensions["soap"]; ok {
		if _, ok := def.Extensions["sockets"]; !ok {
			def.Extensions["sockets"] = "*"
		}
	}

	// Add zip extension if it's missing as it's used by composer to install packages.
	if _, ok := def.Extensions["zip"]; !ok {
		def.Extensions["zip"] = "*"
	}

	// Remove extensions installed by default
	toRemove := []string{"filter", "json", "reflection", "session", "sodium", "spl", "standard"}
	for _, name := range toRemove {
		if _, ok := def.Extensions[name]; ok {
			delete(def.Extensions, name)
		}
	}
}

func inferSystemPackages(def *StageDefinition) {
	systemPackages := map[string]string{
		"libpcre3-dev": "*",
	}

	for ext := range def.Extensions {
		deps, ok := extensionsDeps[ext]
		if !ok {
			continue
		}

		for name, ver := range deps {
			systemPackages[name] = ver
		}
	}

	// Add unzip and git packages as they're used by Composer
	if _, ok := def.SystemPackages["unzip"]; !ok {
		systemPackages["unzip"] = "*"
	}
	if _, ok := def.SystemPackages["git"]; !ok {
		systemPackages["git"] = "*"
	}

	for name, constraint := range systemPackages {
		if _, ok := def.SystemPackages[name]; !ok {
			def.SystemPackages[name] = constraint
		}
	}
}
