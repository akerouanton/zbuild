package filefetch_test

import (
	"context"
	"errors"
	"testing"

	"github.com/NiR-/zbuild/pkg/filefetch"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
)

type dockerTC struct {
	fetcher     filefetch.DockerFetcher
	image       string
	path        string
	expected    string
	expectedErr error
}

func newDockerClient(t *testing.T) *client.Client {
	c, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func initFetchOSReleaseTC(t *testing.T) dockerTC {
	c := newDockerClient(t)
	f := filefetch.DockerFetcher{
		Client: c,
		Labels: map[string]string{},
	}
	return dockerTC{
		fetcher: f,
		image:   "debian:buster-20191014-slim",
		path:    "/etc/os-release",
		expected: `PRETTY_NAME="Debian GNU/Linux 10 (buster)"
NAME="Debian GNU/Linux"
VERSION_ID="10"
VERSION="10 (buster)"
VERSION_CODENAME=buster
ID=debian
HOME_URL="https://www.debian.org/"
SUPPORT_URL="https://www.debian.org/support"
BUG_REPORT_URL="https://bugs.debian.org/"
`,
	}
}

func initFailToFetchNonexistantPathTC(t *testing.T) dockerTC {
	c := newDockerClient(t)
	f := filefetch.DockerFetcher{
		Client: c,
		Labels: map[string]string{},
	}
	return dockerTC{
		fetcher:     f,
		image:       "debian:buster-20191014-slim",
		path:        "/etc/nopenopenope",
		expectedErr: errors.New(`path "/etc/nopenopenope" not found in "debian:buster-20191014-slim"`),
	}
}

func initPullImageAndFetchFileTC(t *testing.T) dockerTC {
	c := newDockerClient(t)
	f := filefetch.DockerFetcher{
		Client: c,
		Labels: map[string]string{},
	}

	_, err := c.ImageRemove(context.TODO(), "debian:bullseye-slim", types.ImageRemoveOptions{
		Force: true,
	})
	if err != nil && !client.IsErrNotFound(err) {
		t.Fatal(err)
	}

	return dockerTC{
		fetcher: f,
		image:   "debian:bullseye-slim",
		path:    "/etc/os-release",
		expected: `PRETTY_NAME="Debian GNU/Linux bullseye/sid"
NAME="Debian GNU/Linux"
ID=debian
HOME_URL="https://www.debian.org/"
SUPPORT_URL="https://www.debian.org/support"
BUG_REPORT_URL="https://bugs.debian.org/"
`,
	}
}

func TestFetchFileWithDocker(t *testing.T) {
	testcases := map[string]func(t *testing.T) dockerTC{
		"fetch /etc/os-release":                initFetchOSReleaseTC,
		"fail to fetch nonexistant path":       initFailToFetchNonexistantPathTC,
		"pull image and fetch /etc/os-release": initPullImageAndFetchFileTC,
	}

	for tcname := range testcases {
		tcinit := testcases[tcname]

		t.Run(tcname, func(t *testing.T) {
			t.Parallel()

			tc := tcinit(t)

			ctx := context.TODO()
			res, err := tc.fetcher.FetchFile(ctx, tc.image, tc.path)

			if tc.expectedErr != nil {
				if err == nil {
					t.Fatalf("Expected err: %v\nGot: <nil>", tc.expectedErr)
				}
				if tc.expectedErr.Error() != err.Error() {
					t.Fatalf("Expected error: %v\nGot: %v", tc.expectedErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if string(res) != tc.expected {
				t.Fatalf("Expected: %s\nGot: %s", tc.expected, res)
			}
		})
	}
}
