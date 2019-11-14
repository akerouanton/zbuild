package main

import (
	"github.com/NiR-/webdf/pkg/builder"
	"github.com/NiR-/webdf/pkg/deftypes/php"
	"github.com/NiR-/webdf/pkg/registry"
	"github.com/moby/buildkit/frontend/gateway/grpcclient"
	"github.com/moby/buildkit/util/appcontext"
	"github.com/sirupsen/logrus"
)

func main() {
	reg := registry.NewTypeRegistry()
	php.RegisterDefType(reg)

	b := builder.Builder{Registry: reg}
	err := grpcclient.RunFromEnvironment(appcontext.Context(), b.Build)
	if err != nil {
		logrus.Fatal(err)
	}
}
