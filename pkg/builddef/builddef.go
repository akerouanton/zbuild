package builddef

import (
	"github.com/NiR-/zbuild/pkg/llbutils"
)

// BuildDef represents a service as declared in zbuild config file.
type BuildDef struct {
	Kind      string                 `yaml:"kind"`
	RawConfig map[string]interface{} `yaml:",inline"`
	RawLocks  []byte                 `yaml:"-"`
}

// BaseConfig exposes fields shared by all/most specific config structs.
type BaseConfig struct {
	ExternalFiles  []llbutils.ExternalFile
	SystemPackages map[string]string `mapstructure:"system_packages"`
}

type BaseLocks struct {
	BaseImage string `yaml:"base_image"`
	// @TODO: is this really useful during builds? should be removed?
	OS OSRelease `yaml:"os"`
}

// BaseStageLocks exposes fields shared by all/most service locks.
type BaseStageLocks struct {
	SystemPackages map[string]string `yaml:"system_packages"`
}

// Locks define a common interface implemented by all specialized Locks structs.
// Its unique method returns the slice of bytes that should be written in the lock file.
type Locks interface {
	RawLocks() ([]byte, error)
}
