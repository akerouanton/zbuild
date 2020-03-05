package defloader

import (
	"context"

	"github.com/NiR-/zbuild/pkg/builddef"
	"github.com/NiR-/zbuild/pkg/statesolver"
	"github.com/moby/buildkit/client/llb"
	"golang.org/x/xerrors"
	"gopkg.in/yaml.v2"
)

// ZbuildfileNotFound is an error returned when there's no zbuild.yml file
// found in the build context.
var ZbuildfileNotFound = xerrors.New("zbuildfile not found")

const sharedKeyZbuildfiles = "zbuildfiles"

// Load uses a StateSolver to load a zbuildfile and its lockfile (as specified
// by the BuildOpts). Since a StateSolver is used, this function can be called
// either from a Buildkit builder (with no direct access to the filesystem) or
// from a CLI binary.
// ZbuildfileNotFound is returned when the zbuild file could not be found.
// However, if the lockfile is not found, the BuildDef.RawLocks property is
// left empty.
// Also, this function doesn't check if the loaded RawLocks are out-of-sync
// with the RawConfig, so it's the caller responsibility to do so.
func Load(
	ctx context.Context,
	solver statesolver.StateSolver,
	buildOpts builddef.BuildOpts,
) (*builddef.BuildDef, error) {
	src := solver.FromContext(buildOpts.BuildContext,
		llb.IncludePatterns([]string{buildOpts.File, buildOpts.LockFile}),
		llb.LocalUniqueID(buildOpts.LocalUniqueID),
		llb.SessionID(buildOpts.SessionID),
		llb.SharedKeyHint(sharedKeyZbuildfiles),
		llb.WithCustomName("load zbuild config files from build context"))

	ymlContent, err := solver.ReadFile(ctx, buildOpts.File, src)
	if err != nil && xerrors.Is(err, statesolver.FileNotFound) {
		return nil, ZbuildfileNotFound
	} else if err != nil {
		return nil, xerrors.Errorf("could not load %s: %w", buildOpts.File, err)
	}

	var def builddef.BuildDef
	if err = yaml.Unmarshal(ymlContent, &def); err != nil {
		return nil, xerrors.Errorf("could not decode %s: %w", buildOpts.File, err)
	}

	lockContent, err := solver.ReadFile(ctx, buildOpts.LockFile, src)
	if err != nil && xerrors.Is(err, statesolver.FileNotFound) {
		return &def, nil
	} else if err != nil {
		return nil, xerrors.Errorf("could not load %s: %w", buildOpts.LockFile, err)
	}

	if err = yaml.Unmarshal(lockContent, &def.RawLocks); err != nil {
		return nil, xerrors.Errorf("could not decode %s: %w", buildOpts.LockFile, err)
	}

	return &def, nil
}
