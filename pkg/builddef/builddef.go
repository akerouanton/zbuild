package builddef

// BuildDef represents a service as declared in zbuild config file.
type BuildDef struct {
	Kind      string                 `yaml:"kind"`
	RawConfig map[string]interface{} `yaml:",inline"`
	RawLocks  []byte                 `yaml:"-"`
}

// Locks define a common interface implemented by all specialized Locks structs.
// Its unique method returns the slice of bytes that should be written in the lock file.
type Locks interface {
	RawLocks() ([]byte, error)
}
