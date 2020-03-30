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

	include := []string{
		prefixContextPath(sourceContext, "composer.json"),
		prefixContextPath(sourceContext, "composer.lock"),
	}
	composerSrc := solver.FromContext(sourceContext,
		llb.IncludePatterns(include),
		llb.SharedKeyHint(SharedKeys.ComposerFiles),
		llb.WithCustomName("load composer files from build context"))

	lockdata, err := solver.ReadFile(ctx, include[1], composerSrc)
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
		// Empty PHP associative arrays are represented as empty list when
		// converted to JSON. As such, we can't type Platform as a
		// map[string]string here, instead we try to convert it into a map
		// below and ignore it if we can't as it means it's empty.
		Platform interface{}
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

	if p, ok := parsed.Platform.(map[string]interface{}); ok {
		lock.platformReqs = findExtRequirements(lock.platformReqs, normalizePlatform(p))
	}

	return lock, nil
}

func normalizePlatform(p map[string]interface{}) map[string]string {
	r := map[string]string{}

	for k, v := range p {
		r[k] = v.(string)
	}

	return r
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
