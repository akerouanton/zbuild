package php_test

import (
	"bytes"
	"context"
	"io/ioutil"
	"testing"

	"github.com/NiR-/notpecl/peclapi"
	"github.com/NiR-/notpecl/pecltest"
	"github.com/NiR-/zbuild/pkg/builddef"
	"github.com/NiR-/zbuild/pkg/defkinds/php"
	"github.com/NiR-/zbuild/pkg/mocks"
	"github.com/NiR-/zbuild/pkg/pkgsolver"
	"github.com/NiR-/zbuild/pkg/statesolver"
	"github.com/golang/mock/gomock"
	"gopkg.in/yaml.v2"
)

type updateLocksTC struct {
	// deffile is a path to a yaml file containing a php build definition
	deffile    string
	handler    *php.PHPHandler
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

func initUpdateLocksWithDebianBaseImageTC(t *testing.T, mockCtrl *gomock.Controller) updateLocksTC {
	solver := mocks.NewMockStateSolver(mockCtrl)
	solver.EXPECT().ResolveImageRef(
		gomock.Any(), "docker.io/library/php:7.3-fpm-buster",
	).Return("docker.io/library/php:7.3-fpm-buster@sha256", nil)

	solver.EXPECT().ExecImage(gomock.Any(), "docker.io/library/php:7.3-fpm-buster@sha256", []string{
		"/usr/bin/env php -r \"echo ini_get('extension_dir');\"",
	}).Return(bytes.NewBufferString("/some/path"), nil)

	solver.EXPECT().FromImage("docker.io/library/php:7.3-fpm-buster@sha256").Times(1)
	solver.EXPECT().ReadFile(
		gomock.Any(),
		"/etc/os-release",
		gomock.Any(),
	).Return(rawDebianOSRelease, nil)

	solver.EXPECT().FromContext(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(2)
	solver.EXPECT().ReadFile(
		gomock.Any(), "composer.lock", gomock.Any(),
	).AnyTimes().Return([]byte{}, statesolver.FileNotFound)

	pkgSolver := mocks.NewMockPackageSolver(mockCtrl)
	pkgSolver.EXPECT().ResolveVersions(
		gomock.Any(),
		"docker.io/library/php:7.3-fpm-buster@sha256",
		map[string]string{
			"git":         "*",
			"libicu-dev":  "*",
			"libssl-dev":  "*",
			"libxml2-dev": "*",
			"libzip-dev":  "*",
			"openssl":     "*",
			"unzip":       "*",
			"zlib1g-dev":  "*",
		},
	).AnyTimes().Return(map[string]string{
		"git":         "git-version",
		"libicu-dev":  "libicu-dev-version",
		"libssl-dev":  "libssl-dev-version",
		"libxml2-dev": "libxml2-dev-version",
		"libzip-dev":  "libzip-dev-version",
		"openssl":     "openssl-version",
		"unzip":       "unzip-version",
		"zlib1g-dev":  "zlib1g-dev-version",
	}, nil)

	pb := pecltest.NewMockBackend(mockCtrl)
	pb.EXPECT().
		ResolveConstraint(gomock.Any(), "yaml", "~1.0", peclapi.Beta).
		AnyTimes().
		Return("1.1.0", nil)
	pb.EXPECT().
		ResolveConstraint(gomock.Any(), "apcu", "*", peclapi.Stable).
		AnyTimes().
		Return("5.1.18", nil)
	pb.EXPECT().
		ResolveConstraint(gomock.Any(), "redis", "~5.1.0", peclapi.Stable).
		AnyTimes().
		Return("5.1.0", nil)

	h := php.NewPHPHandler()
	h.WithPeclBackend(pb)
	h.WithSolver(solver)

	return updateLocksTC{
		deffile: "testdata/locks/without-stages.yml",
		handler: h,
		pkgSolvers: pkgsolver.PackageSolversMap{
			pkgsolver.APT: func(statesolver.StateSolver) pkgsolver.PackageSolver {
				return pkgSolver
			},
		},
		expected: "testdata/locks/without-stages.lock",
	}
}

var rawAlpineOSRelease = []byte(`NAME="Alpine Linux"
ID=alpine
VERSION_ID=3.10.3
PRETTY_NAME="Alpine Linux v3.10"
HOME_URL="https://alpinelinux.org/"
BUG_REPORT_URL="https://bugs.alpinelinux.org/"
`)

func initUpdateLocksWithAlpineBaseImageTC(t *testing.T, mockCtrl *gomock.Controller) updateLocksTC {
	solver := mocks.NewMockStateSolver(mockCtrl)
	solver.EXPECT().ResolveImageRef(
		gomock.Any(), "docker.io/library/php:7.3-fpm-alpine",
	).Return("docker.io/library/php:7.3-fpm-alpine@sha256", nil)

	solver.EXPECT().ExecImage(gomock.Any(), "docker.io/library/php:7.3-fpm-alpine@sha256", []string{
		"/usr/bin/env php -r \"echo ini_get('extension_dir');\"",
	}).Return(bytes.NewBufferString("/some/path"), nil)

	solver.EXPECT().FromImage("docker.io/library/php:7.3-fpm-alpine@sha256").Times(1)
	solver.EXPECT().ReadFile(
		gomock.Any(),
		"/etc/os-release",
		gomock.Any(),
	).Return(rawAlpineOSRelease, nil)

	solver.EXPECT().FromContext(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(2)
	solver.EXPECT().ReadFile(
		gomock.Any(), "composer.lock", gomock.Any(),
	).AnyTimes().Return([]byte{}, statesolver.FileNotFound)

	pkgSolver := mocks.NewMockPackageSolver(mockCtrl)
	pkgSolver.EXPECT().ResolveVersions(
		gomock.Any(),
		"docker.io/library/php:7.3-fpm-alpine@sha256",
		map[string]string{
			"git":         "*",
			"icu-dev":     "*",
			"openssl-dev": "*",
			"libxml2-dev": "*",
			"libzip-dev":  "*",
			"unzip":       "*",
		},
	).AnyTimes().Return(map[string]string{
		"git":         "git-version",
		"icu-dev":     "icu-dev-version",
		"openssl-dev": "libssl-dev-version",
		"libxml2-dev": "libxml2-dev-version",
		"libzip-dev":  "libzip-dev-version",
		"unzip":       "unzip-version",
	}, nil)

	pb := pecltest.NewMockBackend(mockCtrl)
	pb.EXPECT().
		ResolveConstraint(gomock.Any(), "yaml", "~1.0", peclapi.Beta).
		AnyTimes().
		Return("1.1.0", nil)
	pb.EXPECT().
		ResolveConstraint(gomock.Any(), "apcu", "*", peclapi.Stable).
		AnyTimes().
		Return("5.1.18", nil)
	pb.EXPECT().
		ResolveConstraint(gomock.Any(), "redis", "~5.1.0", peclapi.Stable).
		AnyTimes().
		Return("5.1.0", nil)

	h := php.NewPHPHandler()
	h.WithPeclBackend(pb)
	h.WithSolver(solver)

	return updateLocksTC{
		deffile: "testdata/locks/with-alpine-base-image.yml",
		handler: h,
		pkgSolvers: pkgsolver.PackageSolversMap{
			pkgsolver.APK: func(statesolver.StateSolver) pkgsolver.PackageSolver {
				return pkgSolver
			},
		},
		expected: "testdata/locks/with-alpine-base-image.lock",
	}
}

func TestUpdateLocks(t *testing.T) {
	testcases := map[string]func(*testing.T, *gomock.Controller) updateLocksTC{
		"with an alpine base image": initUpdateLocksWithAlpineBaseImageTC,
		"with a debian base image":  initUpdateLocksWithDebianBaseImageTC,
	}

	for tcname := range testcases {
		tcinit := testcases[tcname]

		t.Run(tcname, func(t *testing.T) {
			t.Parallel()

			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			tc := tcinit(t, mockCtrl)
			genericDef := loadGenericDef(t, tc.deffile)

			var locks builddef.Locks
			var rawLocks []byte
			var err error

			ctx := context.Background()
			buildOpts := builddef.BuildOpts{
				Def: &genericDef,
			}

			locks, err = tc.handler.UpdateLocks(ctx, tc.pkgSolvers, buildOpts)
			if tc.expectedErr != nil {
				if err == nil || err.Error() != tc.expectedErr.Error() {
					t.Fatalf("Expected error: %v\nGot: %v", tc.expectedErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

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

			expectedRaw := loadTestdata(t, tc.expected)
			if expectedRaw != string(rawLocks) {
				tempfile := newTempFile(t)
				ioutil.WriteFile(tempfile, rawLocks, 0640) //nolint:errcheck

				t.Fatalf("Expected: <%s>\nGot: <%s>", tc.expected, tempfile)
			}
		})
	}
}

func loadGenericDef(t *testing.T, filepath string) builddef.BuildDef {
	raw := loadTestdata(t, filepath)

	var def builddef.BuildDef
	if err := yaml.Unmarshal([]byte(raw), &def); err != nil {
		t.Fatal(err)
	}

	return def
}

func loadDefLocks(t *testing.T, filepath string) builddef.RawLocks {
	raw := loadTestdata(t, filepath)

	var locks builddef.RawLocks
	if err := yaml.Unmarshal([]byte(raw), &locks); err != nil {
		t.Fatal(err)
	}

	return locks
}
