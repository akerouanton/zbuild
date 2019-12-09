package php

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/NiR-/zbuild/pkg/statesolver"
	"github.com/moby/buildkit/client/llb"
	"golang.org/x/xerrors"
)

type composerLock struct {
	packages     map[string]string
	platformReqs map[string]string
}

// LoadComposerLockFromContext loads composer.lock file from build context and
// adds packages and packages-dev (if stageDef is dev) to stageDef.LockedPackages.
// Also, it adds extensions listed in platform-reqs key to stageDef.PlatformReqs.
// It returns nil if the composer.lock file couldn't be found.
func LoadComposerLock(
	ctx context.Context,
	solver statesolver.StateSolver,
	stageDef *StageDefinition,
) error {
	composerSrc := solver.FromBuildContext(
		llb.IncludePatterns([]string{"composer.json", "composer.lock"}),
		llb.SharedKeyHint(SharedKeys.ComposerFiles),
		llb.WithCustomName("load composer files from build context"))

	lockdata, err := solver.ReadFile(ctx, "composer.lock", composerSrc)
	if xerrors.Is(err, statesolver.FileNotFound) {
		return nil
	} else if err != nil {
		return xerrors.Errorf("could not load composer.lock: %v", err)
	}

	parsed, err := parseComposerLock(lockdata, *stageDef.Dev)
	if err != nil {
		return err
	}

	stageDef.LockedPackages = parsed.packages
	stageDef.PlatformReqs = parsed.platformReqs

	return nil
}

func parseComposerLock(lockdata []byte, isDev bool) (composerLock, error) {
	var parsed map[string]interface{}
	lock := composerLock{
		packages:     map[string]string{},
		platformReqs: map[string]string{},
	}

	if err := json.Unmarshal(lockdata, &parsed); err != nil {
		return lock, xerrors.Errorf("could not unmarshal composer.lock: %w", err)
	}

	packages, ok := parsed["packages"]
	if !ok {
		return lock, xerrors.New("composer.lock has no packages key")
	}

	for _, rawPkg := range packages.([]interface{}) {
		pkg := rawPkg.(map[string]interface{})
		pkgName := pkg["name"].(string)
		pkgVersion := pkg["version"].(string)

		lock.packages[pkgName] = pkgVersion
	}

	devPackages, ok := parsed["packages-dev"]
	if isDev && ok {
		for _, rawPkg := range devPackages.([]interface{}) {
			pkg := rawPkg.(map[string]interface{})
			pkgName := pkg["name"].(string)
			pkgVersion := pkg["version"].(string)

			lock.packages[pkgName] = pkgVersion
		}
	}

	platformReqs, ok := parsed["platform"]
	if !ok {
		return lock, nil
	}

	for req, constraint := range platformReqs.(map[string]interface{}) {
		if !strings.HasPrefix(req, "ext-") {
			continue
		}

		ext := strings.TrimPrefix(req, "ext-")
		lock.platformReqs[ext] = constraint.(string)
	}

	return lock, nil
}
