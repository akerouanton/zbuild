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
	reg.Register("nodejs", &NodeJSHandler{}, true)
}

type NodeJSHandler struct {
	solver statesolver.StateSolver
}

func (h *NodeJSHandler) WithSolver(solver statesolver.StateSolver) {
	h.solver = solver
}

func (h *NodeJSHandler) DebugConfig(
	buildOpts builddef.BuildOpts,
) (interface{}, error) {
	stageDef, err := h.loadDefs(buildOpts)
	if err != nil {
		return nil, err
	}

	// This property would pollute the dumped config
	stageDef.DefLocks.Stages = map[string]StageLocks{}

	return stageDef, nil
}

func (h *NodeJSHandler) Build(
	ctx context.Context,
	buildOpts builddef.BuildOpts,
) (llb.State, *image.Image, error) {
	var state llb.State
	var img *image.Image

	stageDef, err := h.loadDefs(buildOpts)
	if err != nil {
		return state, img, err
	}

	state, img, err = h.buildNodeJS(ctx, stageDef, buildOpts)
	if err != nil {
		err = xerrors.Errorf("could not build nodejs stage: %w", err)
		return state, img, err
	}

	return state, img, nil
}

func (h *NodeJSHandler) buildNodeJS(
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

	state, err = llbutils.InstallSystemPackages(state, llbutils.APT, stageDef.StageLocks.SystemPackages)
	if err != nil {
		return state, img, xerrors.Errorf("failed to add \"install system pacakges\" steps: %w", err)
	}

	state = llbutils.CopyExternalFiles(state, stageDef.ExternalFiles)
	state = llbutils.Mkdir(state, "1000:1000", "/app")
	state = state.User("1000")
	state = state.Dir("/app")

	state = h.globalPackagesInstall(state, stageDef.GlobalPackages.Map(), buildOpts)

	if *stageDef.Dev == false {
		state = h.yarnInstall(stageDef, state, buildOpts)
		state = h.copySources(stageDef, state, buildOpts)
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

	if stageDef.Healthcheck != nil {
		img.Config.Healthcheck = stageDef.Healthcheck.ToImageConfig()
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

func getEnv(src llb.State, name string) string {
	val, _ := src.GetEnv(name)
	return val
}

func (h *NodeJSHandler) globalPackagesInstall(
	state llb.State,
	globalPackages map[string]string,
	buildOpts builddef.BuildOpts,
) llb.State {
	if len(globalPackages) == 0 {
		return state
	}

	pkgs := make([]string, 0, len(globalPackages))
	for pkg, constraint := range globalPackages {
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
	sourceContext := resolveSourceContext(stageDef, buildOpts)
	packageSrc := llbutils.FromContext(sourceContext,
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

func (h *NodeJSHandler) copySources(
	stageDef StageDefinition,
	state llb.State,
	buildOpts builddef.BuildOpts,
) llb.State {
	sourceContext := resolveSourceContext(stageDef, buildOpts)
	src := llbutils.FromContext(sourceContext,
		llb.IncludePatterns(includePatterns(stageDef)),
		llb.ExcludePatterns(excludePatterns(stageDef)),
		llb.LocalUniqueID(buildOpts.LocalUniqueID),
		llb.SessionID(buildOpts.SessionID),
		llb.SharedKeyHint(SharedKeys.BuildContext),
		llb.WithCustomName("load build context"))

	return llbutils.Copy(src, "/", state, "/app", "1000:1000")
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

func excludePatterns(stageDef StageDefinition) []string {
	excludes := []string{}
	// Explicitly exclude stateful dirs to ensure they aren't included when
	// they're in one of Sources
	for _, dir := range stageDef.StatefulDirs {
		excludes = append(excludes, dir)
	}
	return excludes
}

func includePatterns(stageDef StageDefinition) []string {
	includes := []string{}
	for _, dir := range stageDef.Sources {
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
