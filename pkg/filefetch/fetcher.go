package filefetch

import "context"

type FileFetcher interface {
	FetchFile(ctx context.Context, image, path string) ([]byte, error)
}
