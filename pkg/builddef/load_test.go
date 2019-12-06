package builddef_test

import (
	"context"
	"errors"
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/NiR-/zbuild/pkg/builddef"
	"github.com/NiR-/zbuild/pkg/llbtest"
	"github.com/go-test/deep"
	"github.com/golang/mock/gomock"
	"github.com/moby/buildkit/frontend/gateway/client"
)

type loadFromContextTC struct {
	client      *llbtest.MockClient
	buildOpts   builddef.BuildOpts
	expectedDef *builddef.BuildDef
	expectedErr error
}

func itLoadsConfigAndLockFilesFromContextTC(
	t *testing.T,
	mockCtrl *gomock.Controller,
) loadFromContextTC {
	ctx := context.TODO()
	basedir := "testdata/config-files"

	c := llbtest.NewMockClient(mockCtrl)

	contextRef := llbtest.NewMockReference(mockCtrl)
	solvedContext := &client.Result{
		Refs: map[string]client.Reference{"linux/amd64": contextRef},
		Ref:  contextRef,
	}
	c.EXPECT().Solve(ctx, gomock.Any()).Return(solvedContext, nil)

	ymlPath := filepath.Join(basedir, "zbuild.yml")
	ymlContent := readTestdata(t, ymlPath)
	contextRef.EXPECT().ReadFile(ctx, client.ReadRequest{
		Filename: "zbuild.yml",
	}).Return(ymlContent, nil)

	lockPath := filepath.Join(basedir, "zbuild.lock")
	lockContent := readTestdata(t, lockPath)
	contextRef.EXPECT().ReadFile(ctx, client.ReadRequest{
		Filename: "zbuild.lock",
	}).Return(lockContent, nil)

	return loadFromContextTC{
		client: c,
		buildOpts: builddef.BuildOpts{
			File:     "zbuild.yml",
			LockFile: "zbuild.lock",
		},
		expectedDef: &builddef.BuildDef{
			Kind: "some-kind",
			RawConfig: map[string]interface{}{
				"foo": "bar",
			},
			RawLocks: []byte(`foo: bar
baz: plop
`),
		},
	}
}

func itLoadsConfigFileWithoutLockFromContextTC(
	t *testing.T,
	mockCtrl *gomock.Controller,
) loadFromContextTC {
	ctx := context.TODO()
	basedir := "testdata/without-lock"

	c := llbtest.NewMockClient(mockCtrl)

	contextRef := llbtest.NewMockReference(mockCtrl)
	solvedContext := &client.Result{
		Refs: map[string]client.Reference{"linux/amd64": contextRef},
		Ref:  contextRef,
	}
	c.EXPECT().Solve(ctx, gomock.Any()).Return(solvedContext, nil)

	ymlPath := filepath.Join(basedir, "zbuild.yml")
	ymlContent := readTestdata(t, ymlPath)
	contextRef.EXPECT().ReadFile(ctx, client.ReadRequest{
		Filename: "zbuild.yml",
	}).Return(ymlContent, nil)

	contextRef.EXPECT().ReadFile(ctx, client.ReadRequest{
		Filename: "zbuild.lock",
	}).Return([]byte{}, errors.New("file does not exist"))

	return loadFromContextTC{
		client: c,
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

func itFailsToLoadConfigFilesWhenTheresNoYmlFileFromContextTC(
	t *testing.T,
	mockCtrl *gomock.Controller,
) loadFromContextTC {
	ctx := context.TODO()

	c := llbtest.NewMockClient(mockCtrl)

	contextRef := llbtest.NewMockReference(mockCtrl)
	solvedContext := &client.Result{
		Refs: map[string]client.Reference{"linux/amd64": contextRef},
		Ref:  contextRef,
	}
	c.EXPECT().Solve(ctx, gomock.Any()).Return(solvedContext, nil)

	contextRef.EXPECT().ReadFile(ctx, client.ReadRequest{
		Filename: "zbuild.yml",
	}).Return([]byte{}, errors.New("file does not exist"))

	return loadFromContextTC{
		client: c,
		buildOpts: builddef.BuildOpts{
			File:     "zbuild.yml",
			LockFile: "zbuild.lock",
		},
		expectedErr: builddef.ZbuildfileNotFound,
	}
}

func itFailsToLoadConfigFilesWhenContextCannotBeResolvedTC(
	t *testing.T,
	mockCtrl *gomock.Controller,
) loadFromContextTC {
	ctx := context.TODO()

	c := llbtest.NewMockClient(mockCtrl)
	c.EXPECT().Solve(ctx, gomock.Any()).Return(nil, errors.New("some error"))

	return loadFromContextTC{
		client:      c,
		expectedErr: errors.New("failed to resolve build context: some error"),
	}
}

func TestLoadConfigFromBuildContext(t *testing.T) {
	testcases := map[string]func(*testing.T, *gomock.Controller) loadFromContextTC{
		"it loads config and lock files":                                itLoadsConfigAndLockFilesFromContextTC,
		"it loads config file without lock":                             itLoadsConfigFileWithoutLockFromContextTC,
		"it fails to load config files when there's no yml file":        itFailsToLoadConfigFilesWhenTheresNoYmlFileFromContextTC,
		"it fails to load config files when context cannot be resolved": itFailsToLoadConfigFilesWhenContextCannotBeResolvedTC,
	}

	for tcname := range testcases {
		tcinit := testcases[tcname]

		t.Run(tcname, func(t *testing.T) {
			t.Parallel()

			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			tc := tcinit(t, mockCtrl)
			ctx := context.TODO()

			buildDef, err := builddef.LoadFromContext(ctx, tc.client, tc.buildOpts)
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

func TestLoadFromFS(t *testing.T) {
	testcases := map[string]struct {
		basedir     string
		file        string
		lockFile    string
		expectedDef *builddef.BuildDef
		expectedErr error
	}{
		"it loads config and lock files": {
			file:     "testdata/config-files/zbuild.yml",
			lockFile: "testdata/config-files/zbuild.lock",
			expectedDef: &builddef.BuildDef{
				Kind: "some-kind",
				RawConfig: map[string]interface{}{
					"foo": "bar",
				},
				RawLocks: []byte(`foo: bar
baz: plop
`),
			},
		},
		"it loads config file without lock": {
			file:     "testdata/without-lock/zbuild.yml",
			lockFile: "testdata/without-lock/zbuild.lock",
			expectedDef: &builddef.BuildDef{
				Kind: "some-kind",
				RawConfig: map[string]interface{}{
					"bar": "baz",
				},
			},
		},
		"it fails to load config files when there's no yml file": {
			file:        "testdata/does-not-exist/zbuild.yml",
			lockFile:    "testdata/does-not-exist/zbuild.lock",
			expectedErr: errors.New("could not load testdata/does-not-exist/zbuild.yml: zbuildfile not found"),
		},
	}

	for tcname := range testcases {
		tc := testcases[tcname]

		t.Run(tcname, func(t *testing.T) {
			t.Parallel()

			out, err := builddef.LoadFromFS(tc.file, tc.lockFile)
			if tc.expectedErr != nil {
				if err == nil || err.Error() != tc.expectedErr.Error() {
					t.Fatalf("Expected error: %v\nGot: %v", tc.expectedErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if diff := deep.Equal(out, tc.expectedDef); diff != nil {
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
