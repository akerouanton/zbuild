package pkgsolver_test

import (
	"errors"
	"io/ioutil"
	"os"
	"testing"

	"github.com/NiR-/webdf/pkg/pkgsolver"
	"github.com/snyh/go-dpkg-parser"
)

func TestDpkgResolveVersions(t *testing.T) {
	testcases := map[string]struct {
		toResolve   map[string]string
		arch        string
		expectedErr error
	}{
		"successfully resolve package versions": {
			toResolve:   map[string]string{"curl": "*"},
			arch:        "amd64",
			expectedErr: nil,
		},
		"fail to resolve version of unknown package": {
			toResolve:   map[string]string{"yolo": "*"},
			arch:        "amd64",
			expectedErr: errors.New("Not Found resource of yolo"),
		},
		"fail to resolve curl package when arch is invalid": {
			toResolve:   map[string]string{"curl": "*"},
			arch:        "yolo",
			expectedErr: errors.New("Not Found resource of curl"),
		},
	}

	for tcname := range testcases {
		tc := testcases[tcname]

		t.Run(tcname, func(t *testing.T) {
			t.Parallel()

			tempdir, err := ioutil.TempDir("", "")
			if err != nil {
				t.Fatal(err)
			}

			cleanup := func() {
				os.RemoveAll(tempdir)
			}

			dpkgRepo := dpkg.NewRepository(tempdir)
			repoURL := "http://deb.debian.org/debian"
			suiteName := "buster"
			if err := dpkgRepo.AddSuite(repoURL, suiteName, ""); err != nil {
				cleanup()
				t.Fatal(err)
			}

			pkgSolver := pkgsolver.NewDpkgSolver(dpkgRepo)
			pkgSolver.Configure(pkgsolver.SolverConfig{
				Arch:       tc.arch,
				DpkgSuites: [][]string{},
			})
			_, err = pkgSolver.ResolveVersions(tc.toResolve)

			if tc.expectedErr == nil && err != nil {
				t.Fatalf("Expected no errors but got one: %v", err.Error())
			}
			if tc.expectedErr != nil && err == nil {
				t.Fatalf("Expected error: %v\nGot: <nil>", tc.expectedErr.Error())
			}
			if tc.expectedErr != nil && tc.expectedErr.Error() != err.Error() {
				t.Fatalf("Expected: %v\nGot: %v", tc.expectedErr.Error(), err.Error())
			}
		})
	}
}
