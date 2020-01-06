package php

import (
	"context"
	"fmt"
	"path"
	"strings"

	"github.com/NiR-/zbuild/pkg/builddef"
	"github.com/NiR-/zbuild/pkg/defkinds/webserver"
	"github.com/NiR-/zbuild/pkg/llbutils"
	"github.com/mitchellh/mapstructure"
	"golang.org/x/xerrors"
	"gopkg.in/yaml.v2"
)

func (h *PHPHandler) loadDefs(
	ctx context.Context,
	buildOpts builddef.BuildOpts,
) (Definition, StageDefinition, error) {
	var def Definition
	var stageDef StageDefinition

	def, err := NewKind(buildOpts.Def)
	if err != nil {
		return def, stageDef, err
	}

	stageName := buildOpts.Stage
	if strings.HasPrefix(stageName, "webserver-") {
		stageName = strings.TrimPrefix(stageName, "webserver-")
	}

	composerLockLoader := func(stageDef *StageDefinition) error {
		return LoadComposerLock(ctx, h.solver, stageDef)
	}

	stageDef, err = def.ResolveStageDefinition(stageName, composerLockLoader, true)
	if err != nil {
		return def, stageDef, xerrors.Errorf("could not resolve stage %q: %w", stageName, err)
	}

	return def, stageDef, nil
}

// DefaultDefinition returns a Definition with all its fields initialized with
// default values.
func DefaultDefinition() Definition {
	fpm := true
	healthcheck := false
	infer := true
	isDev := true
	isNotDev := false

	return Definition{
		BaseStage: Stage{
			ExternalFiles:  []llbutils.ExternalFile{},
			SystemPackages: &builddef.VersionMap{},
			FPM:            &fpm,
			Extensions:     &builddef.VersionMap{},
			GlobalDeps:     &builddef.VersionMap{},
			ComposerDumpFlags: &ComposerDumpFlags{
				ClassmapAuthoritative: true,
			},
			Sources:      []string{},
			Integrations: []string{},
			StatefulDirs: []string{},
			Healthcheck:  &healthcheck,
			PostInstall:  []string{},
		},
		BaseImage: "",
		Infer:     &infer,
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
	var def Definition
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

	def = DefaultDefinition().Merge(def)

	if err := yaml.Unmarshal(genericDef.RawLocks, &def.Locks); err != nil {
		err := xerrors.Errorf("could not decode lock manifest: %w", err)
		return def, err
	}

	if def.Webserver != nil {
		*def.Webserver = webserver.DefaultDefinition().Merge(*def.Webserver)
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

	BaseImage     string                `mapstructure:"base"`
	Version       string                `mapstructure:"version"`
	MajMinVersion string                `mapstructure:"-"`
	Infer         *bool                 `mapstructure:"infer"`
	Stages        DerivedStageSet       `mapstructure:"stages"`
	Webserver     *webserver.Definition `mapstructure:"webserver"`

	Locks DefinitionLocks `mapstructure:"-"`
}

func (def Definition) Copy() Definition {
	new := Definition{
		BaseStage: def.BaseStage.Copy(),
		BaseImage: def.BaseImage,
		Version:   def.Version,
		Stages:    def.Stages.Copy(),
	}

	if def.Infer != nil {
		infer := *def.Infer
		new.Infer = &infer
	}
	if def.Webserver != nil {
		webserver := *def.Webserver
		new.Webserver = &webserver
	}

	return new
}

func (base Definition) Merge(overriding Definition) Definition {
	new := base.Copy()

	new.BaseStage = new.BaseStage.Merge(overriding.BaseStage)
	new.Stages = new.Stages.Merge(overriding.Stages)
	new.BaseImage = overriding.BaseImage
	new.Version = overriding.Version

	if overriding.Infer != nil {
		infer := *overriding.Infer
		new.Infer = &infer
	}
	if overriding.Webserver != nil {
		webserver := overriding.Webserver.Copy()
		if new.Webserver != nil {
			webserver = new.Webserver.Merge(*overriding.Webserver)
		}
		new.Webserver = &webserver
	}

	return new
}

// Stage holds all the properties from the base stage that could also be
// overriden by derived stages.
type Stage struct {
	ExternalFiles     []llbutils.ExternalFile `mapstructure:"external_files"`
	SystemPackages    *builddef.VersionMap    `mapstructure:"system_packages"`
	FPM               *bool                   `mapstructure:",omitempty"`
	Command           *[]string               `mapstructure:"command"`
	Extensions        *builddef.VersionMap    `mapstructure:"extensions"`
	GlobalDeps        *builddef.VersionMap    `mapstructure:"global_deps"`
	ConfigFiles       PHPConfigFiles          `mapstructure:"config_files"`
	ComposerDumpFlags *ComposerDumpFlags      `mapstructure:"composer_dump"`
	Sources           []string                `mapstructure:"sources"`
	Integrations      []string                `mapstructure:"integrations"`
	StatefulDirs      []string                `mapstructure:"stateful_dirs"`
	Healthcheck       *bool                   `mapstructure:"healthcheck"`
	PostInstall       []string                `mapstructure:"post_install"`
}

func (s Stage) Copy() Stage {
	new := Stage{
		ExternalFiles:     make([]llbutils.ExternalFile, len(s.ExternalFiles)),
		SystemPackages:    s.SystemPackages.Copy(),
		FPM:               s.FPM,
		Command:           s.Command,
		Extensions:        s.Extensions.Copy(),
		GlobalDeps:        s.GlobalDeps.Copy(),
		ConfigFiles:       s.ConfigFiles.Copy(),
		ComposerDumpFlags: s.ComposerDumpFlags,
		Sources:           make([]string, len(s.Sources)),
		Integrations:      make([]string, len(s.Integrations)),
		StatefulDirs:      make([]string, len(s.StatefulDirs)),
		Healthcheck:       s.Healthcheck,
		PostInstall:       make([]string, len(s.PostInstall)),
	}

	copy(new.ExternalFiles, s.ExternalFiles)
	copy(new.Sources, s.Sources)
	copy(new.Integrations, s.Integrations)
	copy(new.StatefulDirs, s.StatefulDirs)
	copy(new.PostInstall, s.PostInstall)

	return new
}

func (s Stage) Merge(overriding Stage) Stage {
	new := s.Copy()
	new.ExternalFiles = append(new.ExternalFiles,
		overriding.ExternalFiles...)
	new.ConfigFiles = new.ConfigFiles.Merge(overriding.ConfigFiles)
	new.Sources = append(new.Sources, overriding.Sources...)
	new.Integrations = append(new.Integrations, overriding.Integrations...)
	new.StatefulDirs = append(new.StatefulDirs, overriding.StatefulDirs...)
	new.PostInstall = append(new.PostInstall, overriding.PostInstall...)

	new.SystemPackages.Merge(overriding.SystemPackages)
	new.GlobalDeps.Merge(overriding.GlobalDeps)
	new.Extensions.Merge(overriding.Extensions)

	if overriding.FPM != nil {
		fpm := *overriding.FPM
		new.FPM = &fpm
	}
	if overriding.Command != nil {
		cmd := *overriding.Command
		new.Command = &cmd
	}
	if overriding.ComposerDumpFlags != nil {
		dumpFlags := *overriding.ComposerDumpFlags
		new.ComposerDumpFlags = &dumpFlags
	}
	if overriding.Healthcheck != nil {
		healthcheck := *overriding.Healthcheck
		new.Healthcheck = &healthcheck
	}

	return new
}

type DerivedStage struct {
	Stage `mapstructure:",squash"`

	DeriveFrom string `mapstructure:"derive_from"`
	// Dev marks if this is a dev stage (with lighter build process). It's used
	// as a pointer to distinguish when the value is nil or when it's false. In
	// the former case, the value from the parent stage is used.
	Dev *bool `mapstructure:"dev"`
}

func (s DerivedStage) Copy() DerivedStage {
	new := DerivedStage{
		Stage:      s.Stage.Copy(),
		DeriveFrom: s.DeriveFrom,
	}

	if s.Dev != nil {
		devMode := *s.Dev
		new.Dev = &devMode
	}

	return new
}

func (s DerivedStage) Merge(overriding DerivedStage) DerivedStage {
	new := s.Copy()

	new.Stage = s.Stage.Merge(overriding.Stage)
	new.DeriveFrom = overriding.DeriveFrom

	if overriding.Dev != nil {
		devMode := *overriding.Dev
		new.Dev = &devMode
	}

	return new
}

type DerivedStageSet map[string]DerivedStage

func (set DerivedStageSet) Copy() DerivedStageSet {
	new := DerivedStageSet{}

	for name, stage := range set {
		new[name] = stage.Copy()
	}

	return new
}

func (base DerivedStageSet) Merge(overriding DerivedStageSet) DerivedStageSet {
	new := base.Copy()

	for name, stage := range overriding {
		if _, ok := new[name]; !ok {
			new[name] = stage
		} else {
			new[name] = new[name].Merge(stage)
		}
	}

	return new
}

type PHPConfigFiles struct {
	IniFile       *string `mapstructure:"php.ini"`
	FPMConfigFile *string `mapstructure:"fpm.conf"`
}

func (base PHPConfigFiles) Copy() PHPConfigFiles {
	new := PHPConfigFiles{}

	if base.IniFile != nil {
		iniFile := *base.IniFile
		new.IniFile = &iniFile
	}
	if base.FPMConfigFile != nil {
		fpmConfigFile := *base.FPMConfigFile
		new.FPMConfigFile = &fpmConfigFile
	}

	return new
}

func (base PHPConfigFiles) Merge(overriding PHPConfigFiles) PHPConfigFiles {
	new := base.Copy()

	if overriding.IniFile != nil {
		iniFile := *overriding.IniFile
		new.IniFile = &iniFile
	}
	if overriding.FPMConfigFile != nil {
		fpmConfigFile := *overriding.FPMConfigFile
		new.FPMConfigFile = &fpmConfigFile
	}

	return new
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
	Dev            bool
	LockedPackages map[string]string
	PlatformReqs   map[string]string
	Webserver      *webserver.Definition
	Locks          StageLocks
}

func (def *Definition) ResolveStageDefinition(
	stageName string,
	composerLockLoader func(*StageDefinition) error,
	withLocks bool,
) (StageDefinition, error) {
	var stageDef StageDefinition
	stages, err := def.resolveStageChain(stageName)
	if err != nil {
		return stageDef, err
	}

	stageDef = mergeStages(def, stages...)
	stageDef.Name = stageName

	// @TODO: this should not be called here as composer.lock content
	// won't change between stage resolution
	if err := composerLockLoader(&stageDef); err != nil {
		return stageDef, err
	}

	if *stageDef.FPM == false && stageDef.Command == nil {
		return stageDef, xerrors.New("FPM mode is disabled but no command was provided")
	}

	if err := addIntegrations(&stageDef); err != nil {
		return stageDef, err
	}

	if def.Infer == nil || *def.Infer == false {
		return stageDef, nil
	}

	if stageDef.Dev == false && *stageDef.FPM == true {
		stageDef.Extensions.Add("apcu", "*")
		stageDef.Extensions.Add("opcache", "*")
	}
	for name, constraint := range stageDef.PlatformReqs {
		stageDef.Extensions.Add(name, constraint)
	}

	inferExtensions(&stageDef)
	inferSystemPackages(&stageDef)

	if !withLocks {
		return stageDef, nil
	}

	locks, ok := def.Locks.Stages[stageName]
	if !ok {
		return stageDef, xerrors.Errorf(
			"no locks available for stage %q. Please update your lockfile", stageName)
	}
	stageDef.Locks = locks

	return stageDef, nil
}

func (def *Definition) resolveStageChain(name string) ([]DerivedStage, error) {
	stages := make([]DerivedStage, 0, len(def.Stages))
	resolvedStages := map[string]struct{}{}
	current := name

	for current != "" && current != "base" {
		if _, ok := resolvedStages[current]; ok {
			return stages, xerrors.Errorf(
				"there's a cyclic dependency between %q and itself", current)
		}

		stage, ok := def.Stages[current]
		if !ok {
			return stages, xerrors.Errorf("stage %q not found", current)
		}

		stages = append(stages, stage)
		resolvedStages[current] = struct{}{}
		current = stage.DeriveFrom
	}

	return stages, nil
}

func extractMajMinVersion(versionString string) string {
	segments := strings.SplitN(versionString, ".", 3)
	return fmt.Sprintf("%s.%s", segments[0], segments[1])
}

func mergeStages(base *Definition, stages ...DerivedStage) StageDefinition {
	stageDef := StageDefinition{
		BaseImage:      base.BaseImage,
		Version:        base.Version,
		MajMinVersion:  base.MajMinVersion,
		Stage:          base.BaseStage.Copy(),
		PlatformReqs:   map[string]string{},
		LockedPackages: map[string]string{},
		Webserver:      base.Webserver,
	}
	if base.Infer != nil {
		stageDef.Infer = *base.Infer
	}

	stages = reverseStages(stages)
	for _, stage := range stages {
		stageDef.Stage = stageDef.Stage.Merge(stage.Stage)

		if stage.Dev != nil {
			stageDef.Dev = *stage.Dev
		}
	}

	if *stageDef.FPM == false {
		stageDef.ConfigFiles.FPMConfigFile = nil
	}
	if stageDef.Dev || *stageDef.FPM {
		healthcheck := false
		stageDef.Healthcheck = &healthcheck
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

func addIntegrations(stageDef *StageDefinition) error {
	for _, integration := range stageDef.Integrations {
		switch integration {
		case "blackfire":
			dest := path.Join(phpExtDirs[stageDef.MajMinVersion], "blackfire.so")
			stageDef.ExternalFiles = append(stageDef.ExternalFiles, llbutils.ExternalFile{
				URL:         "https://blackfire.io/api/v1/releases/probe/php/linux/amd64/72",
				Compressed:  true,
				Pattern:     "blackfire-*.so",
				Destination: dest,
				Mode:        0644,
			})
		}
	}

	if *stageDef.Healthcheck {
		stageDef.ExternalFiles = append(stageDef.ExternalFiles, llbutils.ExternalFile{
			URL:         "https://github.com/NiR-/fcgi-client/releases/download/v0.1.0/fcgi-client.phar",
			Destination: "/usr/local/bin/fcgi-client",
			Mode:        0750,
			Owner:       "1000:1000",
		})
	}

	return nil
}

// List of extensions preinstalled in official PHP images. Fortunately enough,
// currently all images have the same set of preinstalled extensions.
//
// This list has been obtained using:
// docker run --rm -t php:7.2-fpm-buster php -r 'var_dump(get_loaded_extensions());'
var preinstalledExtensions = map[string]struct{}{
	"core":       struct{}{},
	"ctype":      struct{}{},
	"curl":       struct{}{},
	"date":       struct{}{},
	"dom":        struct{}{},
	"fileinfo":   struct{}{},
	"filter":     struct{}{},
	"ftp":        struct{}{},
	"hash":       struct{}{},
	"iconv":      struct{}{},
	"json":       struct{}{},
	"libxml":     struct{}{},
	"mbstring":   struct{}{},
	"mysqlnd":    struct{}{},
	"openssl":    struct{}{},
	"pcre":       struct{}{},
	"pdo":        struct{}{},
	"pdo_sqlite": struct{}{},
	"phar":       struct{}{},
	"posix":      struct{}{},
	"readline":   struct{}{},
	"reflection": struct{}{},
	"session":    struct{}{},
	"simplexml":  struct{}{},
	"sodium":     struct{}{},
	"spl":        struct{}{},
	"sqlite3":    struct{}{},
	"standard":   struct{}{},
	"tokenizer":  struct{}{},
	"xml":        struct{}{},
	"xmlreader":  struct{}{},
	"xmlwriter":  struct{}{},
	"zlib":       struct{}{},
}

func inferExtensions(def *StageDefinition) {
	// soap extension needs sockets extension to work properly
	if def.Extensions.Has("soap") {
		def.Extensions.Add("sockets", "*")
	}

	// Add zip extension if it's missing as it's used by composer to install packages.
	def.Extensions.Add("zip", "*")
}

func inferSystemPackages(def *StageDefinition) {
	// Add libpcre by default, as most frameworks/CMSes are using regexp
	def.SystemPackages.Add("libpcre3-dev", "*")

	for _, ext := range def.Extensions.Names() {
		deps, ok := extensionsDeps[ext]
		if !ok {
			continue
		}

		for name, ver := range deps {
			def.SystemPackages.Add(name, ver)
		}
	}

	// Add unzip and git packages as they're used by Composer
	def.SystemPackages.Add("unzip", "*")
	def.SystemPackages.Add("git", "*")
}
