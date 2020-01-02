package main

import (
	"context"

	"github.com/NiR-/zbuild/pkg/builder"
	_ "github.com/NiR-/zbuild/pkg/defkinds/php"
	_ "github.com/NiR-/zbuild/pkg/defkinds/webserver"
	_ "github.com/NiR-/zbuild/pkg/defkinds/nodejs"
	"github.com/NiR-/zbuild/pkg/registry"
	"github.com/NiR-/zbuild/pkg/statesolver"
	"github.com/moby/buildkit/frontend/gateway/client"
	"github.com/moby/buildkit/frontend/gateway/grpcclient"
	"github.com/moby/buildkit/util/appcontext"
	"github.com/sirupsen/logrus"
)

func main() {
	b := builder.Builder{
		Registry: registry.Registry,
	}
	f := func(ctx context.Context, c client.Client) (*client.Result, error) {
		solver := statesolver.NewBuildkitSolver(c)
		return b.Build(ctx, solver, c)
	}

	err := grpcclient.RunFromEnvironment(appcontext.Context(), f)
	if err != nil {
		logrus.Fatalf("%+v", err)
	}
}
