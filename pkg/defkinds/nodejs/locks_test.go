package nodejs_test

import (
	"context"
	"flag"
	"io/ioutil"
	"testing"

	"github.com/NiR-/zbuild/pkg/builddef"
	"github.com/NiR-/zbuild/pkg/defkinds/nodejs"
	"github.com/NiR-/zbuild/pkg/mocks"
	"github.com/NiR-/zbuild/pkg/pkgsolver"
	"github.com/NiR-/zbuild/pkg/statesolver"
	"github.com/golang/mock/gomock"
	"gopkg.in/yaml.v2"
)

var flagTestdata = flag.Bool("testdata", false, "Use this flag to (re)generate testdata (dumps of LLB states and lockfiles)")

type updateLocksTC struct {
	opts       builddef.UpdateLocksOpts
	handler    *nodejs.NodeJSHandler
	pkgSolvers pkgsolver.PackageSolversMap
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

func initUpdateLocksForDebianTC(t *testing.T, mockCtrl *gomock.Controller) updateLocksTC {
	solver := mocks.NewMockStateSolver(mockCtrl)
	solver.EXPECT().ResolveImageRef(
		gomock.Any(), "docker.io/library/node:12-buster-slim",
	).Return("docker.io/library/node:12-buster-slim@sha256", nil)

	solver.EXPECT().FromImage("docker.io/library/node:12-buster-slim@sha256").Times(1)
	solver.EXPECT().ReadFile(
		gomock.Any(),
		"/etc/os-release",
		gomock.Any(),
	).Return(rawDebianOSRelease, nil)

	pkgSolver := mocks.NewMockPackageSolver(mockCtrl)
	pkgSolver.EXPECT().ResolveVersions(
		gomock.Any(),
		"docker.io/library/node:12-buster-slim@sha256",
		map[string]string{"curl": "*"},
	).AnyTimes().Return(map[string]string{
		"curl": "curl-version",
	}, nil)

	h := nodejs.NodeJSHandler{}
	h.WithSolver(solver)

	return updateLocksTC{
		opts: builddef.UpdateLocksOpts{
			BuildOpts: &builddef.BuildOpts{
				Def: loadBuildDef(t, "testdata/locks/debian.yml"),
			},
			UpdateImageRef:       true,
			UpdateSystemPackages: true,
		},
		handler: &h,
		pkgSolvers: pkgsolver.PackageSolversMap{
			pkgsolver.APT: func(statesolver.StateSolver) pkgsolver.PackageSolver {
				return pkgSolver
			},
		},
		expected: "testdata/locks/debian.lock",
	}
}

var rawAlpine3103OSRelease = []byte(`NAME="Alpine Linux"
ID=alpine
VERSION_ID=3.10.3
PRETTY_NAME="Alpine Linux v3.10"
HOME_URL="https://alpinelinux.org/"
BUG_REPORT_URL="https://bugs.alpinelinux.org/"
`)

func initUpdateLocksForAlpineTC(t *testing.T, mockCtrl *gomock.Controller) updateLocksTC {
	solver := mocks.NewMockStateSolver(mockCtrl)
	solver.EXPECT().ResolveImageRef(
		gomock.Any(), "docker.io/library/node:12-alpine",
	).Return("docker.io/library/node:12-alpine@sha256", nil)

	solver.EXPECT().FromImage("docker.io/library/node:12-alpine@sha256").Times(1)
	solver.EXPECT().ReadFile(
		gomock.Any(),
		"/etc/os-release",
		gomock.Any(),
	).Return(rawAlpine3103OSRelease, nil)

	pkgSolver := mocks.NewMockPackageSolver(mockCtrl)
	pkgSolver.EXPECT().ResolveVersions(
		gomock.Any(),
		"docker.io/library/node:12-alpine@sha256",
		map[string]string{"libsass-dev": "*"},
	).AnyTimes().Return(map[string]string{
		"libsass-dev": "1.2.3",
	}, nil)

	h := nodejs.NodeJSHandler{}
	h.WithSolver(solver)

	return updateLocksTC{
		opts: builddef.UpdateLocksOpts{
			BuildOpts: &builddef.BuildOpts{
				Def: loadBuildDef(t, "testdata/locks/alpine.yml"),
			},
			UpdateImageRef:       true,
			UpdateSystemPackages: true,
		},
		handler: &h,
		pkgSolvers: pkgsolver.PackageSolversMap{
			pkgsolver.APK: func(statesolver.StateSolver) pkgsolver.PackageSolver {
				return pkgSolver
			},
		},
		expected: "testdata/locks/alpine.lock",
	}
}

func initUpdateLocksButNotTheImageRefTC(t *testing.T, mockCtrl *gomock.Controller) updateLocksTC {
	solver := mocks.NewMockStateSolver(mockCtrl)

	pkgSolver := mocks.NewMockPackageSolver(mockCtrl)
	pkgSolver.EXPECT().ResolveVersions(
		gomock.Any(),
		"docker.io/library/node:12-alpine@sha256",
		map[string]string{"libsass-dev": "*"},
	).AnyTimes().Return(map[string]string{
		"libsass-dev": "3.2.1",
	}, nil)

	h := nodejs.NodeJSHandler{}
	h.WithSolver(solver)

	return updateLocksTC{
		opts: builddef.UpdateLocksOpts{
			BuildOpts: &builddef.BuildOpts{
				Def: loadBuildDefWithLocks(t, "testdata/locks/alpine.yml"),
			},
			UpdateImageRef:       false,
			UpdateSystemPackages: true,
		},
		handler: &h,
		pkgSolvers: pkgsolver.PackageSolversMap{
			pkgsolver.APK: func(statesolver.StateSolver) pkgsolver.PackageSolver {
				return pkgSolver
			},
		},
		expected: "testdata/locks/expected-no-image-ref-update.lock",
	}
}

var rawAlpine3112OSRelease = []byte(`NAME="Alpine Linux"
ID=alpine
VERSION_ID=3.11.2
PRETTY_NAME="Alpine Linux v3.11"
HOME_URL="https://alpinelinux.org/"
BUG_REPORT_URL="https://bugs.alpinelinux.org/"
`)

func initUpdateLocksButNotSystemPackagesTC(t *testing.T, mockCtrl *gomock.Controller) updateLocksTC {
	solver := mocks.NewMockStateSolver(mockCtrl)
	solver.EXPECT().ResolveImageRef(
		gomock.Any(), "docker.io/library/node:12-alpine",
	).Return("docker.io/library/node:12-alpine@some-other-sha256", nil)

	solver.EXPECT().FromImage("docker.io/library/node:12-alpine@some-other-sha256").Times(1)
	solver.EXPECT().ReadFile(
		gomock.Any(),
		"/etc/os-release",
		gomock.Any(),
	).Return(rawAlpine3112OSRelease, nil)

	pkgSolver := mocks.NewMockPackageSolver(mockCtrl)

	h := nodejs.NodeJSHandler{}
	h.WithSolver(solver)

	return updateLocksTC{
		opts: builddef.UpdateLocksOpts{
			BuildOpts: &builddef.BuildOpts{
				Def: loadBuildDefWithLocks(t, "testdata/locks/alpine.yml"),
			},
			UpdateImageRef:       true,
			UpdateSystemPackages: false,
		},
		handler: &h,
		pkgSolvers: pkgsolver.PackageSolversMap{
			pkgsolver.APK: func(statesolver.StateSolver) pkgsolver.PackageSolver {
				return pkgSolver
			},
		},
		expected: "testdata/locks/expected-no-system-packages-update.lock",
	}
}

func loadBuildDefWithLocks(t *testing.T, filepath string) *builddef.BuildDef {
	def := loadBuildDef(t, filepath)
	def.RawLocks = loadRawLocks(t, builddef.LockFilepath(filepath))

	return def
}

func loadRawLocks(t *testing.T, filepath string) builddef.RawLocks {
	raw := loadRawTestdata(t, filepath)

	var locks builddef.RawLocks
	if err := yaml.Unmarshal(raw, &locks); err != nil {
		t.Fatal(err)
	}

	return locks
}

func TestUpdateLocks(t *testing.T) {
	testcases := map[string]func(*testing.T, *gomock.Controller) updateLocksTC{
		"update locks for debian base iamge":   initUpdateLocksForDebianTC,
		"update locks for alpine base iamge":   initUpdateLocksForAlpineTC,
		"update locks but not the image ref":   initUpdateLocksButNotTheImageRefTC,
		"update locks but not system packages": initUpdateLocksButNotSystemPackagesTC,
	}

	for tcname := range testcases {
		tcinit := testcases[tcname]

		t.Run(tcname, func(t *testing.T) {
			t.Parallel()

			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			tc := tcinit(t, mockCtrl)
			var locks builddef.Locks
			var err error

			ctx := context.Background()
			locks, err = tc.handler.UpdateLocks(ctx, tc.pkgSolvers, tc.opts)
			if tc.expectedErr != nil {
				if err == nil || err.Error() != tc.expectedErr.Error() {
					t.Fatalf("Expected error: %v\nGot: %v", tc.expectedErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			var rawLocks []byte
			rawLocks, err = yaml.Marshal(locks.RawLocks())
			if err != nil {
				t.Fatal(err)
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
				ioutil.WriteFile(tempfile, rawLocks, 0640) //nolint:errcheck

				t.Fatalf("Expected: <%s>\nGot: <%s>", tc.expected, tempfile)
			}
		})
	}
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
