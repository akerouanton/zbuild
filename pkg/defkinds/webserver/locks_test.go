package webserver_test

import (
	"context"
	"io/ioutil"
	"testing"

	"github.com/NiR-/zbuild/pkg/builddef"
	"github.com/NiR-/zbuild/pkg/defkinds/webserver"
	"github.com/NiR-/zbuild/pkg/mocks"
	"github.com/NiR-/zbuild/pkg/pkgsolver"
	"github.com/golang/mock/gomock"
	"gopkg.in/yaml.v2"
)

type updateLocksTC struct {
	deffile     string
	handler     *webserver.WebserverHandler
	pkgSolver   pkgsolver.PackageSolver
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
	solver.EXPECT().FromImage("docker.io/library/nginx:latest").Times(1)

	solver.EXPECT().ReadFile(
		gomock.Any(),
		"/etc/os-release",
		gomock.Any(),
	).AnyTimes().Return(rawDebianOSRelease, nil)

	pkgSolver := mocks.NewMockPackageSolver(mockCtrl)
	pkgSolver.EXPECT().ResolveVersions(
		"docker.io/library/nginx:latest",
		map[string]string{"curl": "*"},
	).Times(1).Return(map[string]string{
		"curl": "7.64.0-4",
	}, nil)

	h := &webserver.WebserverHandler{}
	h.WithSolver(solver)

	return updateLocksTC{
		deffile:   "testdata/locks/definition.yml",
		handler:   h,
		pkgSolver: pkgSolver,
		expected:  "testdata/locks/definition.lock",
	}
}

func TestUpdateLocks(t *testing.T) {
	if *flagTestdata {
		return
	}

	testcases := map[string]func(*testing.T, *gomock.Controller) updateLocksTC{
		"successfully update locks": initSuccessfullyUpdateLocksTC,
	}

	for tcname := range testcases {
		tcinit := testcases[tcname]

		t.Run(tcname, func(t *testing.T) {
			t.Parallel()

			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			tc := tcinit(t, mockCtrl)
			genericDef := loadGenericDef(t, tc.deffile, "")

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

			expectedRaw := loadRawTestdata(t, tc.expected)
			if string(expectedRaw) != string(rawLocks) {
				t.Fatalf("Expected: %s\nGot: %s", expectedRaw, rawLocks)
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
