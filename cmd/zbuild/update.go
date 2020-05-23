package main

import (
	"github.com/NiR-/zbuild/pkg/builddef"
	"github.com/NiR-/zbuild/pkg/builder"
	"github.com/NiR-/zbuild/pkg/pkgsolver"
	"github.com/NiR-/zbuild/pkg/registry"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/twpayne/go-vfs"
)

var updateFlags = struct {
	file                  string
	context               string
	logLevel              string
	noImageUpdate         bool
	noPackagesUpdate      bool
	noPHPExtensionsUpdate bool
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

	cmd.Flags().BoolVar(&updateFlags.noImageUpdate, "no-image-update", false, "Do not update the base image reference")
	cmd.Flags().BoolVar(&updateFlags.noPackagesUpdate, "no-pacakges-update", false, "Do not update system packages")
	cmd.Flags().BoolVar(&updateFlags.noPHPExtensionsUpdate, "no-php-extensions-update", false, "Do not update PHP extensions")

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

	updateOpts := builddef.UpdateLocksOpts{
		BuildOpts: &builddef.BuildOpts{
			File:         updateFlags.file,
			LockFile:     builddef.LockFilepath(updateFlags.file),
			BuildContext: buildctx,
		},
		UpdateImageRef:       !updateFlags.noImageUpdate,
		UpdateSystemPackages: !updateFlags.noPackagesUpdate,
		UpdatePHPExtensions:  !updateFlags.noPHPExtensionsUpdate,
	}

	solver := newLocalSolver(buildctx.Source)
	b := builder.Builder{
		Registry:   registry.Registry,
		PkgSolvers: pkgsolver.DefaultPackageSolversMap,
		Filesystem: vfs.HostOSFS,
	}

	if err := b.UpdateLockFile(solver, updateOpts); err != nil {
		logrus.Fatalf("%+v", err)
	}
}
