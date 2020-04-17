package webserver

import (
	"context"
	"path"
	"time"

	"github.com/NiR-/zbuild/pkg/builddef"
	"github.com/NiR-/zbuild/pkg/image"
	"github.com/NiR-/zbuild/pkg/llbutils"
	"github.com/NiR-/zbuild/pkg/registry"
	"github.com/NiR-/zbuild/pkg/statesolver"
	"github.com/moby/buildkit/client/llb"
	"golang.org/x/xerrors"
)

var fileOwner = "nginx"
var SharedKeys = struct {
	ConfigFiles string
}{
	ConfigFiles: "config-files",
}

type WebserverHandler struct {
	solver statesolver.StateSolver
}

func init() {
	RegisterKind(registry.Registry)
}

func RegisterKind(registry *registry.KindRegistry) {
	registry.Register("webserver", &WebserverHandler{}, false)
}

func (h *WebserverHandler) DebugConfig(
	buildOpts builddef.BuildOpts,
) (interface{}, error) {
	return NewKind(buildOpts.Def)
}

func (h *WebserverHandler) Build(
	ctx context.Context,
	buildOpts builddef.BuildOpts,
) (llb.State, *image.Image, error) {
	var state llb.State
	var img *image.Image

	def, err := NewKind(buildOpts.Def)
	if err != nil {
		return state, img, err
	}

	state = llbutils.ImageSource(def.Locks.BaseImage, true)
	baseImg, err := image.LoadMeta(ctx, def.Locks.BaseImage)
	if err != nil {
		return state, img, xerrors.Errorf("failed to load %q metadata: %w", def.Locks.BaseImage, err)
	}

	img = image.CloneMeta(baseImg)
	img.Config.Labels[builddef.ZbuildLabel] = "true"

	if buildOpts.SourceState == nil && len(def.Assets) > 0 {
		return state, img, xerrors.New("no source state to copy assets from has been provided")
	}

	pkgManager := llbutils.APT
	if def.Locks.OSRelease.Name == "alpine" {
		pkgManager = llbutils.APK
	}

	if buildOpts.WithCacheMounts && len(def.Locks.SystemPackages) > 0 {
		state = llbutils.SetupSystemPackagesCache(state, pkgManager)
	}

	state, err = llbutils.InstallSystemPackages(state, pkgManager,
		def.Locks.SystemPackages,
		llbutils.NewCachingStrategyFromBuildOpts(buildOpts))
	if err != nil {
		return state, img, xerrors.Errorf("failed to add \"install system pacakges\" steps: %w", err)
	}

	workingDir := img.Config.WorkingDir
	state, err = h.copyConfigFiles(def, state, workingDir, buildOpts)
	if err != nil {
		return state, img, err
	}

	for _, asset := range def.Assets {
		state = llbutils.Copy(
			*buildOpts.SourceState, asset.From, state, asset.To, fileOwner, buildOpts.IgnoreLayerCache)
	}

	setImageMetadata(def, state, img)

	return state, img, nil
}

func (h *WebserverHandler) copyConfigFiles(
	def Definition,
	state llb.State,
	workingDir string,
	buildOpts builddef.BuildOpts,
) (llb.State, error) {
	if len(def.ConfigFiles) == 0 {
		return state, nil
	}

	srcContext := buildOpts.BuildContext
	srcPrefix := srcContext.Subdir()
	include := def.ConfigFiles.SourcePaths(srcPrefix)
	srcState := llbutils.FromContext(srcContext,
		llb.IncludePatterns(include),
		llb.LocalUniqueID(buildOpts.LocalUniqueID),
		llb.SessionID(buildOpts.SessionID),
		llb.SharedKeyHint(SharedKeys.ConfigFiles),
		llb.WithCustomName("load config files from build context"))

	pathParams := map[string]string{
		"config_dir": def.Type.ConfigDir(),
	}
	interpolated, err := def.ConfigFiles.Interpolate(
		srcPrefix, workingDir, pathParams)
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

func prefixContextPath(srcContext *builddef.Context, p string) string {
	if srcContext.IsGitContext() && srcContext.Path != "" {
		return path.Join("/", srcContext.Path, p)
	}

	return p
}

func setImageMetadata(
	def Definition,
	state llb.State,
	img *image.Image,
) {
	if def.Healthcheck.IsEnabled() {
		img.Config.Healthcheck = def.Healthcheck.ToImageConfig()
	}

	// Use SIGSTOP to gracefully stop nginx
	img.Config.StopSignal = "SIGSTOP"
	now := time.Now()
	img.Created = &now
}

func (h *WebserverHandler) WithSolver(solver statesolver.StateSolver) {
	h.solver = solver
}
