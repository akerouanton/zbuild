package builder

import (
	"context"
	"encoding/json"

	"github.com/NiR-/webdf/pkg/config"
	"github.com/NiR-/webdf/pkg/llbutils"
	"github.com/NiR-/webdf/pkg/service"
	"github.com/moby/buildkit/exporter/containerimage/exptypes"
	"github.com/moby/buildkit/frontend/gateway/client"
	"golang.org/x/xerrors"
)

var (
	// NoServiceOption is an error returned by the Builder when the service
	// name was either not provided as a build option or is empty.
	NoServiceOption = xerrors.New("no \"service\" option provided, webdf doesn't know which service to build")
)

type Builder struct {
	Registry *service.TypeRegistry
}

func (b Builder) Build(ctx context.Context, c client.Client) (*client.Result, error) {
	cfg, err := config.LoadFromContext(ctx, c)
	if err != nil {
		return nil, err
	}

	opts := c.BuildOpts().Opts
	svcName, ok := opts["service"]
	if !ok || svcName == "" {
		return nil, NoServiceOption
	}

	svc, err := cfg.FindService(svcName)
	if err != nil {
		return nil, err
	}

	typeHandler, err := b.Registry.FindTypeHandler(svc.Type)
	if err != nil {
		return nil, err
	}

	buildOpts := service.BuildOpts{
		Service:   svc,
		SessionID: c.BuildOpts().SessionID,
	}
	state, img, err := typeHandler.Build(ctx, c, buildOpts)
	if err != nil {
		return nil, err
	}

	res, ref, err := llbutils.SolveState(ctx, c, state)
	if err != nil {
		return nil, err
	}

	config, err := json.Marshal(img)
	if err != nil {
		return nil, xerrors.Errorf("failed to marshal image config: %v", err)
	}

	res.AddMeta(exptypes.ExporterImageConfigKey, config)
	res.SetRef(ref)

	return res, nil
}
