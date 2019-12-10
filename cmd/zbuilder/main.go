package main

import (
	"context"

	"github.com/NiR-/zbuild/pkg/builder"
	"github.com/NiR-/zbuild/pkg/defkinds/php"
	"github.com/NiR-/zbuild/pkg/registry"
	"github.com/NiR-/zbuild/pkg/statesolver"
	"github.com/moby/buildkit/frontend/gateway/client"
	"github.com/moby/buildkit/frontend/gateway/grpcclient"
	"github.com/moby/buildkit/util/appcontext"
	"github.com/sirupsen/logrus"
)

func main() {
	// @TODO: use a default kind registry
	reg := registry.NewKindRegistry()
	php.RegisterKind(reg)

	b := builder.Builder{Registry: reg}
	f := func(ctx context.Context, c client.Client) (*client.Result, error) {
		solver := statesolver.NewBuildkitSolver(c)
		return b.Build(ctx, solver, c)
	}

	err := grpcclient.RunFromEnvironment(appcontext.Context(), f)
	if err != nil {
		logrus.Fatal(err)
	}
}
