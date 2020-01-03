package pkgsolver

import (
	"strings"

	"golang.org/x/xerrors"
)

// PackageSolver is used by specialzed defkinds to lock system packages.
type PackageSolver interface {
	// ResolveVersions takes the reference of a container image where the
	// resolution should happen as first arg. It also takes a map of packages
	// to resolve associated with their version constraint. It returns a map of
	// packages associated with their resolved version. It returns an error if
	// one of the package cannot be resolved.
	ResolveVersions(imageRef string, pkgs map[string]string) (map[string]string, error)
}

type SolverType string

const (
	APT SolverType = "apt"
	APK SolverType = "apk"
)

func checkMissingPackages(packages, resolved map[string]string) error {
	notResolved := []string{}

	for name := range packages {
		if _, ok := resolved[name]; ok {
			continue
		}
		notResolved = append(notResolved, name)
	}

	if len(notResolved) == 0 {
		return nil
	}

	return xerrors.Errorf("packages %s not found",
		strings.Join(notResolved, ", "))
}
