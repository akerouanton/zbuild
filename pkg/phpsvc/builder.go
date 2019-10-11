package phpsvc

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path"
	"strings"
	"time"

	"github.com/NiR-/webdf/pkg/image"
	"github.com/NiR-/webdf/pkg/llbutils"
	"github.com/NiR-/webdf/pkg/service"
	"github.com/mitchellh/mapstructure"
	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/frontend/gateway/client"
	"golang.org/x/xerrors"
)

const (
	baseImageName = "docker.io/library/php"
	webdfLabel    = "com.webdf"

	defaultComposerImageTag = "docker.io/library/composer:1.9.0"
)

func (h PHPHandler) Build(
	ctx context.Context,
	c client.Client,
	opts service.BuildOpts,
) (llb.State, *image.Image, error) {
	svcCfg := defaultCfg
	if err := mapstructure.Decode(opts.Service.RawConfig, &svcCfg); err != nil {
		err := xerrors.Errorf("could not decode config for service %q: %v", opts.Service.Name, err)
		return llb.State{}, nil, err
	}

	svcCfg.Locks = ServiceLocks{}
	if err := mapstructure.Decode(opts.Service.RawLocks, &svcCfg.Locks); err != nil {
		err := xerrors.Errorf("could not decode version locks for service %q: %v", opts.Service.Name, err)
		return llb.State{}, nil, err
	}

	if err := loadPlatformReqsFromContext(ctx, c, &svcCfg, opts); err != nil {
		err := xerrors.Errorf("could not load platform-reqs from composer.lock: %v", err)
		return llb.State{}, nil, err
	}

	addIntegrations(&svcCfg)
	inferExtensions(&svcCfg, opts.Stage)
	inferSystemPackages(&svcCfg)

	return Config2LLB(ctx, &svcCfg, opts)
}

// loadPlatformReqsFromContext loads composer.lock from build conext and adds
// any extensions declared there but not in webdf.yaml.
func loadPlatformReqsFromContext(
	ctx context.Context,
	c client.Client,
	svcCfg *ServiceConfig,
	opts service.BuildOpts,
) error {
	composerSrc := llb.Local("context",
		llb.IncludePatterns([]string{"composer.json", "composer.lock"}),
		llb.SessionID(opts.SessionID),
		llb.SharedKeyHint("composer-files"),
		llb.WithCustomName("load composer files from build context"))
	_, ref, err := llbutils.SolveState(ctx, c, composerSrc)
	if err != nil {
		return err
	}

	lockdata, ok, err := llbutils.ReadFile(ctx, ref, "composer.lock")
	if !ok {
		return xerrors.New("no composer.lock found")
	}
	if err != nil {
		return xerrors.Errorf("could not load composer.lock: %v", err)
	}

	parsed, err := parsePlatformReqs(lockdata)
	if err != nil {
		return err
	}

	for ext, constraint := range parsed {
		if _, ok := svcCfg.Extensions[ext]; !ok {
			svcCfg.Extensions[ext] = constraint
		}
	}

	return nil
}

func loadPlatformReqsFromFS(svcCfg *ServiceConfig) error {
	lockdata, err := ioutil.ReadFile("composer.lock")
	if err != nil {
		return xerrors.Errorf("could not load composer.lock: %v", err)
	}

	parsed, err := parsePlatformReqs(lockdata)
	if err != nil {
		return err
	}

	for ext, constraint := range parsed {
		if _, ok := svcCfg.Extensions[ext]; !ok {
			svcCfg.Extensions[ext] = constraint
		}
	}

	return nil
}

func parsePlatformReqs(lockdata []byte) (map[string]string, error) {
	var composerLock map[string]interface{}
	if err := json.Unmarshal(lockdata, &composerLock); err != nil {
		return map[string]string{}, xerrors.Errorf("could not unmarshal composer.lock: %v", err)
	}

	platformReqs, ok := composerLock["platform"]
	if !ok {
		return map[string]string{}, nil
	}

	exts := make(map[string]string, 0)
	for req, constraint := range platformReqs.(map[string]interface{}) {
		if !strings.HasPrefix(req, "ext-") {
			continue
		}

		ext := strings.TrimPrefix(req, "ext-")
		exts[ext] = constraint.(string)
	}

	return exts, nil
}

func inferExtensions(svcCfg *ServiceConfig, stage string) {
	// soap extension needs sockets extension to work properly
	if _, ok := svcCfg.Extensions["soap"]; ok {
		if _, ok := svcCfg.Extensions["sockets"]; !ok {
			svcCfg.Extensions["sockets"] = "*"
		}
	}

	// Add zip extension if it's missing as it's used by composer to install packages.
	if _, ok := svcCfg.Extensions["zip"]; !ok {
		svcCfg.Extensions["zip"] = "*"
	}

	// Remove json extension as this extension is already enabled in the base
	// image and it requires extra build tools
	if _, ok := svcCfg.Extensions["json"]; ok {
		delete(svcCfg.Extensions, "json")
	}

	if stage != "dev" {
		if _, ok := svcCfg.Extensions["apcu"]; !ok {
			svcCfg.Extensions["apcu"] = "*"
		}
		if _, ok := svcCfg.Extensions["opcache"]; !ok {
			svcCfg.Extensions["opcache"] = "*"
		}
	}
}

func inferSystemPackages(svcCfg *ServiceConfig) {
	systemPackages := map[string]string{
		"libpcre3-dev": "*",
	}

	for ext := range svcCfg.Extensions {
		switch ext {
		case "intl":
			systemPackages["libicu-dev"] = "*"
		case "soap":
			systemPackages["libxml2-dev"] = "*"
		case "sockets":
			systemPackages["libssl-dev"] = "*"
			systemPackages["openssl"] = "*"
		case "zip":
			systemPackages["zlib1g-dev"] = "*"
		}
	}

	// Add unzip and git packages as they're used by Composer
	if _, ok := svcCfg.SystemPackages["unzip"]; !ok {
		systemPackages["unzip"] = "*"
	}
	if _, ok := svcCfg.SystemPackages["git"]; !ok {
		systemPackages["git"] = "*"
	}

	for name, constraint := range systemPackages {
		if _, ok := svcCfg.SystemPackages[name]; !ok {
			svcCfg.SystemPackages[name] = constraint
		}
	}
}

func Config2LLB(
	ctx context.Context,
	cfg *ServiceConfig,
	opts service.BuildOpts,
) (llb.State, *image.Image, error) {
	baseImageRef := baseImageName + ":" + cfg.Version
	if cfg.FPM {
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
	state, err = llbutils.InstallSystemPackages(state, llbutils.APT, cfg.SystemPackages, cfg.Locks.SystemPackages)
	if err != nil {
		return llb.State{}, nil, xerrors.Errorf("failed to install system pacakges: %v", err)
	}

	state = installExtensions(state, cfg.Extensions)
	state = llbutils.CopyExternalFiles(state, cfg.ExternalFiles)

	state = llbutils.Mkdir(state, "1000:1000", "/app", "/composer")
	state = state.User("1000")
	state = state.Dir("/app")
	state = state.AddEnv("COMPOSER_HOME", "/composer")

	// @TODO: copy files from git context instead of local source
	// @TODO: manage stages
	if opts.Stage != "dev" {
		configFilesSrc := llb.Local("context",
			llb.IncludePatterns([]string{cfg.IniFile, cfg.FPMConfigFile}),
			llb.SessionID(opts.SessionID),
			llb.SharedKeyHint("config-files"),
			llb.WithCustomName("load config files from build context"))
		// @TODO: handle case where ini file and/or fpm config file are empty
		state = llbutils.Copy(configFilesSrc, cfg.IniFile, state, "/usr/local/etc/php/php.ini", "1000:1000")
		state = llbutils.Copy(configFilesSrc, cfg.FPMConfigFile, state, "/usr/local/etc/php-fpm.conf", "1000:1000")

		composerSrc := llb.Local("context",
			llb.IncludePatterns([]string{"composer.json", "composer.lock"}),
			llb.SessionID(opts.SessionID),
			llb.SharedKeyHint("composer-files"),
			llb.WithCustomName("load composer files from build context"))
		state = llbutils.Copy(composerSrc, "composer.*", state, "/app/", "1000:1000")
		state = composerInstall(state)

		buildContextSrc := llb.Local("context",
			llb.IncludePatterns(includePatterns(cfg)),
			llb.ExcludePatterns(excludePatterns(cfg)),
			llb.SessionID(opts.SessionID),
			llb.SharedKeyHint("build-context"),
			llb.WithCustomName("load build context"))
		state = llbutils.Copy(buildContextSrc, "/", state, "/app/", "1000:1000")

		state = cacheWarmup(state, cfg.ComposerDumpFlags)
	}

	for _, dir := range cfg.StatefulDirs {
		fullpath := dir
		if !path.IsAbs(fullpath) {
			fullpath = path.Join("/app", dir)
		}

		img.Config.Volumes[fullpath] = struct{}{}
	}

	if cfg.Healthcheck {
		img.Config.Healthcheck = &image.HealthConfig{
			// @TODO: FPM port can actually be changed by FPM config file,
			// find a better way to set this up.
			Test:     []string{"CMD", "test \"$(fcgi-client get 127.0.0.1:9000 /_ping)\" = \"pong\""},
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

func excludePatterns(cfg *ServiceConfig) []string {
	excludes := []string{}
	// Explicitly exclude stateful dirs to ensure they aren't included when
	// they're in one of sourceDirs
	for _, dir := range cfg.StatefulDirs {
		excludes = append(excludes, dir)
	}
	return excludes
}

func includePatterns(cfg *ServiceConfig) []string {
	includes := []string{}
	for _, dir := range cfg.SourceDirs {
		includes = append(includes, dir)
	}
	for _, script := range cfg.ExtraScripts {
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

// @TODO: this is specific to Symfony. We have to find a better way to handle
// this case.
func cacheWarmup(state llb.State, dumpFlags ComposerDumpFlags) llb.State {
	// @TODO: return an error if both dump optimizations are enabled at the same time
	// @TODO: move this logic to a new ComposerDumpFlags method
	cliFlags := []string{"--no-dev", "--optimize"}
	if dumpFlags.ClassmapAuthoritative {
		cliFlags = append(cliFlags, "--optimize", "--classmap-authoritative")
	} else if dumpFlags.APCU {
		cliFlags = append(cliFlags, "--optimize", "--apcu")
	}

	cmds := []string{
		fmt.Sprintf("composer dump-autoload %s", strings.Join(cliFlags, " ")),
		"php -d display_errors=on bin/console cache:warmup --env=prod",
	}

	run := state.Run(
		llbutils.Shellf(strings.Join(cmds, " && ")),
		llb.Dir(state.GetDir()),
		llb.WithCustomName("Warm up Symfony Cache"))
	return run.Root()
}

func installExtensions(state llb.State, extensions map[string]string) llb.State {
	coreExtensions := filterExtensions(extensions, isCoreExtension)
	peclExtensions := filterExtensions(extensions, isNotCoreExtension)

	cmds := []string{}
	if len(coreExtensions) > 0 {
		// @TODO: should use version constraints
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
