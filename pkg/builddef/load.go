package builddef

import (
	"context"

	"github.com/NiR-/zbuild/pkg/statesolver"
	"github.com/moby/buildkit/client/llb"
	"golang.org/x/xerrors"
	"gopkg.in/yaml.v2"
)

// ZbuildfileNotFound is an error returned when there's no zbuild.yml file
// found in the local build context or on the filesystem.
var ZbuildfileNotFound = xerrors.New("zbuildfile not found")

const SharedKeyZbuildfiles = "zbuildfiles"

// @TODO: read files from git context instead of local source?
func Load(
	ctx context.Context,
	solver statesolver.StateSolver,
	buildOpts BuildOpts,
) (*BuildDef, error) {
	src := solver.FromBuildContext(
		llb.IncludePatterns([]string{
			buildOpts.File,
			buildOpts.LockFile,
		}),
		llb.SharedKeyHint(SharedKeyZbuildfiles),
		llb.WithCustomName("load zbuild config files from build context"))

	ymlContent, err := solver.ReadFile(ctx, buildOpts.File, src)
	if err != nil && xerrors.Is(err, statesolver.FileNotFound) {
		return nil, ZbuildfileNotFound
	} else if err != nil {
		return nil, xerrors.Errorf("could not load %s: %w", buildOpts.File, err)
	}

	var def BuildDef
	if err = yaml.Unmarshal(ymlContent, &def); err != nil {
		return nil, xerrors.Errorf("could not decode %s: %w", buildOpts.File, err)
	}

	lockContent, err := solver.ReadFile(ctx, buildOpts.LockFile, src)
	if err != nil && xerrors.Is(err, statesolver.FileNotFound) {
		return &def, nil
	} else if err != nil {
		return nil, xerrors.Errorf("could not load %s: %w", buildOpts.LockFile, err)
	}
	def.RawLocks = lockContent

	return &def, nil
}
