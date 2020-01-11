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

var fileOwner = "1000:1000"
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

	if buildOpts.Source == nil && len(def.Assets) > 0 {
		return state, img, xerrors.New("no source state to copy assets from has been provided")
	}

	state, err = llbutils.InstallSystemPackages(state, llbutils.APT, def.Locks.SystemPackages)

	if def.ConfigFile != nil && *def.ConfigFile != "" {
		state = h.copyConfigFile(state, def, buildOpts)
	}

	for _, asset := range def.Assets {
		state = llbutils.Copy(*buildOpts.Source, asset.From, state, asset.To, fileOwner)
	}

	setImageMetadata(def, state, img)

	return state, img, nil
}

func (h *WebserverHandler) copyConfigFile(
	state llb.State,
	def Definition,
	buildOpts builddef.BuildOpts,
) llb.State {
	configFileSrc := llbutils.BuildContext(buildOpts.ContextName,
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
		img.Config.Healthcheck = &image.HealthConfig{
			Test:     []string{"CMD", "http_proxy= test \"$(curl --fail http://127.0.0.1/_status)\" = \"pong\""},
			Interval: 10 * time.Second,
			Timeout:  1 * time.Second,
			Retries:  3,
		}
	}

	// Use SIGSTOP to gracefully stop nginx
	img.Config.StopSignal = "SIGSTOP"
	img.Config.User = "1000"
	now := time.Now()
	img.Created = &now
}

func (h *WebserverHandler) WithSolver(solver statesolver.StateSolver) {
	h.solver = solver
}
