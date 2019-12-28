package pkgsolver_test

import (
	"errors"
	"io/ioutil"
	"os"
	"testing"

	"github.com/NiR-/zbuild/pkg/builddef"
	"github.com/NiR-/zbuild/pkg/pkgsolver"
	"github.com/snyh/go-dpkg-parser"
)

type configureSolverTC struct {
	solver      pkgsolver.PackageSolver
	osrelease   builddef.OSRelease
	expectedErr error
	cleanup     func()
}

func initSuccessfullyConfigureDpkgSolverTC(t *testing.T) configureSolverTC {
	tempdir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatal(err)
	}

	dpkgRepo := dpkg.NewRepository(tempdir)
	solver := pkgsolver.NewDpkgSolver(dpkgRepo)
	osrelease := builddef.OSRelease{
		VersionName: "buster",
	}

	return configureSolverTC{
		solver:      solver,
		osrelease:   osrelease,
		expectedErr: nil,
		cleanup: func() {
			os.RemoveAll(tempdir)
		},
	}
}

func initFailToConfigureDpkgSolverTC(t *testing.T) configureSolverTC {
	tempdir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatal(err)
	}

	dpkgRepo := dpkg.NewRepository(tempdir)
	solver := pkgsolver.NewDpkgSolver(dpkgRepo)
	osrelease := builddef.OSRelease{
		VersionName: "plop",
	}

	return configureSolverTC{
		solver:      solver,
		osrelease:   osrelease,
		expectedErr: errors.New(`could not add suite plop: can't download "http://deb.debian.org/debian/dists/plop/Release" : 404 Not Found`),
		cleanup: func() {
			os.RemoveAll(tempdir)
		},
	}
}

func TestConfigureSolver(t *testing.T) {
	testcases := map[string]func(*testing.T) configureSolverTC{
		"successfully add dpkg suite": initSuccessfullyConfigureDpkgSolverTC,
		"fail to add dpkg suite":      initFailToConfigureDpkgSolverTC,
	}

	for tcname := range testcases {
		tcinit := testcases[tcname]

		t.Run(tcname, func(t *testing.T) {
			t.Parallel()

			tc := tcinit(t)
			defer tc.cleanup()

			err := tc.solver.Configure(tc.osrelease, "amd64")
			if tc.expectedErr != nil {
				if err == nil || err.Error() != tc.expectedErr.Error() {
					t.Fatalf("Expected: %v\nGot: %v", tc.expectedErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Unexpected error: %v", err.Error())
			}
		})
	}
}
