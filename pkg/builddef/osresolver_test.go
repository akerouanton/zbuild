package builddef_test

import (
	"context"
	"testing"

	"github.com/NiR-/zbuild/pkg/builddef"
	"github.com/NiR-/zbuild/pkg/mocks"
	"github.com/go-test/deep"
	"github.com/golang/mock/gomock"
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
				VersionID:   "9",
			},
		},
	}

	for tcname := range testcases {
		tc := testcases[tcname]

		t.Run(tcname, func(t *testing.T) {
			t.Parallel()

			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			ctx := context.TODO()
			fetcher := mocks.NewMockFileFetcher(mockCtrl)
			fetcher.EXPECT().FetchFile(
				ctx, tc.imageRef, "/etc/os-release",
			).Return(tc.file, nil)

			res, err := builddef.ResolveImageOS(ctx, fetcher, tc.imageRef)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if diff := deep.Equal(res, tc.expected); diff != nil {
				t.Fatal(diff)
			}
		})
	}
}
