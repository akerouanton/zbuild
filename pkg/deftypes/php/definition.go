package php

import (
	"github.com/NiR-/webdf/pkg/builddef"
	"github.com/NiR-/webdf/pkg/llbutils"
	"github.com/NiR-/webdf/pkg/registry"
	"github.com/mitchellh/mapstructure"
	"golang.org/x/xerrors"
)

// RegisterDefType adds a LLB DAG builder to the given TypeRegistry for php
// definition type.
func RegisterDefType(registry *registry.TypeRegistry) {
	registry.Register("php", PHPHandler{})
}

type PHPHandler struct{}

// defaultDefinition returns a Definition with all its fields initialized with
// default values.
func defaultDefinition() Definition {
	fpm := true
	// @TODO: ensure this is a valid default version
	version := "latest"
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
		Version: version,
		Infer:   infer,
		Stages: map[string]DerivedStage{
			"dev": {
				DeriveFrom: "base",
				Dev:        &dev,
			},
		},
	}
}

func decodeGenericDef(genericDef *builddef.BuildDef) (Definition, error) {
	def := defaultDefinition()

	// @TODO: be sure to fail if there's extra props
	if err := mapstructure.Decode(genericDef.RawConfig, &def); err != nil {
		err := xerrors.Errorf("could not decode build manifest: %v", err)
		return def, err
	}

	if err := mapstructure.Decode(genericDef.RawLocks, &def.Locks); err != nil {
		err := xerrors.Errorf("could not decode lock manifest: %v", err)
		return def, err
	}

	return def, nil
}

// Definition holds the specialized config parameters for php images.
// It represents the "base" stage and as such holds the PHP version (ths is the
// only parameter that can't be overriden by derived stages).
type Definition struct {
	BaseStage Stage `mapstructure:",squash"`

	// @TODO: Add base parameter
	// BaseImage string `mapstructure:"base"`
	Version string                  `mapstructure:"version"`
	Infer   bool                    `mapstructure:"infer"`
	Stages  map[string]DerivedStage `mapstructure:"stages"`

	Locks DefinitionLocks `mapstructure:"-"`
}

// Stage holds all the properties from the base stage that could also be
// overriden by derived stages.
type Stage struct {
	builddef.BaseConfig `mapstructure:",squash"`

	FPM               *bool              `mapstructure:",omitempty"`
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

// StageDefinition represents the config of stage once it got merged with all
// its ancestors.
type StageDefinition struct {
	Stage
	Name    string
	Version string
	Infer   bool
	Dev     *bool
}

func (def *Definition) ResolveStageDefinition(name string) (StageDefinition, error) {
	var stageDef StageDefinition

	stages := make([]DerivedStage, len(def.Stages)+1)
	stageNames := make([]string, len(def.Stages)+1)
	nextStage := name

	for nextStage != "" && nextStage != "base" {
		for _, stageName := range stageNames {
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
		stageNames = append(stageNames, nextStage)
		nextStage = stage.DeriveFrom
	}

	stageDef = mergeStages(def, stages...)
	stageDef.Name = name

	return stageDef, nil
}

func mergeStages(base *Definition, stages ...DerivedStage) StageDefinition {
	dev := false
	stageDef := StageDefinition{
		Version: base.Version,
		Infer:   base.Infer,
		Stage:   base.BaseStage,
		Dev:     &dev,
	}

	stages = reverseStages(stages)
	for _, stage := range stages {
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

func addIntegrations(def *StageDefinition) {
	for _, integration := range def.Integrations {
		switch integration {
		case "blackfire":
			// @OTODO: Add blackfire checksum
			def.ExternalFiles = append(def.ExternalFiles, llbutils.ExternalFile{
				URL:        "https://blackfire.io/api/v1/releases/probe/php/linux/amd64/72",
				Compressed: true,
				Pattern:    "blackfire-*.so",
				// @TODO: get the destination path dynamically
				Destination: "/usr/local/lib/php/extensions/no-debug-non-zts-20170718/blackfire.so",
				Mode:        0644,
			})
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
	toRemove := []string{"filter", "json", "reflection", "session", "spl", "standard"}
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
		switch ext {
		case "bcmath":
			// No system dependencies
		case "bz2":
			systemPackages["libbz2-dev"] = "*"
		case "calendar":
			// No system dependencies
		case "ctype":
			// No system dependencies
		case "curl":
			systemPackages["libcurl4-openssl-dev"] = "*"
		case "dba":
			// No system dependencies
		case "dom":
			systemPackages["libxml2-dev"] = "*"
		case "enchant":
			systemPackages["libenchant-dev"] = "*"
		case "exif":
			// No system dependencies
		case "fileinfo":
			// No system dependencies
		case "filter":
			// This extension is installed by default.
		case "ftp":
			systemPackages["libssl-dev"] = "*"
		case "gd":
			systemPackages["libpng-dev"] = "*"
		case "gd.freetype":
			// @TODO: Need to set PHP_FREETYPE_DIR=/usr during docker-php-ext-install
			systemPackages["libfreetype6-dev"] = "*"
		case "gd.jpeg":
			// @TODO: Need to set PHP_JPEG_DIR=/usr during docker-php-ext-install
			systemPackages["libjpeg-dev"] = "*"
		case "gd.webp":
			// @TODO: Need to set PHP_WEBP_DIR=/usr during docker-php-ext-install
			systemPackages["libwebp-dev"] = "*"
		case "gettext":
			// No system dependencies
		case "gmp":
			systemPackages["libgmp-dev"] = "*"
		case "hash":
			// No system dependencies
		case "iconv":
			// No system dependencies
		case "imap":
			// @TODO: needs docker-php-ext-configure --with-imap-ssl --with-kerberos
			systemPackages["libc-client-dev"] = "*"
			systemPackages["libkrb5-dev"] = "*"
		case "interbase":
			// @TODO
		case "intl":
			systemPackages["libicu-dev"] = "*"
		case "json":
			// This extension is installed by default.
		case "ldap":
			systemPackages["libldap2-dev"] = "*"
		case "mbstring":
			// No system dependencies
		case "mcrypt":
			systemPackages["libmcrypt-dev"] = "*"
		case "mysqli":
			// No system dependencies
		case "oci8":
			// @TODO
		case "odbc":
			// @TODO
		case "opcache":
			// No system dependencies
		case "pcntl":
			// No system dependencies
		case "pdo":
			// No system dependencies
		case "pdo_dblib":
			// @TODO
		case "pdo_firebird":
			// @TODO
		case "pdo_mysql":
			// No system dependencies
		case "pdo_oci":
			// @TODO
		case "pdo_odbc":
			// @TODO
		case "pdo_pgsql":
			systemPackages["libpq-dev"] = "*"
		case "pdo_sqlite":
			systemPackages["libsqlite3-dev"] = "*"
		case "pgsql":
			systemPackages["libpq-dev"] = "*"
		case "phar":
			systemPackages["libssl-dev"] = "*"
		case "posix":
			// No system dependencies
		case "pspell":
			systemPackages["libpspell-dev"] = "*"
		case "readline":
			systemPackages["libedit-dev"] = "*"
		case "recode":
			systemPackages["librecode-dev"] = "*"
		case "reflection":
			// This extension is installed by default.
		case "session":
			// This extension is installed by default.
		case "shmop":
			// No system dependencies
		case "simplexml":
			systemPackages["libxml2-dev"] = "*"
		case "snmp":
			systemPackages["libsnmp-dev"] = "*"
		case "soap":
			systemPackages["libxml2-dev"] = "*"
		case "sockets":
			systemPackages["libssl-dev"] = "*"
			systemPackages["openssl"] = "*"
		case "spl":
			// This extension is installed by default.
		case "standard":
			// This extension is installed by default.
		case "sysvmsg":
			// No system dependencies
		case "sysvsem":
			// No system dependencies
		case "sysvshm":
			// No system dependencies
		case "tidy":
			systemPackages["libtidy-dev"] = "*"
		case "tokenizer":
			// No system dependencies
		case "wddx":
			systemPackages["libxml2-dev"] = "*"
		case "xml":
			systemPackages["libxml2-dev"] = "*"
		case "xmlreader":
			// @TODO: this extension seems broken (bad include statement)
		case "xmlrpc":
			systemPackages["libxml2-dev"] = "*"
		case "xmlwriter":
			systemPackages["libxml2-dev"] = "*"
		case "xsl":
			systemPackages["libxslt1-dev"] = "*"
		case "zip":
			systemPackages["zlib1g-dev"] = "*"
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
