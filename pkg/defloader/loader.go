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
// found in the local build context or on the filesystem.
var ZbuildfileNotFound = xerrors.New("zbuildfile not found")

const SharedKeyZbuildfiles = "zbuildfiles"

func Load(
	ctx context.Context,
	solver statesolver.StateSolver,
	buildOpts builddef.BuildOpts,
) (*builddef.BuildDef, error) {
	src := solver.FromContext(buildOpts.BuildContext,
		llb.IncludePatterns([]string{buildOpts.File, buildOpts.LockFile}),
		llb.LocalUniqueID(buildOpts.LocalUniqueID),
		llb.SessionID(buildOpts.SessionID),
		llb.SharedKeyHint(SharedKeyZbuildfiles),
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
