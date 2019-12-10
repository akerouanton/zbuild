package main

import (
	"github.com/NiR-/zbuild/pkg/defkinds/php"
	"github.com/NiR-/zbuild/pkg/registry"
	"github.com/NiR-/zbuild/pkg/statesolver"
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

// @TODO: use a default kind registry
func buildKindRegistry() *registry.KindRegistry {
	reg := registry.NewKindRegistry()
	php.RegisterKind(reg)

	return reg
}

func newDockerSolver() statesolver.DockerSolver {
	docker, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		logrus.Fatal(err)
	}

	return statesolver.DockerSolver{
		Client: docker,
		Labels: map[string]string{},
	}
}
