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

type ComposerLock struct {
	PlatformReqs    *builddef.VersionMap
	PlatformReqsDev *builddef.VersionMap
}

func (h *PHPHandler) composerLockCacheLoader(
	ctx context.Context,
	buildContext *builddef.Context,
) func(stageDef *StageDefinition) error {
	var cache ComposerLock
	loaded := false

	return func(stageDef *StageDefinition) error {
		if !loaded {
			sourceContext := stageDef.DefLocks.SourceContext
			if sourceContext == nil {
				sourceContext = buildContext
			}

			var err error
			cache, err = LoadComposerLock(ctx, h.solver, sourceContext)
			if err != nil {
				return err
			}
			loaded = true
		}

		stageDef.PlatformReqs = cache.PlatformReqs.Copy()
		if stageDef.Dev {
			stageDef.PlatformReqs.Merge(cache.PlatformReqsDev)
		}

		return nil
	}
}

// loadComposerLock loads composer.lock file from build context and adds
// extensions listed in platform-reqs key and in locked packages requirements
// to stageDef.PlatformReqs. It returns nil if the composer.lock file couldn't
// be found.
func LoadComposerLock(
	ctx context.Context,
	solver statesolver.StateSolver,
	sourceContext *builddef.Context,
) (ComposerLock, error) {
	composerLockPath := prefixContextPath(sourceContext, "composer.lock")
	composerSrc := solver.FromContext(sourceContext,
		llb.IncludePatterns([]string{composerLockPath}),
		llb.SharedKeyHint(SharedKeys.ComposerFiles),
		llb.WithCustomName("load composer.lock from build context"))

	lockdata, err := solver.ReadFile(ctx, composerLockPath, composerSrc)
	if xerrors.Is(err, statesolver.FileNotFound) {
		return ComposerLock{}, nil
	} else if err != nil {
		return ComposerLock{}, xerrors.Errorf("could not load composer.lock: %v", err)
	}

	return parseComposerLock(lockdata)
}

type composerPkg struct {
	Name    string
	Version string
	Require map[string]string
}

func parseComposerLock(lockdata []byte) (ComposerLock, error) {
	parsed := struct {
		Packages    []composerPkg `json:"packages"`
		PackagesDev []composerPkg `json:"packages-dev"`
		// Empty PHP associative arrays are represented as empty list when
		// converted to JSON. As such, we can't type Platform as a
		// map[string]string here, instead we try to convert it into a map
		// below and ignore it if we can't as it means it's empty.
		Platform    interface{} `json:"platform"`
		PlatformDev interface{} `json:"platform-dev"`
	}{}

	if err := json.Unmarshal(lockdata, &parsed); err != nil {
		return ComposerLock{}, xerrors.Errorf(
			"could not unmarshal composer.lock: %w", err)
	}

	lock := ComposerLock{
		PlatformReqs:    &builddef.VersionMap{},
		PlatformReqsDev: &builddef.VersionMap{},
	}
	for _, pkg := range parsed.Packages {
		addExtRequirements(lock.PlatformReqs, pkg.Require)
	}
	for _, pkg := range parsed.PackagesDev {
		addExtRequirements(lock.PlatformReqsDev, pkg.Require)
	}

	if p, ok := parsed.Platform.(map[string]interface{}); ok {
		addExtRequirements(lock.PlatformReqs, normalizePlatform(p))
	}
	if p, ok := parsed.PlatformDev.(map[string]interface{}); ok {
		addExtRequirements(lock.PlatformReqsDev, normalizePlatform(p))
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

func addExtRequirements(locks *builddef.VersionMap, reqs map[string]string) {
	for name, val := range reqs {
		if strings.HasPrefix(name, "ext-") {
			ext := strings.TrimPrefix(name, "ext-")
			locks.Add(ext, val)
		}
	}
}
