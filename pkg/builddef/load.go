package builddef

import (
	"context"
	"io/ioutil"
	"os"

	"github.com/NiR-/webdf/pkg/llbutils"
	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/frontend/gateway/client"
	"golang.org/x/xerrors"
	"gopkg.in/yaml.v2"
)

// ConfigYMLNotFound is an error returned when there's no webdf.yml file
// found in the local build context. This file is looked for when one of
// the Buildkit syntax provider (e.g. service types like php, node, etc...)
// got invoked.
var ConfigYMLNotFound = xerrors.New("webdf.yml not found in build context")

// LoadFromContext loads webdf.yml from build context using Buildkit client.
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
		llb.SharedKeyHint("webdf-config-files"),
		llb.WithCustomName("load webdf config files from build context"))

	_, srcRef, err := llbutils.SolveState(ctx, c, src)
	if err != nil {
		return nil, xerrors.Errorf("failed to resolve build context: %v", err)
	}

	ymlContent, ok, err := llbutils.ReadFile(ctx, srcRef, buildOpts.File)
	if err != nil {
		return nil, xerrors.Errorf("could not load %s from build context: %v", buildOpts.File, err)
	} else if !ok {
		return nil, ConfigYMLNotFound
	}

	var def BuildDef
	if err = yaml.Unmarshal(ymlContent, &def); err != nil {
		return nil, xerrors.Errorf("could not decode %s: %v", buildOpts.File, err)
	}

	lockContent, ok, err := llbutils.ReadFile(ctx, srcRef, buildOpts.LockFile)
	if err != nil {
		return nil, xerrors.Errorf("could not load %s from build context: %v", buildOpts.LockFile, err)
	} else if !ok {
		return &def, nil
	}

	var lockCfg map[string]interface{}
	if err = yaml.Unmarshal(lockContent, &lockCfg); err != nil {
		return nil, xerrors.Errorf("could not decode %s: %v", buildOpts.LockFile, err)
	}

	def.RawLocks = lockCfg

	return &def, nil
}

// LoadFromFS loads webdf config file from local filesystem. This is mostly
// useful when webdf is running as CLI tool because it doesn't have access to a
// Buildkit client, whereas syntax providers does.
func LoadFromFS(file, lockFile string) (*BuildDef, error) {
	ymlContent, err := ioutil.ReadFile(file)
	if os.IsNotExist(err) {
		return nil, ConfigYMLNotFound
	} else if err != nil {
		return nil, xerrors.Errorf("could not load %s from filesystem: %v", file, err)
	}

	var def BuildDef
	if err = yaml.Unmarshal(ymlContent, &def); err != nil {
		return nil, xerrors.Errorf("could not decode %s: %v", file, err)
	}

	lockContent, err := ioutil.ReadFile(lockFile)
	if os.IsNotExist(err) {
		return &def, nil
	} else if err != nil {
		return nil, xerrors.Errorf("could not load %s from filesystem: %v", lockFile, err)
	}

	var lockCfg map[string]interface{}
	if err = yaml.Unmarshal(lockContent, &lockCfg); err != nil {
		return nil, xerrors.Errorf("could not decode %s: %v", lockFile, err)
	}

	def.RawLocks = lockCfg

	return &def, nil
}
