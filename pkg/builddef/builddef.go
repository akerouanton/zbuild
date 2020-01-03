package builddef

var (
	ZbuildLabel = "io.zbuild"
)

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

// VersionMap is a list of packages/extensions/etc... associated to version
// constraints or resolved versions. It has dedicated methods to manipulate
// items in the map, most notably Add() which can be used to add an item
// without overwriting any preexisting value. This is used in specialized
// defkind handlers to add infered items without rewriting manually-defined
// values.
type VersionMap map[string]string

func (set *VersionMap) Add(name, val string, overwrite bool) {
	if set == nil {
		return
	}
	if _, ok := (*set)[name]; ok && !overwrite {
		return
	}
	(*set)[name] = val
}

func (set *VersionMap) Remove(name string) {
	if set == nil {
		return
	}
	delete(*set, name)
}

func (set *VersionMap) Has(name string) bool {
	if set == nil {
		return false
	}
	_, ok := (*set)[name]
	return ok
}

func (set *VersionMap) Names() []string {
	if set == nil {
		return []string{}
	}

	names := make([]string, 0, len(*set))
	for extName := range *set {
		names = append(names, extName)
	}

	return names
}

func (set *VersionMap) Map() map[string]string {
	if set == nil {
		return map[string]string{}
	}
	return *set
}

func (set *VersionMap) Copy() *VersionMap {
	if set == nil {
		return &VersionMap{}
	}

	new := VersionMap{}
	for name, val := range *set {
		new[name] = val
	}

	return &new
}

func (set *VersionMap) Merge(overriding *VersionMap) {
	if set == nil || overriding == nil {
		return
	}
	for name, val := range *overriding {
		set.Add(name, val, true)
	}
}
