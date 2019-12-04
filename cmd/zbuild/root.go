package main

import (
	"github.com/NiR-/zbuild/pkg/defkinds/php"
	"github.com/NiR-/zbuild/pkg/filefetch"
	"github.com/NiR-/zbuild/pkg/registry"
	"github.com/docker/docker/client"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	zbuildCmd *cobra.Command
)

func main() {
	zbuildCmd = &cobra.Command{
		Use:               "zbuild",
		DisableAutoGenTag: true,
		Short:             "zbuild is a tool made to easily manage Docker-based environments and help developers working on web projects",
	}

	zbuildCmd.AddCommand(newUpdateCmd())
	zbuildCmd.AddCommand(newDebugLLBCmd())

	if err := zbuildCmd.Execute(); err != nil {
		logrus.Fatal(err)
	}
}

func buildKindRegistry() *registry.KindRegistry {
	docker, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		logrus.Fatal(err)
	}

	fetcher := filefetch.DockerFetcher{
		Client: docker,
		Labels: map[string]string{},
	}

	reg := registry.NewKindRegistry()
	php.RegisterKind(reg, fetcher)

	return reg
}
