package main

import (
	"context"
	"fmt"
	"os"

	_ "github.com/NiR-/zbuild/pkg/defkinds/nodejs"
	_ "github.com/NiR-/zbuild/pkg/defkinds/php"
	_ "github.com/NiR-/zbuild/pkg/defkinds/webserver"
	"github.com/NiR-/zbuild/pkg/statesolver"
	"github.com/containerd/containerd/remotes/docker"
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
	zbuildCmd.AddCommand(newLLBGraphCmd())
	zbuildCmd.AddCommand(newDebugConfigCmd())

	if err := zbuildCmd.Execute(); err != nil {
		logrus.Fatalf("%+v", err)
	}
}

func newDockerSolver(rootDir string) statesolver.DockerSolver {
	c, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		logrus.Fatalf("%+v", err)
	}

	c.NegotiateAPIVersion(context.TODO())

	if rootDir == "" {
		var err error
		rootDir, err = os.Getwd()
		if err != nil {
			logrus.Fatalf("%+v", err)
		}
	}

	return statesolver.DockerSolver{
		Client:        c,
		Labels:        map[string]string{},
		RootDir:       rootDir,
		ImageResolver: docker.NewResolver(docker.ResolverOptions{}),
	}
}

func AddFileFlag(cmd *cobra.Command, val *string) {
	cmd.Flags().StringVarP(val, "file", "f", "zbuild.yml", "Path to the zbuild.yml file to debug")
}

func AddContextFlag(cmd *cobra.Command, val *string) {
	cmd.Flags().StringVarP(val, "context", "c", "", "Root dir of the build context")
}

func AddStageFlag(cmd *cobra.Command, val *string) {
	cmd.Flags().StringVarP(val, "stage", "s", "dev", "Name of the stage to use")
}

func AddLogLevelFlag(cmd *cobra.Command, val *string) {
	cmd.Flags().StringVar(val, "log-level", *val, "Log level (one of: error, warn, info, debug)")
}

func configureLogger(cmd *cobra.Command, level string) {
	parsed, err := logrus.ParseLevel(level)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Invalid log level %q.", level)
		cmd.Usage() //nolint:errcheck
		os.Exit(1)
	}

	logrus.SetLevel(parsed)
}
