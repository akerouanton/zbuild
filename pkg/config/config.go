package config

import (
	"context"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"

	"github.com/NiR-/webdf/pkg/llbutils"
	"github.com/NiR-/webdf/pkg/service"
	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/frontend/gateway/client"
	"golang.org/x/xerrors"
	"gopkg.in/yaml.v2"
)

const (
	filepathYML  = "webdf.yml"
	filepathLock = "webdf.lock"
)

// ConfigYMLNotFound is an error returned when there's no webdf.yml file
// found in the local build context. This file is looked for when one of
// the Buildkit syntax provider (e.g. service types like php, node, etc...)
// got invoked.
var ConfigYMLNotFound = xerrors.New("webdf.yml not found in build context")

// Config represents the whole webdf config file.
type Config struct {
	Services []*service.Service `yaml:"services"`
}

// LoadFromContext loads webdf.yml from build context using Buildkit client.
// @TODO: read files from git context instead of local source?
func LoadFromContext(
	ctx context.Context,
	c client.Client,
) (*Config, error) {
	src := llb.Local("context",
		llb.IncludePatterns([]string{filepathYML, filepathLock}),
		llb.SessionID(c.BuildOpts().SessionID),
		llb.SharedKeyHint("webdf-config-files"),
		llb.WithCustomName("load webdf config files from build context"))

	_, srcRef, err := llbutils.SolveState(ctx, c, src)
	if err != nil {
		return nil, xerrors.Errorf("failed to load webdf config files from build context: %v", err)
	}

	ymlContent, ok, err := llbutils.ReadFile(ctx, srcRef, filepathYML)
	if err != nil {
		return nil, xerrors.Errorf("could not load %s from build context: %v", filepathYML, err)
	} else if !ok {
		return nil, ConfigYMLNotFound
	}

	var cfg Config
	if err = yaml.Unmarshal(ymlContent, &cfg); err != nil {
		return nil, xerrors.Errorf("could not decode %s: %v", filepathYML, err)
	}

	lockContent, ok, err := llbutils.ReadFile(ctx, srcRef, filepathLock)
	if err != nil {
		return nil, xerrors.Errorf("could not load %s from build context: %v", filepathLock, err)
	} else if !ok {
		return &cfg, nil
	}

	var lockCfg map[string]map[string]interface{}
	if err = yaml.Unmarshal(lockContent, &lockCfg); err != nil {
		return nil, xerrors.Errorf("could not decode %s: %v", filepathLock, err)
	}

	for _, svc := range cfg.Services {
		if locks, ok := lockCfg[svc.Name]; ok {
			svc.RawLocks = locks
		}
	}

	return &cfg, nil
}

// LoadFromFS loads webdf config file from local filesystem. This is
// mostly useful when webdf is running as CLI tool because it has direct access
// to the FS, whereas Buildkit syntax providers don't.
//
// It loads webdf.yml config file and tries to load lock config too. It returns
// the config if the lock file couldn't be located and if the lock is found, it
// mutates services to add their lock config. It returns ConfigYMLNotFound if
// the webdf.yml couldn't be located.
func LoadFromFS(base string) (*Config, error) {
	ymlContent, err := ioutil.ReadFile(filepath.Join(base, filepathYML))
	if os.IsNotExist(err) {
		return nil, ConfigYMLNotFound
	} else if err != nil {
		return nil, xerrors.Errorf("could not load %s from filesystem: %v", filepathYML, err)
	}

	var cfg Config
	if err = yaml.Unmarshal(ymlContent, &cfg); err != nil {
		return nil, xerrors.Errorf("could not decode %s: %v", filepathYML, err)
	}

	lockContent, err := ioutil.ReadFile(filepath.Join(base, filepathLock))
	if os.IsNotExist(err) {
		return &cfg, nil
	} else if err != nil {
		return nil, xerrors.Errorf("could not load %s from filesystem: %v", filepathLock, err)
	}

	var lockCfg map[string]map[string]interface{}
	if err = yaml.Unmarshal(lockContent, &lockCfg); err != nil {
		return nil, xerrors.Errorf("could not decode %s: %v", filepathLock, err)
	}

	for _, svc := range cfg.Services {
		if locks, ok := lockCfg[svc.Name]; ok {
			svc.RawLocks = locks
		}
	}

	return &cfg, nil
}

// WriteLockFile writes the webdf.lock file into base directory. The lock file
// content is a map of services with their respective raw locks config.
func (cfg *Config) WriteLockFile(base string) error {
	locksCfg := make(map[string]interface{}, len(cfg.Services))
	for _, svc := range cfg.Services {
		locksCfg[svc.Name] = svc.RawLocks
	}

	lockdata, err := yaml.Marshal(locksCfg)
	if err != nil {
		return xerrors.Errorf("could not marshal locks cfg: %v", err)
	}

	err = ioutil.WriteFile(path.Join(base, filepathLock), lockdata, 0640)
	if err != nil {
		return xerrors.Errorf("could not write %s: %v", filepathLock, lockdata)
	}

	return nil
}

// FindService tries to find a service with the same name as the given one.
// It either returns the service if it's found or an error otherwise.
func (cfg *Config) FindService(svcName string) (*service.Service, error) {
	for _, svc := range cfg.Services {
		if svc.Name == svcName {
			return svc, nil
		}
	}

	return nil, xerrors.Errorf("service %q not found", svcName)
}
