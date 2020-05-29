package php

import (
	"context"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/NiR-/notpecl/pecl"
	"github.com/NiR-/zbuild/pkg/builddef"
	"github.com/NiR-/zbuild/pkg/image"
	"github.com/NiR-/zbuild/pkg/llbutils"
	"github.com/NiR-/zbuild/pkg/registry"
	"github.com/NiR-/zbuild/pkg/statesolver"
	"github.com/moby/buildkit/client/llb"
	"golang.org/x/xerrors"
)

const (
	defaultComposerImageTag = "docker.io/library/composer:1.9.0"

	ConfigDir   = "/usr/local/etc"
	WorkingDir  = "/app"
	ComposerDir = "/composer"
)

var SharedKeys = struct {
	BuildContext  string
	ComposerFiles string
	ConfigFiles   string
}{
	BuildContext:  "build-context",
	ComposerFiles: "composer-files",
	ConfigFiles:   "config-files",
}

func init() {
	RegisterKind(registry.Registry)
}

// RegisterKind adds a LLB DAG builder to the given KindRegistry for php
// definition kind.
func RegisterKind(registry *registry.KindRegistry) {
	registry.Register("php", NewPHPHandler(), true)
}

type PHPHandler struct {
	pecl   pecl.Backend
	solver statesolver.StateSolver
}

func NewPHPHandler() *PHPHandler {
	return &PHPHandler{
		pecl: pecl.New(),
	}
}

func (h *PHPHandler) WithSolver(solver statesolver.StateSolver) {
	h.solver = solver
}

func (h *PHPHandler) WithPeclBackend(pb pecl.Backend) {
	h.pecl = pb
}

func (h *PHPHandler) DebugConfig(
	buildOpts builddef.BuildOpts,
) (interface{}, error) {
	ctx := context.TODO()
	stageDef, err := h.loadDefs(ctx, buildOpts)
	if err != nil {
		return nil, err
	}

	// Remove this value as it would pollute the dump
	stageDef.DefLocks.Stages = map[string]StageLocks{}

	return stageDef, nil
}

func (h *PHPHandler) Build(
	ctx context.Context,
	buildOpts builddef.BuildOpts,
) (llb.State, *image.Image, error) {
	var state llb.State
	var img *image.Image

	stageDef, err := h.loadDefs(ctx, buildOpts)
	if err != nil {
		return state, img, err
	}

	state, img, err = h.buildPHP(ctx, stageDef, buildOpts)
	if err != nil {
		err = xerrors.Errorf("could not build php stage: %w", err)
		return state, img, err
	}

	return state, img, nil
}

func (h *PHPHandler) buildPHP(
	ctx context.Context,
	stageDef StageDefinition,
	buildOpts builddef.BuildOpts,
) (llb.State, *image.Image, error) {
	state := llbutils.ImageSource(stageDef.DefLocks.BaseImage, true)
	baseImg, err := image.LoadMeta(ctx, stageDef.DefLocks.BaseImage)
	if err != nil {
		return state, nil, xerrors.Errorf("failed to load %q metadata: %w", stageDef.DefLocks.BaseImage, err)
	}

	img := image.CloneMeta(baseImg)
	img.Config.Labels[builddef.ZbuildLabel] = "true"

	composer := llbutils.ImageSource(defaultComposerImageTag, false)
	state = llbutils.Copy(
		composer, "/usr/bin/composer", state, "/usr/bin/composer", "", buildOpts.IgnoreLayerCache)

	pkgManager := llbutils.APT
	if stageDef.DefLocks.OSRelease.Name == "alpine" {
		pkgManager = llbutils.APK
	}

	if buildOpts.WithCacheMounts && len(stageDef.StageLocks.SystemPackages) > 0 {
		state = llbutils.SetupSystemPackagesCache(state, pkgManager)
	}

	state, err = llbutils.InstallSystemPackages(state, pkgManager,
		stageDef.StageLocks.SystemPackages,
		llbutils.NewCachingStrategyFromBuildOpts(buildOpts))
	if err != nil {
		return state, img, xerrors.Errorf("failed to add \"install system pacakges\" steps: %w", err)
	}

	state = InstallExtensions(stageDef, state, buildOpts)
	state = llbutils.CopyExternalFiles(state, stageDef.ExternalFiles)

	state = llbutils.Mkdir(state, "1000:1000",
		append([]string{WorkingDir, ComposerDir}, stageDef.StatefulDirs...)...)
	state = state.User("1000")
	state = state.Dir(WorkingDir)
	state = state.AddEnv("COMPOSER_HOME", ComposerDir)

	state, err = copyConfigFiles(stageDef, state, buildOpts)
	if err != nil {
		return state, img, err
	}

	state = globalComposerInstall(stageDef, state, buildOpts)
	if !stageDef.Dev {
		state = composerInstall(stageDef, state, buildOpts)
		state = copySourceFiles(stageDef, state, buildOpts)
		state, err = postInstall(stageDef, state, buildOpts)
		if err != nil {
			return state, img, err
		}
	}

	setImageMetadata(stageDef, state, img)

	return state, img, nil
}

func copyConfigFiles(
	stageDef StageDefinition,
	state llb.State,
	buildOpts builddef.BuildOpts,
) (llb.State, error) {
	if len(stageDef.ConfigFiles) == 0 {
		return state, nil
	}

	srcContext := buildOpts.BuildContext
	srcPrefix := srcContext.Subdir()
	include := stageDef.ConfigFiles.SourcePaths(srcPrefix)
	srcState := llbutils.FromContext(srcContext,
		llb.IncludePatterns(include),
		llb.LocalUniqueID(buildOpts.LocalUniqueID),
		llb.SessionID(buildOpts.SessionID),
		llb.SharedKeyHint(SharedKeys.ConfigFiles),
		llb.WithCustomName("load config files from build context"))

	pathParams := map[string]string{
		"config_dir": ConfigDir,
		"fpm_conf":   path.Join(ConfigDir, "php-fpm.conf"),
		"php_ini":    path.Join(ConfigDir, "php/php.ini"),
	}
	interpolated, err := stageDef.ConfigFiles.Interpolate(
		srcPrefix, WorkingDir, pathParams)
	if err != nil {
		return state, err
	}

	// Despite the IncludePatterns() above, the source state might also
	// contain files that were not including, for instance if the conext is
	// non-local. However, including precise patterns help buildkit determine
	// if the cache is fresh (when using a local context). As such, we can't
	// just copy the whole source state to the dest state.
	state = llbutils.CopyAll(
		srcState, state, interpolated, "1000:1000", buildOpts.IgnoreLayerCache)

	return state, nil
}

func copySourceFiles(
	stageDef StageDefinition,
	state llb.State,
	buildOpts builddef.BuildOpts,
) llb.State {
	sourceContext := resolveSourceContext(stageDef, buildOpts)
	srcState := llbutils.FromContext(sourceContext,
		llb.IncludePatterns(includePatterns(sourceContext, &stageDef)),
		llb.ExcludePatterns(excludePatterns(sourceContext, &stageDef)),
		llb.LocalUniqueID(buildOpts.LocalUniqueID),
		llb.SessionID(buildOpts.SessionID),
		llb.SharedKeyHint(SharedKeys.BuildContext),
		llb.WithCustomName("load build context"))

	if sourceContext.Type == builddef.ContextTypeLocal {
		srcPath := prefixContextPath(sourceContext, "/")
		return llbutils.Copy(
			srcState, srcPath, state, WorkingDir+"/", "1000:1000", buildOpts.IgnoreLayerCache)
	}

	// Despite the IncludePatterns() above, the source state might also
	// contain files that were not including if the conext is non-local.
	// As such, we can't just copy the whole source state to the dest state
	// in such case.
	for _, srcfile := range stageDef.Sources {
		srcPath := prefixContextPath(sourceContext, srcfile)
		destPath := path.Join(WorkingDir, srcfile)
		state = llbutils.Copy(
			srcState, srcPath, state, destPath, "1000:1000", buildOpts.IgnoreLayerCache)
	}

	return state
}

func resolveSourceContext(
	stageDef StageDefinition,
	buildOpts builddef.BuildOpts,
) *builddef.Context {
	if stageDef.DefLocks.SourceContext != nil {
		return stageDef.DefLocks.SourceContext
	}
	return buildOpts.BuildContext
}

func setImageMetadata(
	stage StageDefinition,
	state llb.State,
	img *image.Image,
) {
	for _, dir := range stage.StatefulDirs {
		fullpath := dir
		if !path.IsAbs(fullpath) {
			fullpath = path.Join(WorkingDir, dir)
		}

		img.Config.Volumes[fullpath] = struct{}{}
	}

	if stage.Healthcheck != nil {
		img.Config.Healthcheck = stage.Healthcheck.ToImageConfig()
	}

	img.Config.User = "1000"
	img.Config.WorkingDir = WorkingDir
	img.Config.Env = []string{
		"PATH=/composer/vendor/bin:" + getEnv(state, "PATH"),
		"COMPOSER_HOME=" + ComposerDir,
		"PHP_VERSION=" + getEnv(state, "PHP_VERSION"),
		"PHP_INI_DIR=" + getEnv(state, "PHP_INI_DIR"),
	}
	now := time.Now()
	img.Created = &now

	if stage.Command != nil {
		img.Config.Cmd = *stage.Command
	}
}

func excludePatterns(srcContext *builddef.Context, stageDef *StageDefinition) []string {
	excludes := []string{}
	// Explicitly exclude stateful dirs to ensure they aren't included when
	// they're in one of Sources
	for _, dir := range stageDef.StatefulDirs {
		dirpath := prefixContextPath(srcContext, dir)
		excludes = append(excludes, dirpath)
	}
	return excludes
}

func includePatterns(srcContext *builddef.Context, stageDef *StageDefinition) []string {
	includes := []string{}
	for _, srcpath := range stageDef.Sources {
		fullpath := prefixContextPath(srcContext, srcpath)
		includes = append(includes, fullpath)
	}
	return includes
}

func prefixContextPath(srcContext *builddef.Context, p string) string {
	if srcContext.IsGitContext() && srcContext.Path != "" {
		return path.Join("/", srcContext.Path, p)
	}

	return p
}

func getEnv(src llb.State, name string) string {
	val, _ := src.GetEnv(name)
	return val
}

const composerCacheDir = "/var/cache/composer"

func globalComposerInstall(stageDef StageDefinition, state llb.State, buildOpts builddef.BuildOpts) llb.State {
	deps := make([]string, 0, stageDef.GlobalDeps.Len())
	deps = append(deps, "hirak/prestissimo")

	for dep, constraint := range stageDef.GlobalDeps.Map() {
		if constraint != "" && constraint != "*" {
			dep += ":" + constraint
		}
		deps = append(deps, dep)
	}

	cmds := []string{fmt.Sprintf(
		"composer global require --prefer-dist --classmap-authoritative %s",
		strings.Join(deps, " "))}

	runOpts := []llb.RunOption{
		llb.Dir(state.GetDir()),
		llb.User("1000"),
		llb.AddEnv("COMPOSER_CACHE_DIR", composerCacheDir),
		llb.WithCustomNamef("Run composer global require (%s)", strings.Join(deps, ", "))}

	if buildOpts.IgnoreLayerCache {
		runOpts = append(runOpts, llb.IgnoreCache)
	}

	if buildOpts.WithCacheMounts {
		runOpts = append(runOpts, cacheMountOptForComposer(buildOpts))
	} else {
		cmds = append(cmds, "composer clear-cache")
	}

	runOpts = append(runOpts, llbutils.Shell(cmds...))
	return state.Run(runOpts...).Root()
}

func composerInstall(
	stageDef StageDefinition,
	state llb.State,
	buildOpts builddef.BuildOpts,
) llb.State {
	srcContext := resolveSourceContext(stageDef, buildOpts)
	include := []string{
		prefixContextPath(srcContext, "composer.json"),
		prefixContextPath(srcContext, "composer.lock")}
	srcState := llbutils.FromContext(srcContext,
		llb.IncludePatterns(include),
		llb.LocalUniqueID(buildOpts.LocalUniqueID),
		llb.SessionID(buildOpts.SessionID),
		llb.SharedKeyHint(SharedKeys.ComposerFiles),
		llb.WithCustomName("load composer files from build context"))

	srcPath := prefixContextPath(srcContext, "composer.*")
	state = llbutils.Copy(
		srcState, srcPath, state, WorkingDir+"/", "1000:1000", buildOpts.IgnoreLayerCache)

	cmds := []string{
		"composer install --no-dev --prefer-dist --no-scripts --no-autoloader"}
	runOpts := []llb.RunOption{
		llb.Dir(state.GetDir()),
		llb.User("1000"),
		llb.AddEnv("COMPOSER_CACHE_DIR", composerCacheDir),
		llb.WithCustomName("Run composer install")}

	if buildOpts.IgnoreLayerCache {
		runOpts = append(runOpts, llb.IgnoreCache)
	}

	if buildOpts.WithCacheMounts {
		runOpts = append(runOpts, cacheMountOptForComposer(buildOpts))
	} else {
		cmds = append(cmds, "composer clear-cache")
	}

	runOpts = append(runOpts, llbutils.Shell(cmds...))
	return state.Run(runOpts...).Root()
}

func cacheMountOptForComposer(buildOpts builddef.BuildOpts) llb.RunOption {
	return llbutils.CacheMountOpt(composerCacheDir, buildOpts.CacheIDNamespace, "1000")
}

func postInstall(
	stageDef StageDefinition,
	state llb.State,
	buildOpts builddef.BuildOpts,
) (llb.State, error) {
	dumpFlags, err := stageDef.ComposerDumpFlags.Flags()
	if err != nil {
		return llb.State{}, err
	}

	cmds := append(
		[]string{fmt.Sprintf("composer dump-autoload %s", dumpFlags)},
		stageDef.PostInstall...)
	runOpts := []llb.RunOption{
		llbutils.Shell(cmds...),
		llb.Dir(state.GetDir()),
		llb.WithCustomName("Dump autoloader and execute custom post-install steps")}

	if buildOpts.IgnoreLayerCache {
		runOpts = append(runOpts, llb.IgnoreCache)
	}

	return state.Run(runOpts...).Root(), nil
}
