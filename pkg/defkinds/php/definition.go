package php

import (
	"fmt"
	"path"
	"strings"

	"github.com/NiR-/zbuild/pkg/builddef"
	"github.com/NiR-/zbuild/pkg/defkinds/webserver"
	"github.com/NiR-/zbuild/pkg/llbutils"
	"github.com/mcuadros/go-version"
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
	isDev := true
	isNotDev := false

	return Definition{
		BaseStage: Stage{
			ExternalFiles:  []llbutils.ExternalFile{},
			SystemPackages: map[string]string{},
			FPM:            &fpm,
			Extensions:     map[string]string{},
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
				Dev:        &isDev,
			},
			"prod": {
				DeriveFrom: "base",
				Dev:        &isNotDev,
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

	def.MajMinVersion = extractMajMinVersion(def.Version)

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
	Webserver     *webserver.Definition   `mapstructure:"webserver"`

	Locks DefinitionLocks `mapstructure:"-"`
}

// Stage holds all the properties from the base stage that could also be
// overriden by derived stages.
type Stage struct {
	ExternalFiles     []llbutils.ExternalFile `mapstructure:"external_files"`
	SystemPackages    map[string]string       `mapstructure:"system_packages"`
	FPM               *bool                   `mapstructure:",omitempty"`
	Command           *[]string               `mapstrture:"command,omitempty"`
	Extensions        map[string]string       `mapstructure:"extensions"`
	ConfigFiles       PHPConfigFiles          `mapstructure:"config_files"`
	ComposerDumpFlags *ComposerDumpFlags      `mapstructure:"composer_dump"`
	SourceDirs        []string                `mapstructure:"source_dirs"`
	ExtraScripts      []string                `mapstructure:"extra_scripts"`
	Integrations      []string                `mapstructure:"integrations"`
	StatefulDirs      []string                `mapstructure:"stateful_dirs"`
	Healthcheck       *bool                   `mapstructure:"healthcheck"`
	PostInstall       []string                `mapstructure:"post_install"`
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
type StageDefinition struct {
	Stage
	Name           string
	BaseImage      string
	Version        string
	MajMinVersion  string
	Infer          bool
	Dev            *bool
	LockedPackages map[string]string
	PlatformReqs   map[string]string
	Webserver      *webserver.Definition
}

func (def *Definition) ResolveStageDefinition(
	name string,
	composerLockLoader func(*StageDefinition) error,
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

	if err := composerLockLoader(&stageDef); err != nil {
		return stageDef, err
	}

	if *stageDef.FPM == false && stageDef.Command == nil {
		return stageDef, xerrors.New("FPM mode is disabled but no command was provided")
	}

	if err := addIntegrations(&stageDef); err != nil {
		return stageDef, err
	}

	if !def.Infer {
		return stageDef, nil
	}

	if *stageDef.Dev == false {
		stageDef.Extensions["apcu"] = "*"
		stageDef.Extensions["opcache"] = "*"
	}
	for name, constraint := range stageDef.PlatformReqs {
		if _, ok := stageDef.Extensions[name]; !ok {
			stageDef.Extensions[name] = constraint
		}
	}

	inferExtensions(&stageDef)
	inferSystemPackages(&stageDef)

	return stageDef, nil
}

func extractMajMinVersion(versionString string) string {
	segments := strings.SplitN(versionString, ".", 3)
	return fmt.Sprintf("%s.%s", segments[0], segments[1])
}

func mergeStages(base *Definition, stages ...DerivedStage) StageDefinition {
	dev := false
	stageDef := StageDefinition{
		BaseImage:      base.BaseImage,
		Version:        base.Version,
		MajMinVersion:  base.MajMinVersion,
		Infer:          base.Infer,
		Stage:          base.BaseStage,
		Dev:            &dev,
		PlatformReqs:   map[string]string{},
		LockedPackages: map[string]string{},
		Webserver:      base.Webserver,
	}

	stages = reverseStages(stages)
	for _, stage := range stages {
		if len(stage.ExternalFiles) > 0 {
			stageDef.ExternalFiles = append(stageDef.ExternalFiles, stage.ExternalFiles...)
		}
		if len(stage.SystemPackages) > 0 {
			for name, constraint := range stage.SystemPackages {
				stageDef.SystemPackages[name] = constraint
			}
		}
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

	if *stageDef.FPM == false {
		stageDef.ConfigFiles.FPMConfigFile = nil
	}
	if *stageDef.Dev == true || *stageDef.FPM == false {
		disabled := false
		stageDef.Healthcheck = &disabled
		removeIntegration(&stageDef, "blackfire")
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

var symfonySourceDirs = map[string][]string{
	"~3.0": []string{"app/", "src/"},
	"~4.0": []string{"config/", "src/", "templates/", "translations/"},
	"~5.0": []string{"config/", "src/", "templates/", "translations/"},
}

var symfonyExtraScripts = map[string][]string{
	"~3.0": []string{"bin/console", "web/app.php"},
	"~4.0": []string{"bin/console", "public/index.php"},
	"~5.0": []string{"bin/console", "public/index.php"},
}

func findSymfonySourceDirs(symfonyVer string) []string {
	for constraint, sourceDirs := range symfonySourceDirs {
		c := version.NewConstrainGroupFromString(constraint)
		if c.Match(symfonyVer) {
			return sourceDirs
		}
	}
	return []string{}
}

func findSymfonyExtraScripts(symfonyVer string) []string {
	for constraint, extraScripts := range symfonyExtraScripts {
		c := version.NewConstrainGroupFromString(constraint)
		if c.Match(symfonyVer) {
			return extraScripts
		}
	}
	return []string{}
}

func addIntegrations(def *StageDefinition) error {
	for _, integration := range def.Integrations {
		switch integration {
		case "blackfire":
			dest := path.Join(phpExtDirs[def.MajMinVersion], "blackfire.so")
			def.ExternalFiles = append(def.ExternalFiles, llbutils.ExternalFile{
				URL:         "https://blackfire.io/api/v1/releases/probe/php/linux/amd64/72",
				Compressed:  true,
				Pattern:     "blackfire-*.so",
				Destination: dest,
				Mode:        0644,
			})
		case "symfony":
			symfonyVer, ok := def.LockedPackages["symfony/framework-bundle"]
			if !ok {
				return xerrors.New("Symfony integration is enabled but symfony/framework-bundle was not found in composer.lock")
			}
			symfonyVer = strings.TrimLeft(symfonyVer, "v")

			postInstall := []string{
				"php -d display_errors=on bin/console cache:warmup --env=prod",
			}
			def.PostInstall = append(postInstall, def.PostInstall...)

			def.SourceDirs = append(def.SourceDirs,
				findSymfonySourceDirs(symfonyVer)...)
			def.ExtraScripts = append(def.ExtraScripts,
				findSymfonyExtraScripts(symfonyVer)...)
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

	return nil
}

func removeIntegration(stageDef *StageDefinition, toRemove string) {
	integrations := make([]string, len(stageDef.Integrations))
	cur := 0
	for i := 0; i < len(stageDef.Integrations); i++ {
		if stageDef.Integrations[i] == toRemove {
			continue
		}
		integrations[cur] = stageDef.Integrations[i]
		cur++
	}

	stageDef.Integrations = integrations[:cur]
}

// List of extensions preinstalled in official PHP images. Fortunately enough,
// currently all images have the same set of preinstalled extensions.
//
// This list has been obtained using:
// docker run --rm -t php:7.2-fpm-buster php -r 'var_dump(get_loaded_extensions());'
var preinstalledExtensions = []string{
	"core",
	"date",
	"libxml",
	"openssl",
	"pcre",
	"sqlite3",
	"zlib",
	"ctype",
	"curl",
	"dom",
	"fileinfo",
	"filter",
	"ftp",
	"hash",
	"iconv",
	"json",
	"mbstring",
	"spl",
	"pdo",
	"session",
	"posix",
	"readline",
	"reflection",
	"standard",
	"simplexml",
	"pdo_sqlite",
	"phar",
	"tokenizer",
	"xml",
	"xmlreader",
	"xmlwriter",
	"mysqlnd",
	"sodium",
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
