package webserver_test

import (
	"context"
	"testing"

	"github.com/NiR-/zbuild/pkg/builddef"
	"github.com/NiR-/zbuild/pkg/defkinds/webserver"
	"github.com/NiR-/zbuild/pkg/mocks"
	"github.com/NiR-/zbuild/pkg/pkgsolver"
	"github.com/NiR-/zbuild/pkg/statesolver"
	"github.com/golang/mock/gomock"
	"gopkg.in/yaml.v2"
)

type updateLocksTC struct {
	opts        builddef.UpdateLocksOpts
	handler     *webserver.WebserverHandler
	pkgSolvers  pkgsolver.PackageSolversMap
	expected    string
	expectedErr error
}

var rawDebianBusterOSRelease = []byte(`PRETTY_NAME="Debian GNU/Linux 10 (buster)"
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
	solver.EXPECT().ResolveImageRef(
		gomock.Any(), "docker.io/library/nginx:latest",
	).Return("docker.io/library/nginx:latest@sha256", nil)

	solver.EXPECT().FromImage("docker.io/library/nginx:latest@sha256").Times(1)
	solver.EXPECT().ReadFile(
		gomock.Any(),
		"/etc/os-release",
		gomock.Any(),
	).AnyTimes().Return(rawDebianBusterOSRelease, nil)

	pkgSolver := mocks.NewMockPackageSolver(mockCtrl)
	pkgSolver.EXPECT().ResolveVersions(
		gomock.Any(),
		"docker.io/library/nginx:latest@sha256",
		map[string]string{"curl": "*"},
	).Times(1).Return(map[string]string{
		"curl": "7.64.0-4",
	}, nil)

	h := &webserver.WebserverHandler{}
	h.WithSolver(solver)

	return updateLocksTC{
		opts: builddef.UpdateLocksOpts{
			BuildOpts: &builddef.BuildOpts{
				Def: loadBuildDef(t, "testdata/locks/definition.yml"),
			},
			UpdateImageRef:       true,
			UpdateSystemPackages: true,
		},
		handler: h,
		pkgSolvers: pkgsolver.PackageSolversMap{
			pkgsolver.APT: func(statesolver.StateSolver) pkgsolver.PackageSolver {
				return pkgSolver
			},
		},
		expected: "testdata/locks/definition.lock",
	}
}

var rawDebianBullseyeOSRelease = []byte(`PRETTY_NAME="Debian GNU/Linux 11 (bullseye)"
NAME="Debian GNU/Linux"
VERSION_ID="11"
VERSION="11 (bullseye)"
VERSION_CODENAME=bullseye
ID=debian
HOME_URL="https://www.debian.org/"
SUPPORT_URL="https://www.debian.org/support"
BUG_REPORT_URL="https://bugs.debian.org/"`)

func initUpdateImageRefOnlyTC(t *testing.T, mockCtrl *gomock.Controller) updateLocksTC {
	solver := mocks.NewMockStateSolver(mockCtrl)
	solver.EXPECT().ResolveImageRef(
		gomock.Any(), "docker.io/library/nginx:latest",
	).Return("docker.io/library/nginx:latest@some-updated-sha256", nil)

	solver.EXPECT().FromImage("docker.io/library/nginx:latest@some-updated-sha256").Times(1)
	solver.EXPECT().ReadFile(
		gomock.Any(),
		"/etc/os-release",
		gomock.Any(),
	).AnyTimes().Return(rawDebianBullseyeOSRelease, nil)

	h := &webserver.WebserverHandler{}
	h.WithSolver(solver)

	return updateLocksTC{
		opts: builddef.UpdateLocksOpts{
			BuildOpts: &builddef.BuildOpts{
				Def: loadBuildDefWithLocks(t, "testdata/locks/definition.yml"),
			},
			UpdateImageRef:       true,
			UpdateSystemPackages: false,
		},
		handler:    h,
		pkgSolvers: pkgsolver.PackageSolversMap{},
		expected:   "testdata/locks/update-image-ref-only.lock",
	}
}

func initUpdateSystemPackagesOnlyTC(t *testing.T, mockCtrl *gomock.Controller) updateLocksTC {
	solver := mocks.NewMockStateSolver(mockCtrl)

	pkgSolver := mocks.NewMockPackageSolver(mockCtrl)
	pkgSolver.EXPECT().ResolveVersions(
		gomock.Any(),
		"docker.io/library/nginx:latest@sha256",
		map[string]string{"curl": "*"},
	).Times(1).Return(map[string]string{
		"curl": "7.64.0-4",
	}, nil)

	h := &webserver.WebserverHandler{}
	h.WithSolver(solver)

	return updateLocksTC{
		opts: builddef.UpdateLocksOpts{
			BuildOpts: &builddef.BuildOpts{
				Def: loadBuildDefWithLocks(t, "testdata/locks/definition.yml"),
			},
			UpdateImageRef:       false,
			UpdateSystemPackages: true,
		},
		handler: h,
		pkgSolvers: pkgsolver.PackageSolversMap{
			pkgsolver.APT: func(statesolver.StateSolver) pkgsolver.PackageSolver {
				return pkgSolver
			},
		},
		expected: "testdata/locks/definition.lock",
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
		"successfully update locks": initSuccessfullyUpdateLocksTC,
		"only image ref":            initUpdateImageRefOnlyTC,
		"only system packages":      initUpdateSystemPackagesOnlyTC,
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
				writeTestdata(t, tc.expected, string(rawLocks))
				return
			}

			expectedRaw := loadRawTestdata(t, tc.expected)
			if string(expectedRaw) != string(rawLocks) {
				t.Fatalf("Expected: %s\nGot: %s", expectedRaw, rawLocks)
			}
		})
	}
}

func loadGenericDef(t *testing.T, filepath string) builddef.BuildDef {
	raw := loadRawTestdata(t, filepath)

	var def builddef.BuildDef
	if err := yaml.Unmarshal(raw, &def); err != nil {
		t.Fatal(err)
	}

	return def
}
