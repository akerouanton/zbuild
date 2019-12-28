package nodejs_test

import (
	"context"
	"errors"
	"flag"
	"io/ioutil"
	"testing"

	"github.com/NiR-/zbuild/pkg/builddef"
	"github.com/NiR-/zbuild/pkg/defkinds/nodejs"
	"github.com/NiR-/zbuild/pkg/mocks"
	"github.com/NiR-/zbuild/pkg/pkgsolver"
	"github.com/golang/mock/gomock"
	"gopkg.in/yaml.v2"
)

var flagTestdata = flag.Bool("testdata", false, "Use this flag to (re)generate testdata (dumps of LLB states and lockfiles)")

type updateLocksTC struct {
	file      string
	handler   *nodejs.NodeJSHandler
	pkgSolver pkgsolver.PackageSolver
	// expected is the path to a lock file in testdata/ folder
	expected    string
	expectedErr error
}

var rawDebianOSRelease = []byte(`PRETTY_NAME="Debian GNU/Linux 10 (buster)"
NAME="Debian GNU/Linux"
VERSION_ID="10"
VERSION="10 (buster)"
VERSION_CODENAME=buster
ID=debian
HOME_URL="https://www.debian.org/"
SUPPORT_URL="https://www.debian.org/support"
BUG_REPORT_URL="https://bugs.debian.org/"`)

func initSuccessfullyUpdateLocksTC(t *testing.T, mockCtrl *gomock.Controller) updateLocksTC {
	solver := mocks.NewMockStateSolver(mockCtrl)
	solver.EXPECT().FromImage("docker.io/library/node:12-buster-slim").Times(1)
	solver.EXPECT().ReadFile(
		gomock.Any(),
		"/etc/os-release",
		gomock.Any(),
	).Return(rawDebianOSRelease, nil)

	pkgSolver := mocks.NewMockPackageSolver(mockCtrl)
	pkgSolver.EXPECT().Configure(gomock.Any(), "amd64").Times(1)
	pkgSolver.EXPECT().ResolveVersions(map[string]string{
		"curl": "*",
	}).AnyTimes().Return(map[string]string{
		"curl": "curl-version",
	}, nil)

	h := nodejs.NodeJSHandler{}
	h.WithSolver(solver)

	return updateLocksTC{
		file:      "testdata/locks/without-stages.yml",
		handler:   &h,
		pkgSolver: pkgSolver,
		expected:  "testdata/locks/without-stages.lock",
	}
}

var rawAlpineOSRelease = []byte(`NAME="Alpine Linux"
ID=alpine
VERSION_ID=3.10.3
PRETTY_NAME="Alpine Linux v3.10"
HOME_URL="https://alpinelinux.org/"
BUG_REPORT_URL="https://bugs.alpinelinux.org/"
`)

func failToUpdateLocksForAlpineBaseImageTC(t *testing.T, mockCtrl *gomock.Controller) updateLocksTC {
	solver := mocks.NewMockStateSolver(mockCtrl)
	solver.EXPECT().FromImage("docker.io/library/node:12-alpine").Times(1)
	solver.EXPECT().ReadFile(
		gomock.Any(),
		"/etc/os-release",
		gomock.Any(),
	).Return(rawAlpineOSRelease, nil)

	h := nodejs.NodeJSHandler{}
	h.WithSolver(solver)

	return updateLocksTC{
		file:        "testdata/locks/alpine.yml",
		handler:     &h,
		pkgSolver:   mocks.NewMockPackageSolver(mockCtrl),
		expectedErr: errors.New("unsupported OS \"alpine\": only debian-based base images are supported"),
	}
}

func initSuccessfullyUpdateWebserverLocksTC(t *testing.T, mockCtrl *gomock.Controller) updateLocksTC {
	solver := mocks.NewMockStateSolver(mockCtrl)
	solver.EXPECT().FromImage("docker.io/library/node:12-buster-slim").Times(1)
	solver.EXPECT().ReadFile(
		gomock.Any(),
		"/etc/os-release",
		gomock.Any(),
	).Return(rawDebianOSRelease, nil)
	solver.EXPECT().FromImage("docker.io/library/nginx:latest").Times(1)
	solver.EXPECT().ReadFile(
		gomock.Any(),
		"/etc/os-release",
		gomock.Any(),
	).Return(rawDebianOSRelease, nil)

	pkgSolver := mocks.NewMockPackageSolver(mockCtrl)
	pkgSolver.EXPECT().Configure(gomock.Any(), "amd64").Times(2)
	pkgSolver.EXPECT().ResolveVersions(map[string]string{}).Return(map[string]string{}, nil).Times(2)
	pkgSolver.EXPECT().ResolveVersions(map[string]string{
		"curl": "*",
	}).Return(map[string]string{
		"curl": "curl-version",
	}, nil).Times(1)

	h := nodejs.NodeJSHandler{}
	h.WithSolver(solver)

	// @TODO: use proper default values for webserver definition
	// when used from another defkind
	return updateLocksTC{
		file:      "testdata/locks/with-webserver.yml",
		handler:   &h,
		pkgSolver: pkgSolver,
		expected:  "testdata/locks/with-webserver.lock",
	}
}

func TestUpdateLocks(t *testing.T) {
	testcases := map[string]func(*testing.T, *gomock.Controller) updateLocksTC{
		"successfully update locks":                        initSuccessfullyUpdateLocksTC,
		"successfully update webserver locks":              initSuccessfullyUpdateWebserverLocksTC,
		"fail to update locks for alpine based base image": failToUpdateLocksForAlpineBaseImageTC,
	}

	for tcname := range testcases {
		tcinit := testcases[tcname]

		t.Run(tcname, func(t *testing.T) {
			t.Parallel()

			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			tc := tcinit(t, mockCtrl)
			genericDef := loadGenericDef(t, tc.file, "")

			var locks builddef.Locks
			var rawLocks []byte
			var err error

			ctx := context.Background()
			locks, err = tc.handler.UpdateLocks(ctx, tc.pkgSolver, &genericDef)
			if err == nil {
				rawLocks, err = locks.RawLocks()
			}

			if tc.expectedErr != nil {
				if err == nil || err.Error() != tc.expectedErr.Error() {
					t.Fatalf("Expected error: %v\nGot: %v", tc.expectedErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if *flagTestdata {
				if tc.expected != "" {
					writeTestdata(t, tc.expected, string(rawLocks))
				}
				return
			}

			expectedRaw := string(loadRawTestdata(t, tc.expected))
			if expectedRaw != string(rawLocks) {
				tempfile := newTempFile(t)
				ioutil.WriteFile(tempfile, rawLocks, 0640)

				t.Fatalf("Expected: <%s>\nGot: <%s>", tc.expected, tempfile)
			}
		})
	}
}

func loadGenericDef(t *testing.T, filepath, lockpath string) builddef.BuildDef {
	raw := loadRawTestdata(t, filepath)

	var def builddef.BuildDef
	if err := yaml.Unmarshal(raw, &def); err != nil {
		t.Fatal(err)
	}

	if lockpath != "" {
		lockContent, err := ioutil.ReadFile(lockpath)
		if err != nil {
			t.Fatal(err)
		}
		def.RawLocks = lockContent
	}

	return def
}

func newTempFile(t *testing.T) string {
	file, err := ioutil.TempFile("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	return file.Name()
}

func writeTestdata(t *testing.T, filepath string, content string) {
	err := ioutil.WriteFile(filepath, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Could not write %q: %v", filepath, err)
	}
}
