package phpsvc

import (
	"github.com/NiR-/webdf/pkg/llbutils"
	"github.com/NiR-/webdf/pkg/service"
	"github.com/mitchellh/mapstructure"
	dpkg "github.com/snyh/go-dpkg-parser"
	"golang.org/x/xerrors"
)

// RegisterService adds a LLB DAG builder for php service type to the given
// TypeRegistry.
func RegisterService(registry *service.TypeRegistry) {
	registry.Register("php", PHPHandler{})
}

type PHPHandler struct{}

var defaultCfg = ServiceConfig{
	BaseConfig: service.BaseConfig{
		ExternalFiles:  []llbutils.ExternalFile{},
		SystemPackages: map[string]string{},
	},
	FPM:        true,
	Extensions: map[string]string{},
	ComposerDumpFlags: ComposerDumpFlags{
		ClassmapAuthoritative: true,
	},
	Healthcheck: true,
}

// ServiceConfig holds configuration parameters for PHP services.
type ServiceConfig struct {
	service.BaseConfig `mapstructure:",squash"`

	Version           string
	FPM               bool `mapstructure:",omitempty"`
	Extensions        map[string]string
	IniFile           string            `mapstructure:"ini_file"`
	FPMConfigFile     string            `mapstructure:"fpm_config_file"`
	ComposerDumpFlags ComposerDumpFlags `mapstructure:",omitempty"`
	SourceDirs        []string          `mapstructure:"source_dirs"`
	ExtraScripts      []string          `mapstructure:"extra_scripts"`
	Integrations      []string
	StatefulDirs      []string `mapstructure:"stateful_dirs"`
	Healthcheck       bool

	Locks ServiceLocks `mapstructure:"-"`
}

// ComposerDumpFlags represents the optimization flags taken by Composer for
// `composer dump-autoloader`. Only advanced optimizations can be enabled, as
// the --optimize flag is automatically added whenever building images, except
// for dev stage (see cacheWarmup()).
type ComposerDumpFlags struct {
	// APCU enables --apcu flag during composer dump (will use APCu to cache found/not found classes)
	APCU bool
	// ClassmapAuthoritative enables the matching optimization flag during composer dump.
	ClassmapAuthoritative bool
}

func addIntegrations(cfg *ServiceConfig) {
	for _, integration := range cfg.Integrations {
		switch integration {
		case "blackfire":
			// @OTODO: Add blackfire checksum
			cfg.ExternalFiles = append(cfg.ExternalFiles, llbutils.ExternalFile{
				URL:         "https://blackfire.io/api/v1/releases/probe/php/linux/amd64/72",
				Compressed:  true,
				Pattern:     "blackfire-*.so",
				Destination: "/usr/local/lib/php/extensions/no-debug-non-zts-20170718/blackfire.so",
				Mode:        0644,
			})
		}
	}

	if cfg.Healthcheck {
		cfg.ExternalFiles = append(cfg.ExternalFiles, llbutils.ExternalFile{
			URL:         "https://github.com/NiR-/fcgi-client/releases/download/v0.1.0/fcgi-client.phar",
			Destination: "/usr/local/bin/fcgi-client",
			Mode:        0750,
			Owner:       "1000:1000",
		})
	}
}

// ServiceLocks define version locks for system packages and PHP extensions used
// by PHP services.
type ServiceLocks struct {
	service.BaseLocks `mapstructure:",squash"`

	Extensions map[string]string
}

func (l ServiceLocks) Raw() map[string]interface{} {
	raw := map[string]interface{}{
		"system_packages": l.SystemPackages,
		"extensions":      l.Extensions,
	}
	return raw
}

func (h PHPHandler) UpdateLocks(svc *service.Service, repo *dpkg.Repository) error {
	svcCfg := defaultCfg
	if err := mapstructure.Decode(svc.RawConfig, &svcCfg); err != nil {
		return xerrors.Errorf("could not decode config for service %q: %v", svc.Name, err)
	}

	svcCfg.Locks = ServiceLocks{}
	if err := mapstructure.Decode(svc.RawLocks, &svcCfg.Locks); err != nil {
		return xerrors.Errorf("could not decode version locks for service %q: %v", svc.Name, err)
	}

	if err := loadPlatformReqsFromFS(&svcCfg); err != nil {
		return xerrors.Errorf("could not load platform-reqs from composer.lock: %v", err)
	}

	addIntegrations(&svcCfg)
	// @TODO: add stage argument
	inferExtensions(&svcCfg, "dev")
	inferSystemPackages(&svcCfg)

	var err error
	// @TODO: guess dpkg suite/archive to use from the base service image
	suites := [][]string{
		{"http://deb.debian.org/debian", "jessie"},
		{"http://deb.debian.org/debian", "jessie-updates"},
		{"http://security.debian.org", "jessie/updates"},
	}
	svcCfg.Locks.SystemPackages, err = service.ResolvePackageVersions(svcCfg.SystemPackages, repo, suites, "amd64")
	if err != nil {
		return xerrors.Errorf("could not resolve system package versions: %v", err)
	}

	svcCfg.Locks.Extensions, err = findExtensionVersions(svcCfg.Extensions)
	if err != nil {
		return xerrors.Errorf("could not resolve php extension versions: %v", err)
	}

	svc.RawLocks = svcCfg.Locks.Raw()

	return nil
}

func findExtensionVersions(extensions map[string]string) (map[string]string, error) {
	return extensions, nil
}
