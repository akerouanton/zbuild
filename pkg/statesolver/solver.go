package statesolver

import (
	"bytes"
	"context"

	"github.com/moby/buildkit/client/llb"
	"golang.org/x/xerrors"
)

type StateSolver interface {
	// ResolveImageRef takes a denormalized image reference (e.g. nginx:latest)
	// and resolve it into a fully-qualified image reference with a digest. The
	// returned image reference can be used in lockfiles.
	// It returns an error if it can't resolve the image ref or if this method
	// is not supported.
	ResolveImageRef(ctx context.Context, imageRef string) (string, error)

	// ExecImage is a method that execute a given command in the given image
	// ref. It returns a byte buffer containing the command stdout. An error is
	// returned if the executed command doesn't return an exit code = 0.
	ExecImage(ctx context.Context, imageRef string, cmd []string) (*bytes.Buffer, error)
	// ReadFile is the method to use to read a given file from either an image
	// or a local source (see From methods). It returns the file content as a
	// byte slice if it's found. If the path couldn't be found, it returns
	// a FileNotFound error.
	ReadFile(ctx context.Context, filepath string, opt ReadFileOpt) ([]byte, error)

	FromBuildContext(opts ...llb.LocalOption) ReadFileOpt
	// Fromimage returns a ReadFileOpt that can be used with the ReadFile
	// method of the same solver to read a given path from an image.
	FromImage(image string) ReadFileOpt
}

type ReadFileOpt func(ctx context.Context, filepath string) ([]byte, error)

var (
	FileNotFound = xerrors.New("file not found")
)
