package php

import (
	"context"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/NiR-/webdf/pkg/builddef"
	"github.com/NiR-/webdf/pkg/image"
	"github.com/NiR-/webdf/pkg/llbutils"
	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/frontend/gateway/client"
	"golang.org/x/xerrors"
)

const (
	defaultBaseImage        = "docker.io/library/php"
	defaultComposerImageTag = "docker.io/library/composer:1.9.0"

	webdfLabel = "com.webdf"
)

func (h PHPHandler) Build(
	ctx context.Context,
	c client.Client,
	opts builddef.BuildOpts,
) (llb.State, *image.Image, error) {
	def, err := decodeGenericDef(opts.Def)
	if err != nil {
		return llb.State{}, nil, err
	}

	stage, err := def.ResolveStageDefinition(opts.Stage)
	if err != nil {
		return llb.State{}, nil, xerrors.Errorf("could not resolve stage %q: %+v", opts.Stage, err)
	}

	locks, ok := def.Locks.Stages[opts.Stage]
	if !ok {
		locks = StageLocks{}
	}

	addIntegrations(&stage)

	if def.Infer {
		if err := loadPlatformReqsFromContext(ctx, c, &stage, opts); err != nil {
			err := xerrors.Errorf("could not load platform-reqs from composer.lock: %v", err)
			return llb.State{}, nil, err
		}

		inferExtensions(&stage)
		inferSystemPackages(&stage)
	}

	return Config2LLB(ctx, &stage, locks, opts.SessionID)
}

func (h PHPHandler) DebugLLB(opts builddef.BuildOpts) (llb.State, error) {
	def, err := decodeGenericDef(opts.Def)
	if err != nil {
		return llb.State{}, err
	}

	stage, err := def.ResolveStageDefinition(opts.Stage)
	if err != nil {
		return llb.State{}, xerrors.Errorf("could not resolve stage %q: %+v", opts.Stage, err)
	}

	locks, ok := def.Locks.Stages[opts.Stage]
	if !ok {
		locks = StageLocks{}
	}

	addIntegrations(&stage)

	// @TODO: remove this once lock system is fully operational
	if def.Infer {
		if err := loadPlatformReqsFromFS(&stage); err != nil {
			err := xerrors.Errorf("could not load platform-reqs from composer.lock: %v", err)
			return llb.State{}, err
		}

		inferExtensions(&stage)
		inferSystemPackages(&stage)
	}

	ctx := context.TODO()
	llb, _, err := Config2LLB(ctx, &stage, locks, "<SESSION-ID>")
	return llb, err
}

func Config2LLB(
	ctx context.Context,
	stage *StageDefinition,
	locks StageLocks,
	sessionID string,
) (llb.State, *image.Image, error) {
	baseImageRef := defaultBaseImage + ":" + stage.Version
	if *stage.FPM {
		baseImageRef += "-fpm"
	}

	state := llbutils.ImageSource(baseImageRef, true)
	baseImg, err := image.LoadMeta(ctx, baseImageRef)
	if err != nil {
		return llb.State{}, nil, xerrors.Errorf("loading %q metadata: %v", baseImageRef, err)
	}

	img := image.CloneMeta(baseImg)
	img.Config.Labels[webdfLabel] = "true"

	composer := llbutils.ImageSource(defaultComposerImageTag, false)
	state = llbutils.Copy(composer, "/usr/bin/composer", state, "/usr/bin/composer", "")
	state, err = llbutils.InstallSystemPackages(state, llbutils.APT, locks.SystemPackages)
	if err != nil {
		return llb.State{}, nil, xerrors.Errorf("failed to install system pacakges: %v", err)
	}

	state = installExtensions(state, locks.Extensions)
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
			llb.SessionID(sessionID),
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
			llb.SessionID(sessionID),
			llb.SharedKeyHint("composer-files"),
			llb.WithCustomName("load composer files from build context"))
		state = llbutils.Copy(composerSrc, "composer.*", state, "/app/", "1000:1000")
		state = composerInstall(state)

		buildContextSrc := llb.Local("context",
			llb.IncludePatterns(includePatterns(stage)),
			llb.ExcludePatterns(excludePatterns(stage)),
			llb.SessionID(sessionID),
			llb.SharedKeyHint("build-context"),
			llb.WithCustomName("load build context"))
		state = llbutils.Copy(buildContextSrc, "/", state, "/app/", "1000:1000")

		state = postInstall(state, stage)
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

func postInstall(state llb.State, stage *StageDefinition) llb.State {
	cliFlags := []string{"--no-dev", "--optimize"}
	if stage.ComposerDumpFlags.ClassmapAuthoritative {
		cliFlags = append(cliFlags, "--optimize", "--classmap-authoritative")
	} else if stage.ComposerDumpFlags.APCU {
		cliFlags = append(cliFlags, "--optimize", "--apcu")
	}

	cmds := []string{
		fmt.Sprintf("composer dump-autoload %s", strings.Join(cliFlags, " ")),
	}
	cmds = append(cmds, stage.PostInstall...)

	run := state.Run(
		llbutils.Shellf(strings.Join(cmds, "; ")),
		llb.Dir(state.GetDir()),
		llb.WithCustomName("Dump autoloader and execute custom post-install steps"))
	return run.Root()
}

func installExtensions(state llb.State, extensions map[string]string) llb.State {
	coreExtensions := filterExtensions(extensions, isCoreExtension)
	peclExtensions := filterExtensions(extensions, isNotCoreExtension)

	cmds := []string{}
	if len(coreExtensions) > 0 {
		coreExtensionNames := getExtensionNames(coreExtensions)
		cmds = append(cmds,
			fmt.Sprintf("docker-php-ext-install -j\"$(nproc)\" %s", strings.Join(coreExtensionNames, " ")),
		)
	}
	if len(peclExtensions) > 0 {
		// @TODO: should use version constraints
		peclExtensionSpecs := getExtensionNames(peclExtensions)
		peclExtensionNames := getExtensionNames(peclExtensions)
		cmds = append(cmds,
			fmt.Sprintf("pecl install -o -f %s", strings.Join(peclExtensionSpecs, " ")),
			fmt.Sprintf("docker-php-ext-enable %s", strings.Join(peclExtensionNames, " ")),
			"rm -rf /tmp/pear",
		)
	}

	if len(cmds) == 0 {
		return state
	}

	extensionNames := getExtensionNames(extensions)
	exec := state.Run(
		llbutils.Shellf(strings.Join(cmds, " && ")),
		llb.WithCustomNamef("Install PHP extensions (%s)", strings.Join(extensionNames, ", ")))

	return exec.Root()
}
