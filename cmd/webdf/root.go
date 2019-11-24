package main

import (
	"github.com/NiR-/webdf/pkg/deftypes/php"
	"github.com/NiR-/webdf/pkg/filefetch"
	"github.com/NiR-/webdf/pkg/registry"
	"github.com/docker/docker/client"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	webdfCmd *cobra.Command
)

func main() {
	webdfCmd = &cobra.Command{
		Use:               "webdf",
		DisableAutoGenTag: true,
		Short:             "webdf is a tool made to easily manage Docker-based environments and help developers working on web projects",
	}

	webdfCmd.AddCommand(newUpdateCmd())
	webdfCmd.AddCommand(newDebugLLBCmd())

	if err := webdfCmd.Execute(); err != nil {
		logrus.Fatal(err)
	}
}

func buildTypeRegistry() *registry.TypeRegistry {
	docker, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		logrus.Fatal(err)
	}

	fetcher := filefetch.DockerFetcher{
		Client: docker,
		Labels: map[string]string{},
	}

	reg := registry.NewTypeRegistry()
	php.RegisterDefType(reg, fetcher)

	return reg
}
