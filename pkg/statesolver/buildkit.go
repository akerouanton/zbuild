package statesolver

import (
	"context"

	"github.com/NiR-/zbuild/pkg/llbutils"
	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/frontend/gateway/client"
	"golang.org/x/xerrors"
)

const (
	keyNameContext = "contextkey"
)

func NewBuildkitSolver(c client.Client) BuildkitSolver {
	sessionID := c.BuildOpts().SessionID
	opts := c.BuildOpts().Opts

	contextName := "context"
	if v, ok := opts[keyNameContext]; ok {
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

func (s BuildkitSolver) FromBuildContext(opts ...llb.LocalOption) ReadFileOpt {
	opts = append(opts, llb.SessionID(s.sessionID))
	src := llb.Local(s.contextName, opts...)

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
