package builddef_test

import (
	"context"
	"testing"

	"github.com/NiR-/webdf/pkg/builddef"
	"github.com/go-test/deep"
)

func TestResolveImageOS(t *testing.T) {
	testcases := map[string]struct {
		imageRef string
		file     []byte
		expected builddef.OSRelease
	}{
		"successfully parse an os-release file": {
			imageRef: "debian:buster-20191118-slim",
			file: []byte(`
PRETTY_NAME="Debian GNU/Linux 9 (stretch)"
NAME="Debian GNU/Linux"
VERSION_ID="9"
VERSION="9 (stretch)"
VERSION_CODENAME=stretch
ID=debian
HOME_URL="https://www.debian.org/"
SUPPORT_URL="https://www.debian.org/support"
BUG_REPORT_URL="https://bugs.debian.org/"`),
			expected: builddef.OSRelease{
				Name:        "debian",
				VersionName: "stretch",
				VersionID:   9,
			},
		},
	}

	for tcname := range testcases {
		tc := testcases[tcname]

		t.Run(tcname, func(t *testing.T) {
			t.Parallel()

			ctx := context.TODO()
			fetcher := mockFileFetcher{output: tc.file}
			res, err := builddef.ResolveImageOS(ctx, fetcher, tc.imageRef)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if diff := deep.Equal(tc.expected, res); diff != nil {
				t.Fatal(diff)
			}
		})
	}
}

// @TODO: use gomock instead
type mockFileFetcher struct {
	err    error
	output []byte
}

func (f mockFileFetcher) FetchFile(ctx context.Context, image, path string) ([]byte, error) {
	return f.output, f.err
}
