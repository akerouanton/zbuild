package builddef

import (
	"path/filepath"
	"strings"
)

// BuildOpts represents the parameters passed to specialized builders.
// (see github.com/NiR-/zbuild/pkg/defkinds/)
// @TODO: add support for contextkey
type BuildOpts struct {
	Def       *BuildDef
	SessionID string
	// LocalUniqueID is useful mostly for test purpose, in order to use
	// a predefine value and have stable op digests.
	LocalUniqueID string
	File          string
	LockFile      string
	Stage         string
}

func NewBuildOpts(file, stage, sessionID string) BuildOpts {
	return BuildOpts{
		SessionID: sessionID,
		File:      file,
		LockFile:  LockFilepath(file),
		Stage:     stage,
	}
}

func LockFilepath(ymlFile string) string {
	ext := filepath.Ext(ymlFile)
	return strings.TrimSuffix(ymlFile, ext) + ".lock"
}
