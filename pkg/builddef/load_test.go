package builddef_test

import (
	"context"
	"errors"
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/NiR-/webdf/pkg/builddef"
	"github.com/NiR-/webdf/pkg/llbtest"
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

	ymlPath := filepath.Join(basedir, "webdf.yml")
	ymlContent := readTestdata(t, ymlPath)
	contextRef.EXPECT().ReadFile(ctx, client.ReadRequest{
		Filename: "webdf.yml",
	}).Return(ymlContent, nil)

	lockPath := filepath.Join(basedir, "webdf.lock")
	lockContent := readTestdata(t, lockPath)
	contextRef.EXPECT().ReadFile(ctx, client.ReadRequest{
		Filename: "webdf.lock",
	}).Return(lockContent, nil)

	return loadFromContextTC{
		client: c,
		buildOpts: builddef.BuildOpts{
			File:     "webdf.yml",
			LockFile: "webdf.lock",
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

	ymlPath := filepath.Join(basedir, "webdf.yml")
	ymlContent := readTestdata(t, ymlPath)
	contextRef.EXPECT().ReadFile(ctx, client.ReadRequest{
		Filename: "webdf.yml",
	}).Return(ymlContent, nil)

	contextRef.EXPECT().ReadFile(ctx, client.ReadRequest{
		Filename: "webdf.lock",
	}).Return([]byte{}, errors.New("file does not exist"))

	return loadFromContextTC{
		client: c,
		buildOpts: builddef.BuildOpts{
			File:     "webdf.yml",
			LockFile: "webdf.lock",
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
		Filename: "webdf.yml",
	}).Return([]byte{}, errors.New("file does not exist"))

	return loadFromContextTC{
		client: c,
		buildOpts: builddef.BuildOpts{
			File:     "webdf.yml",
			LockFile: "webdf.lock",
		},
		expectedErr: builddef.ConfigYMLNotFound,
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
		expectedErr: errors.New("failed to resolve build context: failed to execute solve request: some error"),
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
			if tc.expectedErr == nil && err != nil {
				t.Fatalf("No error expected but got one: %v\n", err)
			}
			if tc.expectedErr != nil && err.Error() != tc.expectedErr.Error() {
				t.Fatalf("Expected error: %v\nGot: %v\n", tc.expectedErr, err)
			}
			if diff := deep.Equal(tc.expectedDef, buildDef); diff != nil {
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
			file:     "testdata/config-files/webdf.yml",
			lockFile: "testdata/config-files/webdf.lock",
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
			file:     "testdata/without-lock/webdf.yml",
			lockFile: "testdata/without-lock/webdf.lock",
			expectedDef: &builddef.BuildDef{
				Kind: "some-kind",
				RawConfig: map[string]interface{}{
					"bar": "baz",
				},
			},
		},
		"it fails to load config files when there's no yml file": {
			file:        "testdata/does-not-exist/webdf.yml",
			lockFile:    "testdata/does-not-exist/webdf.lock",
			expectedErr: errors.New("webdf.yml not found in build context"),
		},
	}

	for tcname := range testcases {
		tc := testcases[tcname]

		t.Run(tcname, func(t *testing.T) {
			t.Parallel()

			out, err := builddef.LoadFromFS(tc.file, tc.lockFile)
			if tc.expectedErr != nil && err.Error() != tc.expectedErr.Error() {
				t.Errorf("Expected error: %v\nGot: %v\n", tc.expectedErr, err)
			}
			if diff := deep.Equal(tc.expectedDef, out); diff != nil {
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
