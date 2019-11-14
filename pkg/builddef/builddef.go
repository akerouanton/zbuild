package builddef

import (
	"github.com/NiR-/webdf/pkg/llbutils"
)

// BuildDef represents a service as declared in webdf config file.
type BuildDef struct {
	Type      string                 `yaml:"type"`
	RawConfig map[string]interface{} `yaml:",inline"`
	RawLocks  map[string]interface{} `yaml:"-"`
}

// BaseConfig exposes fields shared by all/most specific config structs.
type BaseConfig struct {
	ExternalFiles  []llbutils.ExternalFile
	SystemPackages map[string]string `mapstructure:"system_packages"`
}

// BaseLocks exposes fields shared by all/most service locks.
type BaseLocks struct {
	SystemPackages map[string]string `mapstructure:"system_packages"`
}

// Locks define a common interface implemented by all specialized Locks structs.
// Its unique method returns the slice of bytes that should be written in the lock file.
type Locks interface {
	RawLocks() ([]byte, error)
}
