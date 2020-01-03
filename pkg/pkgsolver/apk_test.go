package pkgsolver_test

import (
	"errors"
	"testing"

	"github.com/NiR-/zbuild/pkg/pkgsolver"
	"github.com/NiR-/zbuild/pkg/statesolver"
)

func TestAPKResolveVersions(t *testing.T) {
	testcases := map[string]struct {
		imageRef    string
		toResolve   map[string]string
		expectedErr error
	}{
		"successfully resolve package versions": {
			imageRef:  "docker.io/library/alpine:latest",
			toResolve: map[string]string{"curl": "*"},
		},
		"fail to resolve version of unknown package": {
			imageRef:    "docker.io/library/alpine:latest",
			toResolve:   map[string]string{"yolo": "*"},
			expectedErr: errors.New("packages yolo not found"),
		},
	}

	c := newDockerClient(t)

	for tcname := range testcases {
		tc := testcases[tcname]

		t.Run(tcname, func(t *testing.T) {
			t.Parallel()

			solver := statesolver.DockerSolver{
				Client:  c,
				Labels:  map[string]string{},
				RootDir: "testdata",
			}
			pkgSolver := pkgsolver.NewAPKSolver(solver)
			_, err := pkgSolver.ResolveVersions(tc.imageRef, tc.toResolve)

			if tc.expectedErr != nil {
				if err == nil || err.Error() != tc.expectedErr.Error() {
					t.Fatalf("Expected: %v\nGot: %v", tc.expectedErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
		})
	}
}
