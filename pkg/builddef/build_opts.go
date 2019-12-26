package builddef

import (
	"path/filepath"
	"strings"

	"github.com/moby/buildkit/client/llb"
)

// BuildOpts represents the parameters passed to specialized builders.
// (see github.com/NiR-/zbuild/pkg/defkinds/)
type BuildOpts struct {
	Def       *BuildDef
	Source    *llb.State
	SessionID string
	// LocalUniqueID is useful mostly for test purpose, in order to use
	// a predefine value and have stable op digests.
	LocalUniqueID string
	File          string
	LockFile      string
	Stage         string
	ContextName   string
}

func NewBuildOpts(file string) BuildOpts {
	return BuildOpts{
		File:        file,
		LockFile:    LockFilepath(file),
		ContextName: "context",
	}
}

func LockFilepath(ymlFile string) string {
	ext := filepath.Ext(ymlFile)
	return strings.TrimSuffix(ymlFile, ext) + ".lock"
}
