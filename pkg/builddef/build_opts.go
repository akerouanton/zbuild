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
	IgnoreCache   bool
	File          string
	LockFile      string
	Stage         string
	BuildContext  *Context
}

func NewBuildOpts(file, context, stage, sessionID string) (BuildOpts, error) {
	if context == "" {
		context = "context"
	}
	if sessionID == "" {
		sessionID = "<SESSION-ID>"
	}

	opts := BuildOpts{
		File:      file,
		LockFile:  LockFilepath(file),
		Stage:     stage,
		SessionID: sessionID,
	}

	var err error
	opts.BuildContext, err = NewContext(context, "")

	return opts, err
}

func LockFilepath(ymlFile string) string {
	ext := filepath.Ext(ymlFile)
	return strings.TrimSuffix(ymlFile, ext) + ".lock"
}
