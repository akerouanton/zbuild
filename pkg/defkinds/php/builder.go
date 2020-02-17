package php

import (
	"context"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/NiR-/notpecl/backends"
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
	NotPecl backends.NotPeclBackend
	solver  statesolver.StateSolver
}

func NewPHPHandler() *PHPHandler {
	return &PHPHandler{
		NotPecl: backends.NewNotPeclBackend(),
	}
}

func (h *PHPHandler) WithSolver(solver statesolver.StateSolver) {
	h.solver = solver
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
	stage StageDefinition,
	buildOpts builddef.BuildOpts,
) (llb.State, *image.Image, error) {
	state := llbutils.ImageSource(stage.DefLocks.BaseImage, true)
	baseImg, err := image.LoadMeta(ctx, stage.DefLocks.BaseImage)
	if err != nil {
		return state, nil, xerrors.Errorf("failed to load %q metadata: %w", stage.DefLocks.BaseImage, err)
	}

	img := image.CloneMeta(baseImg)
	img.Config.Labels[builddef.ZbuildLabel] = "true"

	composer := llbutils.ImageSource(defaultComposerImageTag, false)
	state = llbutils.Copy(composer, "/usr/bin/composer", state, "/usr/bin/composer", "")

	pkgManager := llbutils.APT
	if stage.DefLocks.OSRelease.Name == "alpine" {
		pkgManager = llbutils.APK
	}

	state, err = llbutils.InstallSystemPackages(state, pkgManager,
		stage.StageLocks.SystemPackages)
	if err != nil {
		return state, img, xerrors.Errorf("failed to add \"install system pacakges\" steps: %w", err)
	}

	state = InstallExtensions(state, stage)
	state = llbutils.CopyExternalFiles(state, stage.ExternalFiles)

	state = llbutils.Mkdir(state, "1000:1000", "/app", "/composer")
	state = state.User("1000")
	state = state.Dir("/app")
	state = state.AddEnv("COMPOSER_HOME", "/composer")

	state = copyConfigFiles(stage, state, buildOpts)
	state = globalComposerInstall(state, stage.GlobalDeps.Map())

	if !stage.Dev {
		state = composerInstall(stage, state, buildOpts)
		state = copySourceFiles(stage, state, buildOpts)
		state, err = postInstall(state, &stage)
		if err != nil {
			return state, img, err
		}
	}

	setImageMetadata(stage, state, img)

	return state, img, nil
}

func copyConfigFiles(
	stage StageDefinition,
	state llb.State,
	buildOpts builddef.BuildOpts,
) llb.State {
	buildctx := buildOpts.BuildContext
	configFiles := make([]string, 0, 2)

	iniFile := ""
	if stage.ConfigFiles.IniFile != nil {
		iniFile = prefixContextPath(buildctx, *stage.ConfigFiles.IniFile)
		configFiles = append(configFiles, iniFile)
	}

	fpmFile := ""
	if stage.ConfigFiles.FPMConfigFile != nil {
		fpmFile = prefixContextPath(buildctx, *stage.ConfigFiles.FPMConfigFile)
		configFiles = append(configFiles, fpmFile)
	}

	sourceState := llbutils.FromContext(buildctx,
		llb.IncludePatterns(configFiles),
		llb.LocalUniqueID(buildOpts.LocalUniqueID),
		llb.SessionID(buildOpts.SessionID),
		llb.SharedKeyHint(SharedKeys.ConfigFiles),
		llb.WithCustomName("load config files from build context"))

	if iniFile != "" {
		state = llbutils.Copy(
			sourceState, iniFile,
			state, "/usr/local/etc/php/php.ini", "1000:1000")
	}
	if fpmFile != "" {
		state = llbutils.Copy(
			sourceState, fpmFile,
			state, "/usr/local/etc/php-fpm.conf", "1000:1000")
	}

	return state
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
		return llbutils.Copy(srcState, srcPath, state, "/app/", "1000:1000")
	}

	// Despite the IncludePatterns() above, the source state might also
	// contain files that were not including if the conext is non-local.
	// As such, we can't just copy the whole source state to the dest state
	// in such case.
	for _, srcfile := range stageDef.Sources {
		srcPath := prefixContextPath(sourceContext, srcfile)
		destPath := path.Join("/app", srcfile)
		state = llbutils.Copy(srcState, srcPath, state, destPath, "1000:1000")
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
			fullpath = path.Join("/app", dir)
		}

		img.Config.Volumes[fullpath] = struct{}{}
	}

	if stage.Healthcheck != nil {
		img.Config.Healthcheck = stage.Healthcheck.ToImageConfig()
	}

	img.Config.User = "1000"
	img.Config.WorkingDir = "/app"
	img.Config.Env = []string{
		"PATH=" + getEnv(state, "PATH"),
		"COMPOSER_HOME=/composer",
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

func globalComposerInstall(state llb.State, globalDeps map[string]string) llb.State {
	deps := make([]string, 0, len(globalDeps))
	deps = append(deps, "hirak/prestissimo")

	for dep, constraint := range globalDeps {
		if constraint != "" && constraint != "*" {
			dep += ":" + constraint
		}
		deps = append(deps, dep)
	}

	cmds := make([]string, 2)
	cmds[0] = fmt.Sprintf("composer global require --prefer-dist --classmap-authoritative %s",
		strings.Join(deps, " "))
	cmds[1] = "composer clear-cache"

	run := state.Run(
		llbutils.Shell(cmds...),
		llb.Dir(state.GetDir()),
		llb.User("1000"),
		llb.WithCustomNamef("Run composer global require (%s)", strings.Join(deps, ", ")))

	return run.Root()
}

func composerInstall(
	stageDef StageDefinition,
	state llb.State,
	buildOpts builddef.BuildOpts,
) llb.State {
	srcContext := resolveSourceContext(stageDef, buildOpts)
	// @TODO: test if composer.* can be used as an include pattern
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
	state = llbutils.Copy(srcState, srcPath, state, "/app/", "1000:1000")

	cmds := []string{
		"composer install --no-dev --prefer-dist --no-scripts --no-autoloader",
		"composer clear-cache",
	}
	run := state.Run(
		llbutils.Shell(cmds...),
		llb.Dir(state.GetDir()),
		llb.User("1000"),
		llb.WithCustomName("Run composer install"),
	)

	return run.Root()
}

func postInstall(state llb.State, stage *StageDefinition) (llb.State, error) {
	dumpFlags, err := stage.ComposerDumpFlags.Flags()
	if err != nil {
		return llb.State{}, err
	}

	cmds := []string{
		fmt.Sprintf("composer dump-autoload %s", dumpFlags),
	}
	cmds = append(cmds, stage.PostInstall...)

	run := state.Run(
		llbutils.Shell(cmds...),
		llb.Dir(state.GetDir()),
		llb.WithCustomName("Dump autoloader and execute custom post-install steps"))
	return run.Root(), nil
}
