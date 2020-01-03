package statesolver

import (
	"bytes"
	"context"
	"strings"

	"github.com/NiR-/zbuild/pkg/llbutils"
	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/frontend/gateway/client"
	"golang.org/x/xerrors"
)

const (
	keyDockerContext = "contextkey"
	keyContext       = "context"
)

func NewBuildkitSolver(c client.Client) BuildkitSolver {
	sessionID := c.BuildOpts().SessionID
	opts := c.BuildOpts().Opts

	contextName := "context"
	if v, ok := opts[keyDockerContext]; ok {
		contextName = v
	} else if v, ok := opts[keyContext]; ok {
		contextName = v
	}

	return BuildkitSolver{
		client:      c,
		sessionID:   sessionID,
		contextName: contextName,
	}
}

type BuildkitSolver struct {
	client      client.Client
	sessionID   string
	contextName string
}

func (s BuildkitSolver) ExecImage(
	ctx context.Context,
	imageRef string,
	cmd []string,
) (*bytes.Buffer, error) {
	escapedCmd := strings.Replace(strings.Join(cmd, "; "), "\"", "\\\"", -1)
	src := llbutils.ImageSource(imageRef, false)
	run := src.Run(
		llb.Shlex("/bin/sh -o errexit -c \"" + escapedCmd + "\" > /tmp/result"))

	_, ref, err := llbutils.SolveState(ctx, s.client, run.Root())
	if err != nil {
		return nil, err
	}

	raw, ok, err := llbutils.ReadFile(ctx, ref, "/tmp/result")
	buf := bytes.NewBuffer(raw)
	if err != nil {
		err = xerrors.Errorf("failed to execute %q in %q: %w", escapedCmd, imageRef, err)
		return buf, err
	} else if !ok {
		err = xerrors.Errorf("failed to execute %q in %q", escapedCmd, imageRef)
		return buf, err
	}

	return buf, nil
}

func (s BuildkitSolver) FromBuildContext(opts ...llb.LocalOption) ReadFileOpt {
	opts = append(opts, llb.SessionID(s.sessionID))
	src := llbutils.BuildContext(s.contextName, opts...)

	return func(ctx context.Context, filepath string) ([]byte, error) {
		raw, err := s.readFromLLB(ctx, src, filepath)
		if err != nil {
			return raw, xerrors.Errorf("failed to read %s from build context: %w", filepath, err)
		}

		return raw, nil
	}
}

func (s BuildkitSolver) FromImage(image string) ReadFileOpt {
	return func(ctx context.Context, filepath string) ([]byte, error) {
		src := llbutils.ImageSource(image, false)
		raw, err := s.readFromLLB(ctx, src, filepath)
		if err != nil {
			return raw, xerrors.Errorf("failed to read %s from %s: %w", filepath, image, err)
		}

		return raw, nil
	}
}

func (s BuildkitSolver) readFromLLB(
	ctx context.Context,
	src llb.State,
	filepath string,
) ([]byte, error) {
	_, srcRef, err := llbutils.SolveState(ctx, s.client, src)
	if err != nil {
		return nil, err
	}

	raw, ok, err := llbutils.ReadFile(ctx, srcRef, filepath)
	if err != nil {
		return raw, err
	} else if !ok {
		return raw, FileNotFound
	}
	return raw, nil
}

func (s BuildkitSolver) ReadFile(ctx context.Context, filepath string, opt ReadFileOpt) ([]byte, error) {
	return opt(ctx, filepath)
}
