package main

import (
	"os"
	"path"

	"github.com/NiR-/zbuild/pkg/builder"
	"github.com/NiR-/zbuild/pkg/pkgsolver"
	"github.com/NiR-/zbuild/pkg/registry"
	"github.com/sirupsen/logrus"
	dpkg "github.com/snyh/go-dpkg-parser"
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
	pkgSolver, err := initPackageSolver()
	if err != nil {
		logrus.Fatal(err)
	}

	b := builder.Builder{
		Registry:  registry.DefaultRegistry,
		PkgSolver: pkgSolver,
	}
	solver := newDockerSolver(updateFlags.context)

	if err := b.UpdateLockFile(solver, updateFlags.file); err != nil {
		logrus.Fatal(err)
	}
}

func initPackageSolver() (pkgsolver.PackageSolver, error) {
	var pkgSolver pkgsolver.PackageSolver

	basepath := os.Getenv("XDG_DATA_HOME")
	if basepath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return pkgSolver, err
		}
		basepath = path.Join(home, ".local/share")
	}

	path := path.Join(basepath, "zbuild/dpkg")
	dpkgRepo := dpkg.NewRepository(path)
	pkgSolver = pkgsolver.NewDpkgSolver(dpkgRepo)

	return pkgSolver, nil
}
