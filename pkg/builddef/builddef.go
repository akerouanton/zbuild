package builddef

import (
	"path"
	"sort"

	"github.com/buildkite/interpolate"
	"github.com/mitchellh/hashstructure"
)

var (
	ZbuildLabel = "io.zbuild"
)

// BuildDef is the generic data structure holding a generic build definition.
// It's composed of a kind, the only generic property, and of a RawConfig,
// which holds all the specialized properties (depending on the Kind). Also,
// it contains the RawLocks associated with the RawConfig.
type BuildDef struct {
	// Kind property which represent the type of specialized build definition
	// the RawConfig property is holding.
	Kind string `yaml:"kind"`
	// RawConfig is the map of properties used by that specific Kind of
	// specialized build definition.
	RawConfig map[string]interface{} `yaml:",inline" hash:"set"`
	// RawLocks holds the map of locked properties used by that specific Kind
	// of specialized build definition.
	RawLocks RawLocks `yaml:"-" hash:"-"`
}

// Hash returns a FNV hash of the BuildDef struct. This is used to ensure that
// the Locks aren't out-of-sync with the BuildDef.
func (def BuildDef) Hash() uint64 {
	hash, _ := hashstructure.Hash(def, nil)
	return hash
}

// RawLocks holds the hash of the BuildDef these RawLocks are associated to,
// as well as a raw map of all the locked properties.
type RawLocks struct {
	// DefHash is the hash of the BuildDef the last time the locks were
	// generated. This is used to ensure that the lockfile is up-to-date with
	// the BuildDef. It's the only generic lock property, extracted from the
	// lockfile when loaded.
	DefHash uint64                 `yaml:"defhash"`
	Raw     map[string]interface{} `yaml:",inline"`
}

// Locks define a common interface implemented by all specialized Locks structs.
// Its unique method returns the locks as a map of interfaces, as used by
// mapstructure. This lets builder package arbitrarily manipulate the locks
// before writing them to disk. This is used to add the webserver locks
// when a webserver definition is embedded in a zbuildfile of another kind.
type Locks interface {
	RawLocks() map[string]interface{}
}

// VersionMap is a list of packages/extensions/etc... associated to version
// constraints or resolved versions. It has dedicated methods to manipulate
// items in the map, most notably Add() which can be used to add an item
// without overwriting any preexisting value. This is used in specialized
// defkind handlers to add infered items without rewriting manually-defined
// values.
type VersionMap map[string]string

// Len returns the number of versions in the map or 0 if the map is nil.
func (set *VersionMap) Len() int {
	if set == nil {
		return 0
	}

	return len(*set)
}

func (set *VersionMap) Add(name, val string) {
	if set == nil {
		return
	}
	if _, ok := (*set)[name]; ok {
		return
	}
	(*set)[name] = val
}

func (set *VersionMap) Overwrite(name, val string) {
	if set == nil {
		return
	}
	(*set)[name] = val
}

func (set *VersionMap) Remove(name string) {
	if set == nil {
		return
	}
	if _, ok := (*set)[name]; ok {
		delete(*set, name)
	}
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
		set.Overwrite(name, val)
	}
}

// PathsMap represents a set of paths with the source paths as keys and dest
// paths as values. It provides POSIX-like parameter expansion through its
// Interpolate() methods.
type PathsMap map[string]string

// SourcePaths() returns the sorted list of sources paths from the map
// prepended with the given prefix.
func (paths PathsMap) SourcePaths(srcPrefix string) []string {
	sources := make([]string, 0, len(paths))

	for src := range paths {
		sources = append(sources, path.Join(srcPrefix, src))
	}

	sort.Strings(sources)
	return sources
}

// Interpolate expands POSIX-like parameters from the dest paths of the map
// using the given map of parameters and prepends all the source paths with
// the given prefix. Interpolated dest paths that aren't absolute are prefixed
// with the given basedir.
func (paths PathsMap) Interpolate(
	srcPrefix string,
	basedir string,
	params map[string]string,
) (map[string]string, error) {
	interpolated := make(PathsMap)
	paramsEnv := interpolate.NewMapEnv(params)

	for src, dest := range paths {
		var err error
		dest, err = interpolate.Interpolate(paramsEnv, dest)
		if err != nil {
			return interpolated, err
		}

		if !path.IsAbs(dest) {
			dest = path.Join(basedir, dest)
		}

		src = path.Join(srcPrefix, src)
		interpolated[src] = dest
	}

	return interpolated, nil
}

func (paths PathsMap) Copy() PathsMap {
	new := PathsMap{}
	for src, dest := range paths {
		new[src] = dest
	}
	return new
}

func (paths PathsMap) Merge(overriding PathsMap) PathsMap {
	if overriding == nil {
		overriding = PathsMap{}
	}

	knownDest := map[string]struct{}{}
	for _, dest := range overriding {
		knownDest[dest] = struct{}{}
	}

	for src, dest := range paths {
		if _, ok := knownDest[dest]; ok {
			continue
		}

		overriding[src] = dest
		knownDest[dest] = struct{}{}
	}

	return overriding
}
