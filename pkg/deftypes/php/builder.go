package php

import (
	"context"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/NiR-/webdf/pkg/builddef"
	"github.com/NiR-/webdf/pkg/filefetch"
	"github.com/NiR-/webdf/pkg/image"
	"github.com/NiR-/webdf/pkg/llbutils"
	"github.com/NiR-/webdf/pkg/registry"
	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/frontend/gateway/client"
	"golang.org/x/xerrors"
)

const (
	defaultBaseImage        = "docker.io/library/php"
	defaultComposerImageTag = "docker.io/library/composer:1.9.0"

	webdfLabel = "io.webdf"
)

// RegisterDefType adds a LLB DAG builder to the given TypeRegistry for php
// definition type.
func RegisterDefType(registry *registry.TypeRegistry, fetcher filefetch.FileFetcher) {
	registry.Register("php", NewPHPHandler(fetcher))
}

type PHPHandler struct {
	fetcher filefetch.FileFetcher
}

func NewPHPHandler(fetcher filefetch.FileFetcher) PHPHandler {
	return PHPHandler{fetcher}
}

func (h PHPHandler) Build(
	ctx context.Context,
	c client.Client,
	buildOpts builddef.BuildOpts,
) (llb.State, *image.Image, error) {
	opts := toLLBOpts{
		buildOpts: buildOpts,
		platformReqsLoader: func(stage *StageDefinition) error {
			return LoadPlatformReqsFromContext(ctx, c, stage, buildOpts)
		},
	}

	return Config2LLB(ctx, opts)
}

func (h PHPHandler) DebugLLB(buildOpts builddef.BuildOpts) (llb.State, error) {
	buildOpts.SessionID = "<SESSION-ID>"
	opts := toLLBOpts{
		buildOpts: buildOpts,
		platformReqsLoader: func(stage *StageDefinition) error {
			basedir := path.Dir(buildOpts.File)
			return LoadPlatformReqsFromFS(stage, basedir)
		},
	}

	ctx := context.TODO()
	llb, _, err := Config2LLB(ctx, opts)
	return llb, err
}

type toLLBOpts struct {
	buildOpts          builddef.BuildOpts
	platformReqsLoader func(*StageDefinition) error
}

func Config2LLB(
	ctx context.Context,
	opts toLLBOpts,
) (llb.State, *image.Image, error) {
	var state llb.State
	var img *image.Image

	def, err := NewSpecializedDefinition(opts.buildOpts.Def)
	if err != nil {
		return state, img, err
	}

	stageName := opts.buildOpts.Stage
	stage, err := def.ResolveStageDefinition(stageName, opts.platformReqsLoader)
	if err != nil {
		return state, img, xerrors.Errorf("could not resolve stage %q: %v", opts.buildOpts.Stage, err)
	}

	locks, ok := def.Locks.Stages[opts.buildOpts.Stage]
	if !ok {
		return state, img, xerrors.Errorf(
			"could not build stage %q: no locks available. Please update your lockfile",
			opts.buildOpts.Stage,
		)
	}

	state = llbutils.ImageSource(def.Locks.BaseImage, true)
	baseImg, err := image.LoadMeta(ctx, def.Locks.BaseImage)
	if err != nil {
		return state, img, xerrors.Errorf("loading %q metadata: %v", def.Locks.BaseImage, err)
	}

	img = image.CloneMeta(baseImg)
	img.Config.Labels[webdfLabel] = "true"

	composer := llbutils.ImageSource(defaultComposerImageTag, false)
	state = llbutils.Copy(composer, "/usr/bin/composer", state, "/usr/bin/composer", "")
	state, err = llbutils.InstallSystemPackages(state, llbutils.APT, locks.SystemPackages)
	if err != nil {
		return state, img, xerrors.Errorf("failed to install system pacakges: %v", err)
	}

	state = InstallExtensions(state, locks.Extensions)
	state = llbutils.CopyExternalFiles(state, stage.ExternalFiles)

	state = llbutils.Mkdir(state, "1000:1000", "/app", "/composer")
	state = state.User("1000")
	state = state.Dir("/app")
	state = state.AddEnv("COMPOSER_HOME", "/composer")

	// @TODO: copy files from git context instead of local source
	if *stage.Dev {
		configFilesSrc := llb.Local("context",
			llb.IncludePatterns([]string{
				*stage.ConfigFiles.IniFile,
				*stage.ConfigFiles.FPMConfigFile,
			}),
			llb.LocalUniqueID(opts.buildOpts.LocalUniqueID),
			llb.SessionID(opts.buildOpts.SessionID),
			llb.SharedKeyHint("config-files"),
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

		composerSrc := llb.Local("context",
			llb.IncludePatterns([]string{"composer.json", "composer.lock"}),
			llb.LocalUniqueID(opts.buildOpts.LocalUniqueID),
			llb.SessionID(opts.buildOpts.SessionID),
			llb.SharedKeyHint("composer-files"),
			llb.WithCustomName("load composer files from build context"))
		state = llbutils.Copy(composerSrc, "composer.*", state, "/app/", "1000:1000")
		state = composerInstall(state)

		buildContextSrc := llb.Local("context",
			llb.IncludePatterns(includePatterns(&stage)),
			llb.ExcludePatterns(excludePatterns(&stage)),
			llb.LocalUniqueID(opts.buildOpts.LocalUniqueID),
			llb.SessionID(opts.buildOpts.SessionID),
			llb.SharedKeyHint("build-context"),
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
			// @TODO: FPM port can actually be changed by FPM config file,
			// find a better way to set this up.
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