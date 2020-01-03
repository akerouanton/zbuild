package main

import (
	"github.com/NiR-/zbuild/pkg/builder"
	"github.com/NiR-/zbuild/pkg/pkgsolver"
	"github.com/NiR-/zbuild/pkg/registry"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var updateFlags = struct {
	file    string
	context string
}{}

func newUpdateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "update",
		DisableAutoGenTag: true,
		Short:             "Update version locks",
		Run:               HandleUpdateCmd,
	}

	AddFileFlag(cmd, &updateFlags.file)
	AddContextFlag(cmd, &updateFlags.context)

	return cmd
}

func HandleUpdateCmd(cmd *cobra.Command, args []string) {
	solver := newDockerSolver(updateFlags.context)
	pkgSolver := pkgsolver.NewAPTSolver(solver)
	b := builder.Builder{
		Registry:  registry.Registry,
		PkgSolver: pkgSolver,
	}

	if err := b.UpdateLockFile(solver, updateFlags.file); err != nil {
		logrus.Fatalf("%+v", err)
	}
}
