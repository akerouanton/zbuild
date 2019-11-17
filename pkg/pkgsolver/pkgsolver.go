package pkgsolver

import (
	"github.com/snyh/go-dpkg-parser"
	"golang.org/x/xerrors"
)

type PackageSolver struct {
	dpkgRepo *dpkg.Repository
}

func NewPackageSolver(dpkgRepo *dpkg.Repository) PackageSolver {
	return PackageSolver{dpkgRepo}
}

// WithDpkgSuites takes a list of string pairs: the first string is the
// repository URL and second string is the distribution name.
// See https://wiki.debian.org/SourcesList
func (s PackageSolver) WithDpkgSuites(suites [][]string) error {
	for _, suite := range suites {
		if err := s.dpkgRepo.AddSuite(suite[0], suite[1], ""); err != nil {
			return xerrors.Errorf("could not add suite %s: %v", suite[1], err)
		}
	}
	return nil
}

// ResolveVersions takes a map of packages to resolve associated with their
// version constraint and the architecture to resolve package for. It returns
// a map of packages associated with their resolved version. If one of the
// package cannot be resolved or if the given arch is not supported, it returns
// an error.
func (s PackageSolver) ResolveVersions(
	pkgs map[string]string,
	arch string,
) (map[string]string, error) {
	versions := make(map[string]string, len(pkgs))

	ar, err := s.dpkgRepo.Archive(arch)
	if err != nil {
		return versions, err
	}

	for pkg := range pkgs {
		bp, err := ar.FindBinary(pkg)
		if err != nil {
			return versions, err
		}

		versions[pkg] = bp.Version
	}

	return versions, nil
}
