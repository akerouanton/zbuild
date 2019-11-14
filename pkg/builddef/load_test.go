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
	c.EXPECT().BuildOpts().Return(client.BuildOpts{
		SessionID: "sessid",
	})

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
		expectedDef: &builddef.BuildDef{
			ConfigFilename: "webdf.yml",
			LockFilename:   "webdf.lock",
			Type:           "some-type",
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

func itLoadsConfigFileWithoutLockFromContextTC(
	t *testing.T,
	mockCtrl *gomock.Controller,
) loadFromContextTC {
	ctx := context.TODO()
	basedir := "testdata/without-lock"

	c := llbtest.NewMockClient(mockCtrl)
	c.EXPECT().BuildOpts().Return(client.BuildOpts{
		SessionID: "sessid",
	})

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
		expectedDef: &builddef.BuildDef{
			ConfigFilename: "webdf.yml",
			LockFilename:   "webdf.lock",
			Type:           "some-type",
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
	c.EXPECT().BuildOpts().Return(client.BuildOpts{
		SessionID: "sessid",
	})

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
		client:      c,
		expectedErr: builddef.ConfigYMLNotFound,
	}
}

func itFailsToLoadConfigFilesWhenContextCannotBeResolvedTC(
	t *testing.T,
	mockCtrl *gomock.Controller,
) loadFromContextTC {
	ctx := context.TODO()

	c := llbtest.NewMockClient(mockCtrl)
	c.EXPECT().BuildOpts().Return(client.BuildOpts{
		SessionID: "sessid",
	})

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

			config, err := builddef.LoadFromContext(ctx, tc.client)
			if tc.expectedErr == nil && err != nil {
				t.Fatalf("No error expected but got one: %v\n", err)
			}
			if tc.expectedErr != nil && err.Error() != tc.expectedErr.Error() {
				t.Fatalf("Expected error: %v\nGot: %v\n", tc.expectedErr, err)
			}
			if diff := deep.Equal(tc.expectedDef, config); diff != nil {
				t.Error(diff)
			}
		})
	}
}

func TestLoadFromFS(t *testing.T) {
	testcases := map[string]struct {
		basedir     string
		expectedDef *builddef.BuildDef
		expectedErr error
	}{
		"it loads config and lock files": {
			basedir: "testdata/config-files",
			expectedDef: &builddef.BuildDef{
				ConfigFilename: "webdf.yml",
				LockFilename:   "webdf.lock",
				Type:           "some-type",
				RawConfig: map[string]interface{}{
					"foo": "bar",
				},
				RawLocks: map[string]interface{}{
					"foo": "bar",
					"baz": "plop",
				},
			},
		},
		"it loads config file without lock": {
			basedir: "testdata/without-lock",
			expectedDef: &builddef.BuildDef{
				ConfigFilename: "webdf.yml",
				LockFilename:   "webdf.lock",
				Type:           "some-type",
				RawConfig: map[string]interface{}{
					"bar": "baz",
				},
			},
		},
		"it fails to load config files when there's no yml file": {
			basedir:     "testdata/does-not-exist",
			expectedErr: errors.New("webdf.yml not found in build context"),
		},
	}

	for tcname := range testcases {
		tc := testcases[tcname]

		t.Run(tcname, func(t *testing.T) {
			t.Parallel()

			out, err := builddef.LoadFromFS(tc.basedir)
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
