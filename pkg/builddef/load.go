package builddef

import (
	"context"
	"io/ioutil"
	"os"

	"github.com/NiR-/zbuild/pkg/llbutils"
	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/frontend/gateway/client"
	"golang.org/x/xerrors"
	"gopkg.in/yaml.v2"
)

// ZbuildfileNotFound is an error returned when there's no zbuild.yml file
// found in the local build context or on the filesystem.
var ZbuildfileNotFound = xerrors.New("zbuildfile not found")

// LoadFromContext loads zbuild.yml from build context using Buildkit client.
// @TODO: read files from git context instead of local source?
func LoadFromContext(
	ctx context.Context,
	c client.Client,
	buildOpts BuildOpts,
) (*BuildDef, error) {
	src := llb.Local("context",
		llb.IncludePatterns([]string{
			buildOpts.File,
			buildOpts.LockFile,
		}),
		llb.SessionID(buildOpts.SessionID),
		llb.SharedKeyHint("zbuild-config-files"),
		llb.WithCustomName("load zbuild config files from build context"))

	_, srcRef, err := llbutils.SolveState(ctx, c, src)
	if err != nil {
		return nil, xerrors.Errorf("failed to resolve build context: %w", err)
	}

	ymlContent, ok, err := llbutils.ReadFile(ctx, srcRef, buildOpts.File)
	if err != nil {
		return nil, xerrors.Errorf("could not load %s from build context: %w", buildOpts.File, err)
	} else if !ok {
		return nil, ZbuildfileNotFound
	}

	var def BuildDef
	if err = yaml.Unmarshal(ymlContent, &def); err != nil {
		return nil, xerrors.Errorf("could not decode %s: %w", buildOpts.File, err)
	}

	lockContent, ok, err := llbutils.ReadFile(ctx, srcRef, buildOpts.LockFile)
	if err != nil {
		return nil, xerrors.Errorf("could not load %s from build context: %w", buildOpts.LockFile, err)
	} else if !ok {
		return &def, nil
	}

	def.RawLocks = lockContent

	return &def, nil
}

// LoadFromFS loads zbuild config file from local filesystem. This is mostly
// useful when zbuild is running as CLI tool because it doesn't have access to a
// Buildkit client, whereas syntax providers does.
func LoadFromFS(file, lockFile string) (*BuildDef, error) {
	ymlContent, err := ioutil.ReadFile(file)
	if os.IsNotExist(err) {
		return nil, xerrors.Errorf("could not load %s: %w", file, ZbuildfileNotFound)
	} else if err != nil {
		return nil, xerrors.Errorf("could not load %s from filesystem: %w", file, err)
	}

	var def BuildDef
	if err = yaml.Unmarshal(ymlContent, &def); err != nil {
		return nil, xerrors.Errorf("could not decode %s: %w", file, err)
	}

	lockContent, err := ioutil.ReadFile(lockFile)
	if os.IsNotExist(err) {
		return &def, nil
	} else if err != nil {
		return nil, xerrors.Errorf("could not load %s from filesystem: %w", lockFile, err)
	}
	def.RawLocks = lockContent

	return &def, nil
}
