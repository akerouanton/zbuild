package statesolver

import "context"

type StateSolver interface {
	FetchFile(ctx context.Context, image, path string) ([]byte, error)
}
