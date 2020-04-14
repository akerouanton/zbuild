package main

import (
	"fmt"
	"os"

	"github.com/NiR-/zbuild/pkg/builder"
	"github.com/NiR-/zbuild/pkg/registry"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var debugConfigFlags = struct {
	file    string
	stage   string
	context string
}{}

const debugConfigDescription = `Show the final config used to build a stage.

This command can be used to dump into JSON the final stage config after all
merge and inference operations happened.`

func newDebugConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "debug-config",
		Short: "Show the final config used to build a stage.",
		Long:  debugConfigDescription,
		Run:   HandleDebugConfigCmd,
	}

	AddFileFlag(cmd, &debugConfigFlags.file)
	AddStageFlag(cmd, &debugConfigFlags.stage)
	AddContextFlag(cmd, &debugConfigFlags.context)

	return cmd
}

func HandleDebugConfigCmd(cmd *cobra.Command, args []string) {
	b := builder.Builder{
		Registry: registry.Registry,
	}
	solver := newLocalSolver(debugConfigFlags.context)

	dump, err := b.DumpConfig(solver,
		debugConfigFlags.file,
		debugConfigFlags.stage)
	if err != nil {
		logrus.Fatalf("%+v", err)
	}

	fmt.Fprint(os.Stdout, string(dump))
}
