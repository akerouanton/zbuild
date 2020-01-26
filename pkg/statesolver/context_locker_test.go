package statesolver_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/NiR-/zbuild/pkg/builddef"
	"github.com/NiR-/zbuild/pkg/mocks"
	"github.com/NiR-/zbuild/pkg/statesolver"
	"github.com/go-test/deep"
	"github.com/golang/mock/gomock"
)

type lockContextTC struct {
	context     builddef.Context
	solver      statesolver.StateSolver
	expected    builddef.Context
	expectedErr error
}

func initLockGitBranchToASpecificReferenceTC(t *testing.T, mockCtrl *gomock.Controller) lockContextTC {
	outbuf := bytes.NewBufferString("6efe5ec4eeefbb601c31ff2b1f976e379500068a\n")

	solver := mocks.NewMockStateSolver(mockCtrl)
	solver.EXPECT().ExecImage(gomock.Any(), "docker.io/akerouanton/zbuild-git:v0.1", []string{
		"git clone --quiet git://github.com/NiR-/zbuild-testrepo /tmp/repo 1>/dev/null 2>&1",
		"cd /tmp/repo",
		"git rev-parse -q --verify 'some-branch'"}).Return(outbuf, nil)

	return lockContextTC{
		context: builddef.Context{
			Type:   builddef.ContextTypeGit,
			Source: "git://github.com/NiR-/zbuild-testrepo",
			GitContext: builddef.GitContext{
				Reference: "some-branch",
			},
		},
		solver: solver,
		expected: builddef.Context{
			Source: "git://github.com/NiR-/zbuild-testrepo",
			Type:   builddef.ContextTypeGit,
			GitContext: builddef.GitContext{
				Reference: "6efe5ec4eeefbb601c31ff2b1f976e379500068a",
			},
		},
	}
}

func TestLockContext(t *testing.T) {
	testcases := map[string]func(t *testing.T, mockCtrl *gomock.Controller) lockContextTC{
		"lock git branch to a specific reference": initLockGitBranchToASpecificReferenceTC,
	}

	for tcname := range testcases {
		tcinit := testcases[tcname]

		t.Run(tcname, func(t *testing.T) {
			t.Parallel()

			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			tc := tcinit(t, mockCtrl)
			ctx := context.Background()
			locked, err := statesolver.LockContext(ctx, tc.solver, &tc.context)

			if tc.expectedErr != nil {
				if err == nil || err.Error() != tc.expectedErr.Error() {
					t.Fatalf("Expected error: %v\nGot: %v", tc.expectedErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if diff := deep.Equal(*locked, tc.expected); diff != nil {
				t.Fatal(diff)
			}
		})
	}
}
