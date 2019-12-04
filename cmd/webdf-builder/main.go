package main

import (
	"github.com/NiR-/webdf/pkg/builder"
	"github.com/NiR-/webdf/pkg/defkinds/php"
	"github.com/NiR-/webdf/pkg/filefetch"
	"github.com/NiR-/webdf/pkg/registry"
	"github.com/docker/docker/client"
	"github.com/moby/buildkit/frontend/gateway/grpcclient"
	"github.com/moby/buildkit/util/appcontext"
	"github.com/sirupsen/logrus"
)

func main() {
	docker, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		logrus.Fatal(err)
	}

	// @TODO: not needed
	fetcher := filefetch.DockerFetcher{
		Client: docker,
		Labels: map[string]string{},
	}

	reg := registry.NewKindRegistry()
	php.RegisterKind(reg, fetcher)

	b := builder.Builder{Registry: reg}
	err = grpcclient.RunFromEnvironment(appcontext.Context(), b.Build)
	if err != nil {
		logrus.Fatal(err)
	}
}
