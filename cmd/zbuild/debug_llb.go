package main

import (
	"os"

	"github.com/NiR-/zbuild/pkg/builder"
	"github.com/NiR-/zbuild/pkg/llbutils"
	"github.com/NiR-/zbuild/pkg/registry"
	"github.com/moby/buildkit/client/llb"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var debugFlags = struct {
	file    string
	stage   string
	context string
	asJSON  bool
}{}

const debugDescription = `Output LLB DAG in binary or JSON format.

This command alone is not really useful. To have a readable output, you have to
either pipe its output to ` + "`buildctl debug dump-llb`" + `:

	$ zbuild debug-llb | buildctl debug dump-llb | jq -C . | less -R

Or you can generate JSON dumps like the ones used by most zbuild testcases
using:

	$ zbuild debug-llb --json

You can also pipe this last command into ` + "`zbuild llbgraph`" + `:

	$ zbuild debug-llb --json | zbuild llbgraph
`

func newDebugLLBCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "debug-llb",
		Hidden: true,
		Short:  "Output LLB DAG in binary or JSON format.",
		Long:   debugDescription,
		Run:    HandleDebugLLBCmd,
	}

	AddFileFlag(cmd, &debugFlags.file)
	AddContextFlag(cmd, &debugFlags.context)
	AddStageFlag(cmd, &debugFlags.stage)

	cmd.Flags().BoolVar(&debugFlags.asJSON, "json", false, "Output the LLB DAG in JSON format")

	return cmd
}

func HandleDebugLLBCmd(cmd *cobra.Command, args []string) {
	b := builder.Builder{
		Registry: registry.Registry,
	}
	solver := newDockerSolver(debugFlags.context)

	state, err := b.Debug(solver, debugFlags.file, debugFlags.stage)
	if err != nil {
		logrus.Fatalf("%+v", err)
	}

	if debugFlags.asJSON {
		out, err := llbutils.StateToJSON(state)
		if err != nil {
			logrus.Fatalf("%+v", err)
		}

		if _, err := os.Stdout.Write(out); err != nil {
			logrus.Fatalf("%+v", err)
		}

		return
	}

	out, err := state.Marshal(llb.LinuxAmd64)
	if err != nil {
		logrus.Fatalf("%+v", err)
	}
	llb.WriteTo(out, os.Stdout) //nolint:errcheck
}
