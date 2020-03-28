package builddef

import (
	"path/filepath"
	"strings"

	"github.com/moby/buildkit/client/llb"
)

// BuildOpts represents the parameters passed to specialized builders.
// (see github.com/NiR-/zbuild/pkg/defkinds/)
type BuildOpts struct {
	Def         *BuildDef
	SourceState *llb.State
	SessionID   string
	// LocalUniqueID is useful mostly for test purpose, in order to use
	// a predefine value and have stable op digests.
	LocalUniqueID string
	// IgnoreCache determines wheter layer caching shall be disabled. It's
	// the responsibility of specialized kind handlers to correctly apply this
	// option.
	IgnoreCache bool
	// WithCacheMounts determines if the specialized builders should use a
	// custom cache to store downloaded pckages, compiled files, etc...
	WithCacheMounts  bool
	CacheIDNamespace string
	File             string
	LockFile         string
	Stage            string
	BuildContext     *Context
}

func NewBuildOpts(file, context, stage, sessionID, cacheIDNamespace string) (BuildOpts, error) {
	if context == "" {
		context = "context"
	}
	if sessionID == "" {
		sessionID = "<SESSION-ID>"
	}

	opts := BuildOpts{
		File:             file,
		LockFile:         LockFilepath(file),
		Stage:            stage,
		SessionID:        sessionID,
		WithCacheMounts:  true,
		CacheIDNamespace: cacheIDNamespace,
	}

	var err error
	opts.BuildContext, err = NewContext(context, "")

	return opts, err
}

func LockFilepath(ymlFile string) string {
	ext := filepath.Ext(ymlFile)
	return strings.TrimSuffix(ymlFile, ext) + ".lock"
}
