package config_test

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/NiR-/webdf/pkg/config"
	"github.com/NiR-/webdf/pkg/llbtest"
	"github.com/go-test/deep"
	"github.com/golang/mock/gomock"
	"github.com/moby/buildkit/frontend/gateway/client"
)

type loadConfigTC struct {
	name        string
	basedir     string
	expected    *config.Config
	expectedErr error
}

var (
	loadConfigTCs = []loadConfigTC{
		{
			name:    "it loads config and lock files",
			basedir: "testdata/config-files",
			expected: &config.Config{
				Services: []config.Service{
					{
						Name: "service-name",
						Type: "some-type",
						RawConfig: map[string]interface{}{
							"foo": "bar",
						},
					},
				},
				LockConfig: map[string]interface{}{
					"foo": "foo",
					"bar": "bar",
				},
			},
			expectedErr: nil,
		},
		{
			name:    "it loads config file without lock",
			basedir: "testdata/without-lock",
			expected: &config.Config{
				Services: []config.Service{
					{
						Name: "service-name",
						Type: "some-type",
						RawConfig: map[string]interface{}{
							"bar": "baz",
						},
					},
				},
				LockConfig: nil,
			},
			expectedErr: nil,
		},
		{
			name:        "it fails to load config files when there's no yml file",
			basedir:     "testdata/missing-config-files",
			expected:    nil,
			expectedErr: config.ConfigYMLNotFound,
		},
	}
)

func TestLocalConfigFromBuildContext(t *testing.T) {
	for tid := range loadConfigTCs {
		tc := loadConfigTCs[tid]

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			srcRef := llbtest.NewMockReference(mockCtrl)
			res := &client.Result{
				Refs: map[string]client.Reference{"linux/amd64": srcRef},
				Ref:  srcRef,
			}

			ctx := context.TODO()
			c := llbtest.NewMockClient(mockCtrl)
			c.EXPECT().BuildOpts().Return(client.BuildOpts{
				SessionID: "build-session-id",
			})
			c.EXPECT().Solve(ctx, gomock.Any()).Return(res, nil)

			webdfContent, webdfOK := readTestdata(t, filepath.Join(tc.basedir, "webdf.yml"))
			readWebdfCall := srcRef.EXPECT().ReadFile(ctx, client.ReadRequest{
				Filename: "webdf.yml",
			})
			if !webdfOK {
				readWebdfCall.Return([]byte{}, os.ErrNotExist)
			}
			if webdfOK {
				readWebdfCall.Return(webdfContent, nil)

				lockContent, lockOK := readTestdata(t, filepath.Join(tc.basedir, "webdf.lock"))
				readLockCall := srcRef.EXPECT().ReadFile(ctx, client.ReadRequest{
					Filename: "webdf.lock",
				})
				if lockOK {
					readLockCall.Return(lockContent, nil)
				} else {
					readLockCall.Return([]byte{}, os.ErrNotExist)
				}
			}

			config, err := config.LoadFromContext(ctx, c)
			if tc.expectedErr == nil && err != nil {
				t.Fatalf("No error expected but got one: %v\n", err)
			}
			if tc.expectedErr != nil && err.Error() != tc.expectedErr.Error() {
				t.Fatalf("Expected error: %v\nGot: %v\n", tc.expectedErr, err)
			}
			if diff := deep.Equal(tc.expected, config); diff != nil {
				t.Error(diff)
			}
		})
	}
}

func TestLoadFromFS(t *testing.T) {
	for tid := range loadConfigTCs {
		tc := loadConfigTCs[tid]

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			out, err := config.LoadFromFS(tc.basedir)
			if tc.expectedErr != nil && err.Error() != tc.expectedErr.Error() {
				t.Errorf("Expected error: %v\nGot: %v\n", tc.expectedErr, err)
			}
			if diff := deep.Equal(tc.expected, out); diff != nil {
				t.Error(diff)
			}
		})
	}
}

func readTestdata(t *testing.T, filepath string) ([]byte, bool) {
	content, err := ioutil.ReadFile(filepath)
	if os.IsNotExist(err) {
		return []byte{}, false
	}
	if err != nil {
		t.Fatalf("could not load %q: %v", filepath, err)
	}
	return content, true
}
