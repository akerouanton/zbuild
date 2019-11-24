package pkgsolver_test

import (
	"errors"
	"io/ioutil"
	"os"
	"testing"

	"github.com/NiR-/webdf/pkg/builddef"
	"github.com/NiR-/webdf/pkg/pkgsolver"
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

			config, err := pkgsolver.GuessSolverConfig(tc.osrelease, "amd64")
			if err != nil {
				t.Fatal(err)
			}

			err = tc.solver.Configure(config)
			if tc.expectedErr == nil && err != nil {
				t.Fatalf("Expected no errors but got one: %v", err.Error())
			}
			if tc.expectedErr != nil && err == nil {
				t.Fatalf("Expected: %v\nGot: <nil>", tc.expectedErr.Error())
			}
			if tc.expectedErr != nil && tc.expectedErr.Error() != err.Error() {
				t.Fatalf("Expected: %v\nGot: %v", tc.expectedErr.Error(), err.Error())
			}
		})
	}
}
