package builddef

import (
	"strings"

	"golang.org/x/xerrors"
)

// OSRelease represents the data about a Linux distribution as read from
// /etc/os-release. This struct is generally part of specialized definition
// locks and is used to determine which packages should be picked (e.g. php
// system packages inference).
type OSRelease struct {
	Name        string
	VersionName string
	VersionID   string
}

// ParseOSRelease takes the raw content of a /etc/os-release file and
// transforms it into an OSRelease.
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

	if res.Name == "" {
		return res, xerrors.New("invalid os-release content: no field ID found")
	}
	if res.VersionID == "" {
		return res, xerrors.New("invalid os-release content: no field VERSION_ID found")
	}

	return res, nil
}
