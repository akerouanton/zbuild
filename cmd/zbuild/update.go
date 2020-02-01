package main

import (
	"github.com/NiR-/zbuild/pkg/builddef"
	"github.com/NiR-/zbuild/pkg/builder"
	"github.com/NiR-/zbuild/pkg/pkgsolver"
	"github.com/NiR-/zbuild/pkg/registry"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var updateFlags = struct {
	file     string
	context  string
	logLevel string
}{
	logLevel: "warn",
}

func newUpdateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "update",
		DisableAutoGenTag: true,
		Short:             "Update version locks",
		Run:               HandleUpdateCmd,
	}

	AddFileFlag(cmd, &updateFlags.file)
	AddContextFlag(cmd, &updateFlags.context)
	AddLogLevelFlag(cmd, &updateFlags.logLevel)

	return cmd
}

func HandleUpdateCmd(cmd *cobra.Command, args []string) {
	configureLogger(cmd, updateFlags.logLevel)

	buildctx, err := builddef.NewContext(updateFlags.context, "")
	if err != nil {
		logrus.Fatalf("%+v", err)
	}
	if !buildctx.IsLocalContext() {
		logrus.Fatalf("Only local contexts are supported by zbuild update.")
	}

	buildOpts := builddef.BuildOpts{
		File:         updateFlags.file,
		LockFile:     builddef.LockFilepath(updateFlags.file),
		BuildContext: buildctx,
	}

	solver := newDockerSolver(buildOpts.BuildContext.Source)
	b := builder.Builder{
		Registry:   registry.Registry,
		PkgSolvers: pkgsolver.DefaultPackageSolversMap,
	}

	if err := b.UpdateLockFile(solver, buildOpts); err != nil {
		logrus.Fatalf("%+v", err)
	}
}
