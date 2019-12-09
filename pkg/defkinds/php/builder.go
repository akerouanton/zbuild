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

	zbuildLabel = "io.zbuild"
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
	RegisterKind(registry.DefaultRegistry)
}

// RegisterKind adds a LLB DAG builder to the given KindRegistry for php
// definition kind.
func RegisterKind(registry *registry.KindRegistry) {
	registry.Register("php", NewPHPHandler())
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

func (h *PHPHandler) Build(
	ctx context.Context,
	buildOpts builddef.BuildOpts,
) (llb.State, *image.Image, error) {
	var state llb.State
	var img *image.Image

	def, err := NewKind(buildOpts.Def)
	if err != nil {
		return state, img, err
	}

	stageName := buildOpts.Stage
	composerLockLoader := func(stageDef *StageDefinition) error {
		return LoadComposerLock(ctx, h.solver, stageDef)
	}
	stage, err := def.ResolveStageDefinition(stageName, composerLockLoader)
	if err != nil {
		return state, img, xerrors.Errorf("could not resolve stage %q: %w", buildOpts.Stage, err)
	}

	locks, ok := def.Locks.Stages[buildOpts.Stage]
	if !ok {
		return state, img, xerrors.Errorf(
			"could not build stage %q: no locks available. Please update your lockfile",
			buildOpts.Stage,
		)
	}

	state = llbutils.ImageSource(def.Locks.BaseImage, true)
	baseImg, err := image.LoadMeta(ctx, def.Locks.BaseImage)
	if err != nil {
		return state, img, xerrors.Errorf("loading %q metadata: %w", def.Locks.BaseImage, err)
	}

	img = image.CloneMeta(baseImg)
	img.Config.Labels[zbuildLabel] = "true"

	composer := llbutils.ImageSource(defaultComposerImageTag, false)
	state = llbutils.Copy(composer, "/usr/bin/composer", state, "/usr/bin/composer", "")
	state, err = llbutils.InstallSystemPackages(state, llbutils.APT, locks.SystemPackages)
	if err != nil {
		return state, img, xerrors.Errorf("failed to add \"install system pacakges\" steps: %w", err)
	}

	state = InstallExtensions(state, def, locks.Extensions)
	state = llbutils.CopyExternalFiles(state, stage.ExternalFiles)

	state = llbutils.Mkdir(state, "1000:1000", "/app", "/composer")
	state = state.User("1000")
	state = state.Dir("/app")
	state = state.AddEnv("COMPOSER_HOME", "/composer")

	// @TODO: copy files from git context instead of local source
	if *stage.Dev {
		configFilesSrc := llb.Local(buildOpts.ContextName,
			llb.IncludePatterns([]string{
				*stage.ConfigFiles.IniFile,
				*stage.ConfigFiles.FPMConfigFile,
			}),
			llb.LocalUniqueID(buildOpts.LocalUniqueID),
			llb.SessionID(buildOpts.SessionID),
			llb.SharedKeyHint(SharedKeys.ConfigFiles),
			llb.WithCustomName("load config files from build context"))
		state = llbutils.Copy(
			configFilesSrc,
			*stage.ConfigFiles.IniFile,
			state,
			"/usr/local/etc/php/php.ini",
			"1000:1000")
		state = llbutils.Copy(
			configFilesSrc,
			*stage.ConfigFiles.FPMConfigFile,
			state,
			"/usr/local/etc/php-fpm.conf",
			"1000:1000")

		composerSrc := llb.Local(buildOpts.ContextName,
			llb.IncludePatterns([]string{"composer.json", "composer.lock"}),
			llb.LocalUniqueID(buildOpts.LocalUniqueID),
			llb.SessionID(buildOpts.SessionID),
			llb.SharedKeyHint(SharedKeys.ComposerFiles),
			llb.WithCustomName("load composer files from build context"))
		state = llbutils.Copy(composerSrc, "composer.*", state, "/app/", "1000:1000")
		state = composerInstall(state)

		buildContextSrc := llb.Local(buildOpts.ContextName,
			llb.IncludePatterns(includePatterns(&stage)),
			llb.ExcludePatterns(excludePatterns(&stage)),
			llb.LocalUniqueID(buildOpts.LocalUniqueID),
			llb.SessionID(buildOpts.SessionID),
			llb.SharedKeyHint(SharedKeys.BuildContext),
			llb.WithCustomName("load build context"))
		state = llbutils.Copy(buildContextSrc, "/", state, "/app/", "1000:1000")

		state, err = postInstall(state, &stage)
		if err != nil {
			return state, img, err
		}
	}

	for _, dir := range stage.StatefulDirs {
		fullpath := dir
		if !path.IsAbs(fullpath) {
			fullpath = path.Join("/app", dir)
		}

		img.Config.Volumes[fullpath] = struct{}{}
	}

	if *stage.Healthcheck {
		img.Config.Healthcheck = &image.HealthConfig{
			Test:     []string{"CMD", "http_proxy= test \"$(fcgi-client get 127.0.0.1:9000 /_ping)\" = \"pong\""},
			Interval: 10 * time.Second,
			Timeout:  1 * time.Second,
			Retries:  3,
		}
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

	return state, img, nil
}

func excludePatterns(stage *StageDefinition) []string {
	excludes := []string{}
	// Explicitly exclude stateful dirs to ensure they aren't included when
	// they're in one of sourceDirs
	for _, dir := range stage.StatefulDirs {
		excludes = append(excludes, dir)
	}
	return excludes
}

func includePatterns(stage *StageDefinition) []string {
	includes := []string{}
	for _, dir := range stage.SourceDirs {
		includes = append(includes, dir)
	}
	for _, script := range stage.ExtraScripts {
		includes = append(includes, script)
	}
	return includes
}

func getEnv(src llb.State, name string) string {
	val, _ := src.GetEnv(name)
	return val
}

func composerInstall(state llb.State) llb.State {
	cmds := []string{
		"composer global require --prefer-dist hirak/prestissimo",
		"composer install --no-dev --prefer-dist --no-scripts",
		"composer clear-cache",
	}
	run := state.Run(
		llbutils.Shellf(strings.Join(cmds, " && ")),
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
		llbutils.Shellf(strings.Join(cmds, "; ")),
		llb.Dir(state.GetDir()),
		llb.WithCustomName("Dump autoloader and execute custom post-install steps"))
	return run.Root(), nil
}
