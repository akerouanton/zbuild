package statesolver_test

import (
	"context"
	"encoding/json"
	"io"
	"testing"

	"github.com/NiR-/zbuild/pkg/statesolver"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"golang.org/x/xerrors"
)

const debianBusterSlimRef = "debian:buster-20191014-slim"
const debianBullseyeSlimRef = "debian:bullseye-slim"

type dockerReadFileTC struct {
	opt         statesolver.ReadFileOpt
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

func initFetchOSReleaseFromImageTC(t *testing.T, solver statesolver.DockerSolver) dockerReadFileTC {
	return dockerReadFileTC{
		opt:  solver.FromImage(debianBusterSlimRef),
		path: "/etc/os-release",
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

func initFailToFetchNonexistantPathFromImageTC(t *testing.T, solver statesolver.DockerSolver) dockerReadFileTC {
	return dockerReadFileTC{
		opt:  solver.FromImage(debianBusterSlimRef),
		path: "/etc/nonexistant",
		expectedErr: xerrors.Errorf(
			"failed to read /etc/nonexistant from %s: %w",
			debianBusterSlimRef, statesolver.FileNotFound,
		),
	}
}

func initPullImageAndReadFileFromImageTC(t *testing.T, solver statesolver.DockerSolver) dockerReadFileTC {
	return dockerReadFileTC{
		opt:  solver.FromImage("debian:bullseye-slim"),
		path: "/etc/os-release",
		expected: `PRETTY_NAME="Debian GNU/Linux bullseye/sid"
NAME="Debian GNU/Linux"
ID=debian
HOME_URL="https://www.debian.org/"
SUPPORT_URL="https://www.debian.org/support"
BUG_REPORT_URL="https://bugs.debian.org/"
`,
	}
}

func initFailToFetchFromNonexistantImageTC(t *testing.T, solver statesolver.DockerSolver) dockerReadFileTC {
	return dockerReadFileTC{
		opt:         solver.FromImage("akerouanton/nopenopenope"),
		path:        "/etc/os-release",
		expectedErr: xerrors.New("failed to read /etc/os-release from akerouanton/nopenopenope: Error response from daemon: pull access denied for akerouanton/nopenopenope, repository does not exist or may require 'docker login'"),
	}
}

func initDockerReadFileFromBuildContextTC(t *testing.T, solver statesolver.DockerSolver) dockerReadFileTC {
	return dockerReadFileTC{
		opt:      solver.FromBuildContext(),
		path:     "testfile",
		expected: string(loadRawTestdata(t, "testdata/testfile")),
	}
}

func initFailToReadNonexistantFileFromBuildContextTC(t *testing.T, solver statesolver.DockerSolver) dockerReadFileTC {
	return dockerReadFileTC{
		opt:         solver.FromBuildContext(),
		path:        "nonexistant",
		expectedErr: xerrors.Errorf("failed to read nonexistant from build context: %w", statesolver.FileNotFound),
	}
}

func TestDockerReadFile(t *testing.T) {
	testcases := map[string]func(*testing.T, statesolver.DockerSolver) dockerReadFileTC{
		"fetch /etc/os-release from image":                 initFetchOSReleaseFromImageTC,
		"fail to fetch nonexistant path":                   initFailToFetchNonexistantPathFromImageTC,
		"pull image and fetch /etc/os-release":             initPullImageAndReadFileFromImageTC,
		"fail to fetch from nonexistant image":             initFailToFetchFromNonexistantImageTC,
		"read file from build context":                     initDockerReadFileFromBuildContextTC,
		"fail to read nonexistant file from build context": initFailToReadNonexistantFileFromBuildContextTC,
	}

	c := newDockerClient(t)
	pullImage(t, c, debianBusterSlimRef)
	removeImage(t, c, debianBullseyeSlimRef)

	for tcname := range testcases {
		tcinit := testcases[tcname]

		t.Run(tcname, func(t *testing.T) {
			t.Parallel()

			solver := statesolver.DockerSolver{
				Client:  c,
				Labels:  map[string]string{},
				RootDir: "testdata",
			}
			tc := tcinit(t, solver)

			ctx := context.Background()
			res, err := solver.ReadFile(ctx, tc.path, tc.opt)

			if tc.expectedErr != nil {
				if err == nil || err.Error() != tc.expectedErr.Error() {
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

func pullImage(t *testing.T, c *client.Client, imgRef string) {
	ctx := context.Background()
	r, err := c.ImagePull(ctx, imgRef, types.ImagePullOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	decoder := json.NewDecoder(r)
	for {
		var msg map[string]interface{}
		if err := decoder.Decode(&msg); err != nil {
			if err == io.EOF {
				break
			}
			t.Fatal(err)
		}
	}
}

func removeImage(t *testing.T, c *client.Client, imgRef string) {
	ctx := context.Background()
	_, err := c.ImageRemove(ctx, "debian:bullseye-slim", types.ImageRemoveOptions{
		Force: true,
	})
	if err != nil && !client.IsErrNotFound(err) {
		t.Fatal(err)
	}
}
