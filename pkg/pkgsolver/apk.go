package pkgsolver

import (
	"bytes"
	"context"
	"regexp"
	"strings"

	"github.com/NiR-/zbuild/pkg/statesolver"
)

type APKSolver struct {
	solver statesolver.StateSolver
}

func NewAPKSolver(solver statesolver.StateSolver) *APKSolver {
	return &APKSolver{
		solver: solver,
	}
}

func (s *APKSolver) ResolveVersions(
	imageRef string,
	pkgs map[string]string,
) (map[string]string, error) {
	resolved := map[string]string{}
	toResolve := make([]string, 0, len(pkgs))

	for name, version := range pkgs {
		if version != "" && version != "*" {
			// @TODO: check if the given version is valid
			resolved[name] = version
		}
		toResolve = append(toResolve, name)
	}

	if len(toResolve) == 0 {
		return resolved, nil
	}

	cmd := make([]string, 4, len(pkgs)+4)
	cmd[0] = "apk"
	cmd[1] = "--no-cache"
	cmd[2] = "info"
	cmd[3] = "--description"
	cmd = append(cmd, toResolve...)

	ctx := context.Background()
	outbuf, err := s.solver.ExecImage(ctx, imageRef, []string{
		strings.Join(cmd, " "),
	})
	// Unfortunately APK returns exit code 1 when a package is not found but
	// it doesn't provide any error message at all.
	if err != nil && !strings.Contains(err.Error(), "exited with code 1") {
		return resolved, err
	}

	resolved = s.parseAPKInfo(outbuf, toResolve, resolved)
	err = checkMissingPackages(pkgs, resolved)

	return resolved, err
}

func (s *APKSolver) parseAPKInfo(
	buf *bytes.Buffer,
	pkgNames []string,
	res map[string]string,
) map[string]string {
	exp, err := s.buildDescriptionRegexp(pkgNames)
	if err != nil {
		return res
	}

	matches := exp.FindAllSubmatch(buf.Bytes(), -1)
	for _, submatches := range matches {
		if len(submatches) != 3 {
			continue
		}

		name := string(submatches[1])
		version := string(submatches[2])
		res[name] = version
	}

	return res
}

func (s APKSolver) buildDescriptionRegexp(pkgNames []string) (*regexp.Regexp, error) {
	expstr := "(" + strings.Join(pkgNames, "|") + ")-([^ ]+) description:"
	return regexp.Compile(expstr)
}
