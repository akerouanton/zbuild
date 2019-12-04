package pkgsolver

import (
	"github.com/snyh/go-dpkg-parser"
	"golang.org/x/xerrors"
)

type DpkgSolver struct {
	arch     string
	dpkgRepo *dpkg.Repository
}

func NewDpkgSolver(dpkgRepo *dpkg.Repository) *DpkgSolver {
	return &DpkgSolver{
		arch:     "",
		dpkgRepo: dpkgRepo,
	}
}

func (s *DpkgSolver) Configure(config SolverConfig) error {
	for _, suite := range config.DpkgSuites {
		if err := s.dpkgRepo.AddSuite(suite[0], suite[1], ""); err != nil {
			return xerrors.Errorf("could not add suite %s: %w", suite[1], err)
		}
	}

	s.arch = config.Arch
	return nil
}

func (s *DpkgSolver) ResolveVersions(pkgs map[string]string) (map[string]string, error) {
	versions := make(map[string]string)

	ar, err := s.dpkgRepo.Archive(s.arch)
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

func (s *DpkgSolver) Type() SolverType {
	return Dpkg
}
