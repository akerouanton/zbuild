package statesolver

import (
	"context"
	"fmt"
	"strings"

	"github.com/NiR-/zbuild/pkg/builddef"
)

var (
	imageGit = "docker.io/akerouanton/zbuild-git:v0.1"
)

func LockContext(ctx context.Context, solver StateSolver, c *builddef.Context) (*builddef.Context, error) {
	if c == nil {
		return nil, nil
	}

	// Only git contexts can be locked to a precise version.
	if !c.IsGitContext() {
		return c, nil
	}

	repoURI := normalizeRepoURI(c)
	sourceRef := sourceRefOrHead(c)

	cmd := []string{
		fmt.Sprintf("git clone --quiet %s /tmp/repo 1>/dev/null 2>&1", repoURI),
		"cd /tmp/repo",
		fmt.Sprintf("git rev-parse -q --verify '%s'", sourceRef)}
	out, err := solver.ExecImage(ctx, imageGit, cmd)
	if err != nil {
		return c, err
	}

	locked := c.Copy()
	locked.Reference = strings.Trim(out.String(), "\n")
	return locked, nil
}

func normalizeRepoURI(c *builddef.Context) string {
	repoURI := c.Source
	if !strings.HasPrefix(repoURI, "git://") {
		repoURI = "git://" + repoURI
	}
	return repoURI
}

func sourceRefOrHead(c *builddef.Context) string {
	sourceRef := "HEAD"
	if c.Reference != "" {
		sourceRef = c.Reference
	}
	return sourceRef
}
