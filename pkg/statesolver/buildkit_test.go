package statesolver_test

import (
	"context"
	"io/ioutil"
	"testing"

	"github.com/NiR-/zbuild/pkg/llbtest"
	"github.com/NiR-/zbuild/pkg/statesolver"
	"github.com/golang/mock/gomock"
	"github.com/moby/buildkit/frontend/gateway/client"
	"golang.org/x/xerrors"
)

type buildkitReadFileTC struct {
	solver      statesolver.BuildkitSolver
	opt         statesolver.ReadFileOpt
	filepath    string
	expected    string
	expectedErr error
}

func initFromBuildContextTC(t *testing.T, mockCtrl *gomock.Controller) buildkitReadFileTC {
	contextRef := llbtest.NewMockReference(mockCtrl)
	solved := &client.Result{
		Refs: map[string]client.Reference{
			"linux/amd64": contextRef,
		},
		Ref: contextRef,
	}

	c := llbtest.NewMockClient(mockCtrl)
	c.EXPECT().BuildOpts().AnyTimes().Return(client.BuildOpts{
		SessionID: "<SESSION-ID>",
		Opts: map[string]string{
			"contextkey": "some-context",
		},
	})
	c.EXPECT().Solve(
		gomock.Any(), gomock.Any(),
	).Return(solved, nil)

	raw := loadRawTestdata(t, "testdata/testfile")
	contextRef.EXPECT().ReadFile(gomock.Any(), client.ReadRequest{
		Filename: "testdata/testfile",
	}).Return([]byte(raw), nil)

	solver := statesolver.NewBuildkitSolver(c)
	opt := solver.FromBuildContext()

	return buildkitReadFileTC{
		solver:   solver,
		opt:      opt,
		filepath: "testdata/testfile",
		expected: "some file content",
	}
}

func initFailToReadFileFromBuildContextTC(t *testing.T, mockCtrl *gomock.Controller) buildkitReadFileTC {
	contextRef := llbtest.NewMockReference(mockCtrl)
	solved := &client.Result{
		Refs: map[string]client.Reference{
			"linux/amd64": contextRef,
		},
		Ref: contextRef,
	}

	c := llbtest.NewMockClient(mockCtrl)
	c.EXPECT().BuildOpts().AnyTimes().Return(client.BuildOpts{
		SessionID: "<SESSION-ID>",
		Opts:      map[string]string{},
	})
	c.EXPECT().Solve(
		gomock.Any(), gomock.Any(),
	).Return(solved, nil)

	contextRef.EXPECT().ReadFile(gomock.Any(), client.ReadRequest{
		Filename: "/foo",
	}).Return([]byte{}, xerrors.New("file does not exist"))

	solver := statesolver.NewBuildkitSolver(c)
	opt := solver.FromBuildContext()

	return buildkitReadFileTC{
		solver:      solver,
		opt:         opt,
		filepath:    "/foo",
		expectedErr: xerrors.Errorf("failed to read /foo from build context: %w", statesolver.FileNotFound),
	}
}

func initFromImageTC(t *testing.T, mockCtrl *gomock.Controller) buildkitReadFileTC {
	srcRef := llbtest.NewMockReference(mockCtrl)
	solved := &client.Result{
		Refs: map[string]client.Reference{
			"linux/amd64": srcRef,
		},
		Ref: srcRef,
	}

	c := llbtest.NewMockClient(mockCtrl)
	c.EXPECT().BuildOpts().AnyTimes().Return(client.BuildOpts{
		SessionID: "<SESSION-ID>",
		Opts:      map[string]string{},
	})
	// @TODO: Add a matcher to test the definition passed as argument
	c.EXPECT().Solve(
		gomock.Any(), gomock.Any(),
	).Return(solved, nil)

	raw := loadRawTestdata(t, "testdata/testfile")
	srcRef.EXPECT().ReadFile(gomock.Any(), client.ReadRequest{
		Filename: "/foo/bar",
	}).Return([]byte(raw), nil)

	solver := statesolver.NewBuildkitSolver(c)
	opt := solver.FromImage("docker.io/library/debian:buster")

	return buildkitReadFileTC{
		solver:   solver,
		opt:      opt,
		filepath: "/foo/bar",
		expected: "some file content",
	}
}

func initFailToReadNonexistantFileFromImageTC(t *testing.T, mockCtrl *gomock.Controller) buildkitReadFileTC {
	srcRef := llbtest.NewMockReference(mockCtrl)
	solved := &client.Result{
		Refs: map[string]client.Reference{
			"linux/amd64": srcRef,
		},
		Ref: srcRef,
	}

	c := llbtest.NewMockClient(mockCtrl)
	c.EXPECT().BuildOpts().AnyTimes().Return(client.BuildOpts{
		SessionID: "<SESSION-ID>",
		Opts:      map[string]string{},
	})
	c.EXPECT().Solve(
		gomock.Any(), gomock.Any(),
	).Return(solved, nil)

	srcRef.EXPECT().ReadFile(gomock.Any(), client.ReadRequest{
		Filename: "/foo",
	}).Return([]byte{}, xerrors.New("file does not exist"))

	solver := statesolver.NewBuildkitSolver(c)
	opt := solver.FromImage("docker.io/library/debian:buster")

	return buildkitReadFileTC{
		solver:      solver,
		opt:         opt,
		filepath:    "/foo",
		expectedErr: xerrors.Errorf("failed to read /foo from docker.io/library/debian:buster: file not found"),
	}
}

func initFailToReadFromNonexistantImageTC(t *testing.T, mockCtrl *gomock.Controller) buildkitReadFileTC {
	c := llbtest.NewMockClient(mockCtrl)
	c.EXPECT().BuildOpts().AnyTimes().Return(client.BuildOpts{
		SessionID: "<SESSION-ID>",
		Opts:      map[string]string{},
	})
	c.EXPECT().Solve(
		gomock.Any(), gomock.Any(),
	).Return(nil, xerrors.New("failed to solve state"))

	solver := statesolver.NewBuildkitSolver(c)
	opt := solver.FromImage("docker.io/library/debian:buster")

	return buildkitReadFileTC{
		solver:      solver,
		opt:         opt,
		filepath:    "/foo",
		expectedErr: xerrors.New("failed to read /foo from docker.io/library/debian:buster: failed to solve state"),
	}
}

func TestBuildkitReadFile(t *testing.T) {
	testcases := map[string]func(*testing.T, *gomock.Controller) buildkitReadFileTC{
		"from build context":                         initFromBuildContextTC,
		"fail to read file from build context":       initFailToReadFileFromBuildContextTC,
		"from image":                                 initFromImageTC,
		"fail to read a nonexistant file from image": initFailToReadNonexistantFileFromImageTC,
		"fail to read from nonexistant image":        initFailToReadFromNonexistantImageTC,
	}

	for tcname := range testcases {
		tcinit := testcases[tcname]
		t.Run(tcname, func(t *testing.T) {
			t.Parallel()

			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			tc := tcinit(t, mockCtrl)
			ctx := context.Background()

			raw, err := tc.solver.ReadFile(ctx, tc.filepath, tc.opt)
			if tc.expectedErr != nil {
				if err == nil || err.Error() != tc.expectedErr.Error() {
					t.Fatalf("Expected error: %v\nGot: %v", tc.expectedErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if string(raw) != tc.expected {
				t.Fatalf("Expected: %s\nGot: %s", string(raw), tc.expected)
			}
		})
	}
}

func loadRawTestdata(t *testing.T, filepath string) []byte {
	raw, err := ioutil.ReadFile(filepath)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
