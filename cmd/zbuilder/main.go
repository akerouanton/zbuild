package main

import (
	"github.com/NiR-/zbuild/pkg/builder"
	"github.com/NiR-/zbuild/pkg/defkinds/php"
	"github.com/NiR-/zbuild/pkg/registry"
	"github.com/moby/buildkit/frontend/gateway/grpcclient"
	"github.com/moby/buildkit/util/appcontext"
	"github.com/sirupsen/logrus"
)

func main() {
	// @TODO: use a default kind registry
	reg := registry.NewKindRegistry()
	php.RegisterKind(reg)

	b := builder.Builder{Registry: reg}
	err := grpcclient.RunFromEnvironment(appcontext.Context(), b.Build)
	if err != nil {
		logrus.Fatal(err)
	}
}
