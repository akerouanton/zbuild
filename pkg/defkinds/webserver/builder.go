package webserver

import (
	"context"
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
	ConfigFile string
}{
	ConfigFile: "config-file",
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

	state, err = llbutils.InstallSystemPackages(state, llbutils.APT, def.Locks.SystemPackages)
	if err != nil {
		return state, img, xerrors.Errorf("failed to add \"install system pacakges\" steps: %w", err)
	}

	if def.ConfigFile != nil && *def.ConfigFile != "" {
		state = h.copyConfigFile(def, state, buildOpts)
	}

	for _, asset := range def.Assets {
		state = llbutils.Copy(*buildOpts.SourceState, asset.From, state, asset.To, fileOwner)
	}

	setImageMetadata(def, state, img)

	return state, img, nil
}

func (h *WebserverHandler) copyConfigFile(
	def Definition,
	state llb.State,
	buildOpts builddef.BuildOpts,
) llb.State {
	configFileSrc := llbutils.FromContext(buildOpts.BuildContext,
		llb.IncludePatterns([]string{*def.ConfigFile}),
		llb.LocalUniqueID(buildOpts.LocalUniqueID),
		llb.SessionID(buildOpts.SessionID),
		llb.SharedKeyHint(SharedKeys.ConfigFile),
		llb.WithCustomName("load config file from build context"))

	return llbutils.Copy(
		configFileSrc,
		*def.ConfigFile,
		state,
		def.Type.ConfigPath(),
		fileOwner,
	)
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
