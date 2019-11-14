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

func (s PackageSolver) WithDpkgSuites(suites [][]string) error {
	for _, suite := range suites {
		if err := s.dpkgRepo.AddSuite(suite[0], suite[1], ""); err != nil {
			return xerrors.Errorf("could not add suite %s/%s: %v", suite[0], suite[1], err)
		}
	}
	return nil
}

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
