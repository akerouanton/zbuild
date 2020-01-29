package php

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/NiR-/zbuild/pkg/builddef"
	"github.com/NiR-/zbuild/pkg/statesolver"
	"github.com/moby/buildkit/client/llb"
	"golang.org/x/xerrors"
)

type composerLock struct {
	packages     map[string]string
	platformReqs map[string]string
}

// LoadComposerLock loads composer.lock file from build context and adds
// packages and packages-dev (if stageDef is dev) to stageDef.LockedPackages.
// Also it adds extensions listed in platform-reqs key to stageDef.PlatformReqs.
// It returns nil if the composer.lock file couldn't be found.
// @TODO: move to PHPHandler
func LoadComposerLock(
	ctx context.Context,
	solver statesolver.StateSolver,
	stageDef *StageDefinition,
	buildContext *builddef.Context,
) error {
	sourceContext := stageDef.DefLocks.SourceContext
	if sourceContext == nil {
		sourceContext = buildContext
	}

	composerSrc := solver.FromContext(sourceContext,
		llb.IncludePatterns([]string{"composer.json", "composer.lock"}),
		llb.SharedKeyHint(SharedKeys.ComposerFiles),
		llb.WithCustomName("load composer files from build context"))

	lockdata, err := solver.ReadFile(ctx, "composer.lock", composerSrc)
	if xerrors.Is(err, statesolver.FileNotFound) {
		return nil
	} else if err != nil {
		return xerrors.Errorf("could not load composer.lock: %v", err)
	}

	parsed, err := parseComposerLock(lockdata, stageDef.Dev)
	if err != nil {
		return err
	}

	stageDef.LockedPackages = parsed.packages
	stageDef.PlatformReqs = parsed.platformReqs

	return nil
}

func parseComposerLock(lockdata []byte, isDev bool) (composerLock, error) {
	parsed := struct {
		Packages    []composerPkg
		PackagesDev []composerPkg `json:"packages-dev"`
		Platform    map[string]string
	}{}

	if err := json.Unmarshal(lockdata, &parsed); err != nil {
		return composerLock{}, xerrors.Errorf(
			"could not unmarshal composer.lock: %w", err)
	}

	lock := composerLock{
		packages:     map[string]string{},
		platformReqs: map[string]string{},
	}
	for _, pkg := range parsed.Packages {
		lock.packages[pkg.Name] = pkg.Version
		lock.platformReqs = findExtRequirements(lock.platformReqs, pkg.Require)
	}

	if isDev {
		for _, pkg := range parsed.PackagesDev {
			lock.packages[pkg.Name] = pkg.Version
			lock.platformReqs = findExtRequirements(lock.platformReqs, pkg.Require)
		}
	}

	lock.platformReqs = findExtRequirements(lock.platformReqs, parsed.Platform)
	return lock, nil
}

type composerPkg struct {
	Name    string
	Version string
	Require map[string]string
}

func findExtRequirements(locks map[string]string, reqs map[string]string) map[string]string {
	for name, val := range reqs {
		if strings.HasPrefix(name, "ext-") {
			ext := strings.TrimPrefix(name, "ext-")
			locks[ext] = val
		}
	}
	return locks
}
