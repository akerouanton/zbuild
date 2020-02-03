package statesolver

import (
	"context"

	"github.com/NiR-/zbuild/pkg/builddef"
	"golang.org/x/xerrors"
)

// ResolveImageOS reads /etc/os-release from a given image and parse it.
func ResolveImageOS(
	ctx context.Context,
	solver StateSolver,
	imageRef string,
) (builddef.OSRelease, error) {
	var res builddef.OSRelease
	raw, err := solver.ReadFile(ctx, "/etc/os-release",
		solver.FromImage(imageRef))
	if xerrors.Is(err, FileNotFound) {
		return res, xerrors.Errorf("could not find /etc/os-release in %s", imageRef)
	} else if err != nil {
		return res, err
	}

	return builddef.ParseOSRelease(raw)
}
