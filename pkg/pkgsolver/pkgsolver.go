package pkgsolver

import (
	"context"
	"fmt"
	"strings"

	"github.com/NiR-/zbuild/pkg/statesolver"
	"golang.org/x/xerrors"
)

// PackageSolver is an interface used by specialzed defkinds to lock system
// packages.
//
// Note that PackageSolvers are structs that implement this interface instead
// of just being functions implementing ResolveVersions signature because
// gomock supports only interfaces.
type PackageSolver interface {
	// ResolveVersions takes the reference of a container image where the
	// resolution should happen as first arg. It also takes a map of packages
	// to resolve associated with their version constraint. It returns a map of
	// packages associated with their resolved version. It returns an error if
	// one of the package cannot be resolved.
	//
	// The set of supported version constraint formats depends of each
	// implementation.
	ResolveVersions(ctx context.Context, imageRef string, pkgs map[string]string) (map[string]string, error)
}

type SolverType string

const (
	APT SolverType = "apt"
	APK SolverType = "apk"
)

// PackageSolversMap is a list of SolverType associated to matching factories
// for PackageSolver implementations. Specialized defkind handlers use this to
// pin packages to specific versions.
type PackageSolversMap map[SolverType]func(statesolver.StateSolver) PackageSolver

// New create a new PackageSolver for the given SolverType.
func (pkgSolvers PackageSolversMap) New(
	solverType SolverType,
	solver statesolver.StateSolver,
) PackageSolver {
	factory, ok := pkgSolvers[solverType]
	if !ok {
		panic(fmt.Sprintf("No package solver %q found.", solverType))
	}
	return factory(solver)
}

// DefaultPackageSolversMap contains the default set of package solver
// factories used by zbuild (and zbuilder).
var DefaultPackageSolversMap = PackageSolversMap{
	// Each New*Solver have more specialized return types than the functions
	// here so the latter are here only to make New*Solver functions compatible
	// with PackageSolversMap.
	APT: func(solver statesolver.StateSolver) PackageSolver {
		return NewAPTSolver(solver)
	},
	APK: func(solver statesolver.StateSolver) PackageSolver {
		return NewAPKSolver(solver)
	},
}

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
