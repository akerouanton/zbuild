package statesolver

import (
	"context"

	"github.com/moby/buildkit/client/llb"
	"golang.org/x/xerrors"
)

type StateSolver interface {
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
