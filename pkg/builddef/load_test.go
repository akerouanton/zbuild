package builddef_test

import (
	"context"
	"io/ioutil"
	"testing"

	"github.com/NiR-/zbuild/pkg/builddef"
	"github.com/NiR-/zbuild/pkg/mocks"
	"github.com/NiR-/zbuild/pkg/statesolver"
	"github.com/go-test/deep"
	"github.com/golang/mock/gomock"
)

type loadTC struct {
	solver      statesolver.StateSolver
	buildOpts   builddef.BuildOpts
	expectedDef *builddef.BuildDef
	expectedErr error
}

func itLoadsConfigAndLockFilesTC(
	t *testing.T,
	mockCtrl *gomock.Controller,
) loadTC {
	solver := mocks.NewMockStateSolver(mockCtrl)
	ymlContent := readTestdata(t, "testdata/config-files/zbuild.yml")

	solver.EXPECT().FromBuildContext(gomock.Any()).Times(1)
	solver.EXPECT().ReadFile(
		gomock.Any(), "zbuild.yml", gomock.Any(),
	).Return(ymlContent, nil)

	lockContent := readTestdata(t, "testdata/config-files/zbuild.lock")
	solver.EXPECT().ReadFile(
		gomock.Any(), "zbuild.lock", gomock.Any(),
	).Return(lockContent, nil)

	return loadTC{
		solver: solver,
		buildOpts: builddef.BuildOpts{
			File:     "zbuild.yml",
			LockFile: "zbuild.lock",
		},
		expectedDef: &builddef.BuildDef{
			Kind: "some-kind",
			RawConfig: map[string]interface{}{
				"foo": "bar",
			},
			RawLocks: map[string]interface{}{
				"foo": "bar",
				"baz": "plop",
			},
		},
	}
}

func itLoadsConfigFileWithoutLockTC(
	t *testing.T,
	mockCtrl *gomock.Controller,
) loadTC {
	solver := mocks.NewMockStateSolver(mockCtrl)
	solver.EXPECT().FromBuildContext(gomock.Any()).Times(1)

	ymlContent := readTestdata(t, "testdata/without-lock/zbuild.yml")
	solver.EXPECT().ReadFile(
		gomock.Any(),
		"zbuild.yml",
		gomock.Any(),
	).Return(ymlContent, nil)

	solver.EXPECT().ReadFile(
		gomock.Any(), "zbuild.lock", gomock.Any(),
	).Return([]byte{}, statesolver.FileNotFound)

	return loadTC{
		solver: solver,
		buildOpts: builddef.BuildOpts{
			File:     "zbuild.yml",
			LockFile: "zbuild.lock",
		},
		expectedDef: &builddef.BuildDef{
			Kind: "some-kind",
			RawConfig: map[string]interface{}{
				"bar": "baz",
			},
			RawLocks: nil,
		},
	}
}

func itFailsToLoadConfigFilesWhenTheresNoYmlFileTC(
	t *testing.T,
	mockCtrl *gomock.Controller,
) loadTC {
	solver := mocks.NewMockStateSolver(mockCtrl)
	solver.EXPECT().FromBuildContext(gomock.Any()).Times(1)

	solver.EXPECT().ReadFile(
		gomock.Any(),
		"zbuild.yml",
		gomock.Any(),
	).Return([]byte{}, statesolver.FileNotFound)

	return loadTC{
		solver: solver,
		buildOpts: builddef.BuildOpts{
			File:     "zbuild.yml",
			LockFile: "zbuild.lock",
		},
		expectedErr: builddef.ZbuildfileNotFound,
	}
}

func TestLoadConfig(t *testing.T) {
	testcases := map[string]func(*testing.T, *gomock.Controller) loadTC{
		"it loads config and lock files":                         itLoadsConfigAndLockFilesTC,
		"it loads config file without lock":                      itLoadsConfigFileWithoutLockTC,
		"it fails to load config files when there's no yml file": itFailsToLoadConfigFilesWhenTheresNoYmlFileTC,
	}

	for tcname := range testcases {
		tcinit := testcases[tcname]

		t.Run(tcname, func(t *testing.T) {
			t.Parallel()

			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			tc := tcinit(t, mockCtrl)
			ctx := context.TODO()

			buildDef, err := builddef.Load(
				ctx, tc.solver, tc.buildOpts,
			)
			if tc.expectedErr != nil {
				if err == nil || err.Error() != tc.expectedErr.Error() {
					t.Fatalf("Expected error: %v\nGot: %v", tc.expectedErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if diff := deep.Equal(buildDef, tc.expectedDef); diff != nil {
				t.Error(diff)
			}
		})
	}
}

func readTestdata(t *testing.T, filepath string) []byte {
	content, err := ioutil.ReadFile(filepath)
	if err != nil {
		t.Fatalf("could not load %q: %v", filepath, err)
	}
	return content
}
