package pkgsolver

import (
	"bytes"
	"context"
	"strings"

	"github.com/NiR-/zbuild/pkg/statesolver"
)

type APTSolver struct {
	solver statesolver.StateSolver
}

func NewAPTSolver(solver statesolver.StateSolver) *APTSolver {
	return &APTSolver{
		solver: solver,
	}
}

func (s *APTSolver) ResolveVersions(
	ctx context.Context,
	imageRef string,
	pkgs map[string]string,
) (map[string]string, error) {
	resolved := map[string]string{}
	cmd := make([]string, 2, len(pkgs)+2)
	cmd[0] = "apt-cache"
	cmd[1] = "madison"

	for pkg, ver := range pkgs {
		if ver != "" && ver != "*" {
			// @TODO: check if the given version is valid
			resolved[pkg] = ver
			continue
		}
		cmd = append(cmd, pkg)
	}

	if len(cmd) == 2 {
		return resolved, nil
	}

	cmd = []string{
		"apt-get update 1>/dev/null 2>&1",
		strings.Join(cmd, " "),
	}
	outbuf, err := s.solver.ExecImage(ctx, imageRef, cmd)
	if err != nil {
		return resolved, err
	}

	resolved = parseAPTCacheMadison(outbuf, resolved)
	err = checkMissingPackages(pkgs, resolved)

	return resolved, err
}

func parseAPTCacheMadison(
	buf *bytes.Buffer,
	res map[string]string,
) map[string]string {
	for {
		line, err := buf.ReadString('\n')
		if err != nil {
			break
		}

		parts := strings.SplitN(line, " | ", 3)
		if len(parts) != 3 {
			continue
		}

		pkgName := strings.Trim(parts[0], " ")
		pkgVersion := strings.Trim(parts[1], " ")
		if _, ok := res[pkgName]; !ok {
			res[pkgName] = pkgVersion
		}
	}

	return res
}
