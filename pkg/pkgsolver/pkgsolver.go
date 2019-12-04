package pkgsolver

import (
	"github.com/NiR-/zbuild/pkg/builddef"
	"golang.org/x/xerrors"
)

// PackageSolver is used by specialzed defkinds to lock system packages.
type PackageSolver interface {
	// Configure sets the package "suites" that should be used for subsequent
	// calls to ResolveVersions(). See GuessSolverConfig()
	Configure(config SolverConfig) error
	// ResolveVersions takes a map of packages to resolve, associated with their
	// version constraint. It returns a map of packages associated with their
	// resolved version. If one of the package cannot be resolved or if the
	// given arch is not supported, it returns an error.
	ResolveVersions(pkgs map[string]string) (map[string]string, error)
	// Type returns the SolverType matching the current instance of
	// PackageSolver.
	Type() SolverType
}

type SolverType string

const (
	Dpkg SolverType = "dpkg"
	// Apk  SolverType = "apk"
)

type SolverConfig struct {
	Arch string
	// See https://wiki.debian.org/SourcesList
	DpkgSuites [][]string
}

func GuessSolverConfig(osrelease builddef.OSRelease, arch string) (SolverConfig, error) {
	solverType := Dpkg
	if osrelease.Name == "alpine" {
		return SolverConfig{}, xerrors.New("alpine is not supported yet")
	}

	dpkgSuites := [][]string{}
	// apkSuites := [][]string{}

	switch solverType {
	case Dpkg:
		// @TODO: load these suites from the base image instead of guessing them based on version codename
		// because of that, zbuild only supports debian for now
		dpkgSuites = [][]string{
			{"http://deb.debian.org/debian", osrelease.VersionName},
			{"http://deb.debian.org/debian", osrelease.VersionName + "-updates"},
			{"http://security.debian.org", osrelease.VersionName + "/updates"},
		}
	default:
		return SolverConfig{}, xerrors.Errorf("%q package solver is not supported", solverType)
	}

	config := SolverConfig{
		Arch:       arch,
		DpkgSuites: dpkgSuites,
		// apkSuites: apkSuites,
	}
	return config, nil
}
