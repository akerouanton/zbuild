package nodejs

import (
	"context"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/NiR-/zbuild/pkg/builddef"
	"github.com/NiR-/zbuild/pkg/image"
	"github.com/NiR-/zbuild/pkg/llbutils"
	"github.com/NiR-/zbuild/pkg/registry"
	"github.com/NiR-/zbuild/pkg/statesolver"
	"github.com/moby/buildkit/client/llb"
	"golang.org/x/xerrors"
)

var SharedKeys = struct {
	BuildContext string
	ConfigFiles  string
	PackageFiles string
}{
	BuildContext: "build-context",
	ConfigFiles:  "config-files",
	PackageFiles: "package-files",
}

func init() {
	RegisterKind(registry.Registry)
}

func RegisterKind(reg *registry.KindRegistry) {
	reg.Register("nodejs", &NodeJSHandler{})
}

type NodeJSHandler struct {
	solver statesolver.StateSolver
}

func (h *NodeJSHandler) WithSolver(solver statesolver.StateSolver) {
	h.solver = solver
}

func (h *NodeJSHandler) Build(
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
	isWebserverBuild := false
	if strings.HasPrefix(stageName, "webserver-") {
		isWebserverBuild = true
		stageName = strings.TrimPrefix(stageName, "webserver-")
	}

	stageDef, err := def.ResolveStageDefinition(stageName, true)
	if err != nil {
		return state, img, xerrors.Errorf("could not resolve stage %q: %w", stageName, err)
	}

	if isWebserverBuild && *stageDef.Dev {
		return state, img, xerrors.Errorf("webserver cannot be built from dev stages")
	}

	state, img, err = h.buildNodeJS(ctx, def, stageDef, buildOpts)
	if err != nil {
		err = xerrors.Errorf("could not build nodejs stage: %w", err)
		return state, img, err
	}

	if !isWebserverBuild {
		return state, img, nil
	}

	state, img, err = h.buildWebserver(ctx, def, state, buildOpts)
	if err != nil {
		err = xerrors.Errorf("could not build webserver stage: %w", err)
		return state, img, err
	}

	return state, img, nil
}

func (h *NodeJSHandler) buildNodeJS(
	ctx context.Context,
	def Definition,
	stageDef StageDefinition,
	buildOpts builddef.BuildOpts,
) (llb.State, *image.Image, error) {
	state := llbutils.ImageSource(def.Locks.BaseImage, true)
	baseImg, err := image.LoadMeta(ctx, def.Locks.BaseImage)
	if err != nil {
		return state, nil, xerrors.Errorf("loading %q metadata: %w", def.Locks.BaseImage, err)
	}

	img := image.CloneMeta(baseImg)
	img.Config.Labels[builddef.ZbuildLabel] = "true"

	state, err = llbutils.InstallSystemPackages(state, llbutils.APT, stageDef.Locks.SystemPackages)
	if err != nil {
		return state, img, xerrors.Errorf("failed to add \"install system pacakges\" steps: %w", err)
	}

	state = llbutils.CopyExternalFiles(state, stageDef.ExternalFiles)
	state = llbutils.Mkdir(state, "1000:1000", "/app")
	state = state.User("1000")
	state = state.Dir("/app")

	state = h.globalPackagesInstall(stageDef, state, buildOpts)

	if *stageDef.Dev == false {
		state = h.yarnInstall(stageDef, state, buildOpts)
		state = h.copySourceDirs(stageDef, state, buildOpts)
		state = h.build(stageDef, state)
	}

	setImageMetadata(stageDef, state, img)

	return state, img, nil
}

func setImageMetadata(stageDef StageDefinition, state llb.State, img *image.Image) {
	for _, dir := range stageDef.StatefulDirs {
		fullpath := dir
		if !path.IsAbs(fullpath) {
			fullpath = path.Join("/app", dir)
		}

		img.Config.Volumes[fullpath] = struct{}{}
	}

	if *stageDef.Healthcheck {
		img.Config.Healthcheck = &image.HealthConfig{
			Test:     []string{"CMD", "http_proxy= test \"$(curl --fail http://127.0.0.1/_ping)\" = \"pong\""},
			Interval: 10 * time.Second,
			Timeout:  1 * time.Second,
			Retries:  3,
		}
	}

	nodeEnv := "development"
	if *stageDef.Dev == false {
		nodeEnv = "production"
	}

	img.Config.User = "1000"
	img.Config.WorkingDir = "/app"
	img.Config.Env = []string{
		"PATH=" + getEnv(state, "PATH"),
		"NODE_ENV=" + nodeEnv,
	}
	now := time.Now()
	img.Created = &now

	if stageDef.Command != nil {
		img.Config.Cmd = *stageDef.Command
	}
}

func (h *NodeJSHandler) buildWebserver(
	ctx context.Context,
	def Definition,
	state llb.State,
	buildOpts builddef.BuildOpts,
) (llb.State, *image.Image, error) {
	webserverHandler, err := registry.FindHandler("webserver")
	if err != nil {
		return state, nil, err
	}
	webserverHandler.WithSolver(h.solver)

	webserverLocks, err := def.Locks.Webserver.RawLocks()
	if err != nil {
		return state, nil, err
	}

	newOpts := buildOpts
	newOpts.Def = &builddef.BuildDef{
		Kind:      "webserver",
		RawConfig: def.Webserver.RawConfig(),
		RawLocks:  webserverLocks,
	}
	newOpts.Source = &state

	return webserverHandler.Build(ctx, newOpts)
}

func getEnv(src llb.State, name string) string {
	val, _ := src.GetEnv(name)
	return val
}

func (h *NodeJSHandler) globalPackagesInstall(
	stageDef StageDefinition,
	state llb.State,
	buildOpts builddef.BuildOpts,
) llb.State {
	if len(stageDef.GlobalPackages) == 0 {
		return state
	}

	pkgs := make([]string, 0, len(stageDef.GlobalPackages))
	for pkg, constraint := range stageDef.GlobalPackages {
		if constraint != "" && constraint != "*" {
			pkg += "@" + constraint
		}
		pkgs = append(pkgs, pkg)
	}

	cmd := fmt.Sprintf("yarn add -g %s", strings.Join(pkgs, " "))
	run := state.Run(
		llbutils.Shell(cmd),
		llb.User("1000"),
		llb.WithCustomNamef("Run %s", cmd))

	return run.Root()
}

// @TODO: add npm support
func (h *NodeJSHandler) yarnInstall(
	stageDef StageDefinition,
	state llb.State,
	buildOpts builddef.BuildOpts,
) llb.State {
	packageSrc := llb.Local(buildOpts.ContextName,
		llb.IncludePatterns([]string{"package.json", "yarn.lock"}),
		llb.LocalUniqueID(buildOpts.LocalUniqueID),
		llb.SessionID(buildOpts.SessionID),
		llb.SharedKeyHint(SharedKeys.PackageFiles),
		llb.WithCustomName("load package.json and yarn.lock from build context"))
	state = llbutils.Copy(packageSrc, "package.json", state, "/app/", "1000:1000")
	state = llbutils.Copy(packageSrc, "yarn.lock", state, "/app/", "1000:1000")

	run := state.Run(
		llbutils.Shell("yarn install --frozen-lockfile"),
		llb.Dir(state.GetDir()),
		llb.User("1000"),
		llb.WithCustomName("Run yarn install"))

	return run.Root()
}

func (h *NodeJSHandler) copySourceDirs(
	stageDef StageDefinition,
	state llb.State,
	buildOpts builddef.BuildOpts,
) llb.State {
	buildContextSrc := llb.Local(buildOpts.ContextName,
		llb.IncludePatterns(includePatterns(stageDef)),
		llb.ExcludePatterns(excludePatterns(stageDef)),
		llb.LocalUniqueID(buildOpts.LocalUniqueID),
		llb.SessionID(buildOpts.SessionID),
		llb.SharedKeyHint(SharedKeys.BuildContext),
		llb.WithCustomName("load build context"))

	return llbutils.Copy(buildContextSrc, "/", state, "/app", "1000:1000")
}

func excludePatterns(stageDef StageDefinition) []string {
	excludes := []string{}
	// Explicitly exclude stateful dirs to ensure they aren't included when
	// they're in one of sourceDirs
	for _, dir := range stageDef.StatefulDirs {
		excludes = append(excludes, dir)
	}
	return excludes
}

func includePatterns(stageDef StageDefinition) []string {
	includes := []string{}
	for _, dir := range stageDef.SourceDirs {
		includes = append(includes, dir)
	}
	return includes
}

func (h *NodeJSHandler) build(
	stageDef StageDefinition,
	state llb.State,
) llb.State {
	if stageDef.BuildCommand == nil {
		return state
	}

	run := state.Run(
		llbutils.Shell(*stageDef.BuildCommand),
		llb.Dir(state.GetDir()),
		llb.AddEnv("NODE_ENV", "production"),
		llb.WithCustomName("Build"))
	return run.Root()
}
