package main

import (
	"os"

	"github.com/NiR-/zbuild/pkg/builder"
	"github.com/moby/buildkit/client/llb"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var debugFlags = struct {
	file    string
	stage   string
	context string
}{}

// @TODO: buildctl should not be required
const debugDescription = `Output LLB DAG in binary format.

This command alone is not really useful. To have a readable output, you have to
pipe its output to ` + "`buildctl debug dump-llb`" + `:

	zbuild debug-llb | buildctl debug dump-llb | jq -C . | less -R
`

func newDebugLLBCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "debug-llb",
		Hidden: true,
		Short:  "Output LLB DAG in binary format.",
		Long:   debugDescription,
		Run:    HandleDebugLLBCmd,
	}

	cmd.Flags().StringVarP(&debugFlags.stage, "target", "t", "dev", "Name of the stage to debug")
	AddFileFlag(cmd, &debugFlags.file)
	AddContextFlag(cmd, &debugFlags.context)

	return cmd
}

func HandleDebugLLBCmd(cmd *cobra.Command, args []string) {
	solver := newDockerSolver(debugFlags.context)
	reg := buildKindRegistry()
	b := builder.Builder{Registry: reg}

	state, err := b.Debug(solver, debugFlags.file, debugFlags.stage)
	if err != nil {
		logrus.Fatal(err)
	}

	out, err := state.Marshal(llb.LinuxAmd64)
	if err != nil {
		logrus.Fatal(err)
	}

	llb.WriteTo(out, os.Stdout)
}

