package webserver

import (
	"context"

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
	registry.Register("webserver", &WebserverHandler{})
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

	if def.Healthcheck {
		def.SystemPackages["curl"] = "*"
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

	if def.ConfigFile != "" {
		configFileSrc := llb.Local(buildOpts.ContextName,
			llb.IncludePatterns([]string{def.ConfigFile}),
			llb.LocalUniqueID(buildOpts.LocalUniqueID),
			llb.SessionID(buildOpts.SessionID),
			llb.SharedKeyHint(SharedKeys.ConfigFile),
			llb.WithCustomName("load config file from build context"))

		state = llbutils.Copy(configFileSrc, def.ConfigFile, state, def.Type.ConfigPath(), fileOwner)
	}

	for _, asset := range def.Assets {
		state = llbutils.Copy(*buildOpts.Source, asset.From, state, asset.To, fileOwner)
	}

	return state, img, nil
}

func (h *WebserverHandler) WithSolver(solver statesolver.StateSolver) {
	h.solver = solver
}
