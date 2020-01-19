package statesolver

import (
	"context"
	"strings"

	"golang.org/x/xerrors"
)

// OSRelease represents the data about the base image OS needed by zbuild. These
// data typically come from /etc/os-release file.
type OSRelease struct {
	Name        string
	VersionName string
	VersionID   string
}

func ParseOSRelease(file []byte) (OSRelease, error) {
	var res OSRelease

	lines := strings.Split(string(file), "\n")
	for _, line := range lines {
		parts := strings.SplitN(line, "=", 2)

		switch parts[0] {
		case "ID":
			res.Name = parts[1]
		case "VERSION_CODENAME":
			res.VersionName = parts[1]
		case "VERSION_ID":
			res.VersionID = strings.Trim(parts[1], "\"")
		}
	}

	return res, nil
}

func ResolveImageOS(
	ctx context.Context,
	solver StateSolver,
	imageRef string,
) (OSRelease, error) {
	var res OSRelease
	raw, err := solver.ReadFile(ctx, "/etc/os-release",
		solver.FromImage(imageRef))
	if xerrors.Is(err, FileNotFound) {
		return res, xerrors.Errorf("could not find /etc/os-release in %s", imageRef)
	} else if err != nil {
		return res, err
	}

	return ParseOSRelease(raw)
}
