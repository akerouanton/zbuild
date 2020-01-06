package php_test

import (
	"context"
	"io/ioutil"
	"testing"
	"time"

	"github.com/NiR-/zbuild/pkg/builddef"
	"github.com/NiR-/zbuild/pkg/defkinds/php"
	"github.com/NiR-/zbuild/pkg/image"
	"github.com/NiR-/zbuild/pkg/llbtest"
	"github.com/NiR-/zbuild/pkg/mocks"
	"github.com/go-test/deep"
	"github.com/golang/mock/gomock"
	"github.com/moby/buildkit/frontend/gateway/client"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"gopkg.in/yaml.v2"
)

type buildTC struct {
	handler       *php.PHPHandler
	client        client.Client
	buildOpts     builddef.BuildOpts
	expectedState string
	expectedImage *image.Image
	expectedErr   error
}

func initBuildLLBForDevStageTC(t *testing.T, mockCtrl *gomock.Controller) buildTC {
	genericDef := loadGenericDef(t, "testdata/build/zbuild.yml")
	genericDef.RawLocks = loadDefLocks(t, "testdata/build/zbuild.lock")

	solver := mocks.NewMockStateSolver(mockCtrl)

	raw := loadRawTestdata(t, "testdata/composer/composer-symfony4.4.lock")
	solver.EXPECT().FromBuildContext(gomock.Any()).Times(1)
	solver.EXPECT().ReadFile(
		gomock.Any(), "composer.lock", gomock.Any(),
	).Return(raw, nil)

	kindHandler := php.NewPHPHandler()
	kindHandler.WithSolver(solver)

	return buildTC{
		handler: kindHandler,
		client:  llbtest.NewMockClient(mockCtrl),
		buildOpts: builddef.BuildOpts{
			Def:           &genericDef,
			Stage:         "dev",
			SessionID:     "<SESSION-ID>",
			LocalUniqueID: "x1htr02606a9rk8b0daewh9es",
			ContextName:   "context",
		},
		expectedState: "testdata/build/state-dev.json",
		expectedImage: &image.Image{
			Image: specs.Image{
				Architecture: "amd64",
				OS:           "linux",
				RootFS: specs.RootFS{
					Type: "layers",
				},
			},
			Config: image.ImageConfig{
				ImageConfig: specs.ImageConfig{
					User: "1000",
					Env: []string{
						"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
						"COMPOSER_HOME=/composer",
						"PHP_VERSION=7.3.13",
						"PHP_INI_DIR=/usr/local/etc/php",
					},
					Entrypoint: []string{"docker-php-entrypoint"},
					Cmd:        []string{"php-fpm"},
					WorkingDir: "/app",
					StopSignal: "SIGQUIT",
					Volumes:    map[string]struct{}{},
					ExposedPorts: map[string]struct{}{
						"9000/tcp": {},
					},
					Labels: map[string]string{
						"io.zbuild": "true",
					},
				},
			},
		},
	}
}

func initBuildLLBForProdStageTC(t *testing.T, mockCtrl *gomock.Controller) buildTC {
	genericDef := loadGenericDef(t, "testdata/build/zbuild.yml")
	genericDef.RawLocks = loadDefLocks(t, "testdata/build/zbuild.lock")

	solver := mocks.NewMockStateSolver(mockCtrl)

	raw := loadRawTestdata(t, "testdata/composer/composer-symfony4.4.lock")
	solver.EXPECT().FromBuildContext(gomock.Any()).Times(1)
	solver.EXPECT().ReadFile(
		gomock.Any(), "composer.lock", gomock.Any(),
	).Return(raw, nil)

	kindHandler := php.NewPHPHandler()
	kindHandler.WithSolver(solver)

	return buildTC{
		handler: kindHandler,
		client:  llbtest.NewMockClient(mockCtrl),
		buildOpts: builddef.BuildOpts{
			Def:           &genericDef,
			Stage:         "prod",
			SessionID:     "<SESSION-ID>",
			LocalUniqueID: "x1htr02606a9rk8b0daewh9es",
			ContextName:   "context",
		},
		expectedState: "testdata/build/state-prod.json",
		expectedImage: &image.Image{
			Image: specs.Image{
				Architecture: "amd64",
				OS:           "linux",
				RootFS: specs.RootFS{
					Type: "layers",
				},
			},
			Config: image.ImageConfig{
				ImageConfig: specs.ImageConfig{
					User: "1000",
					Env: []string{
						"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
						"COMPOSER_HOME=/composer",
						"PHP_VERSION=7.3.13",
						"PHP_INI_DIR=/usr/local/etc/php",
					},
					Entrypoint: []string{"docker-php-entrypoint"},
					Cmd:        []string{"php-fpm"},
					WorkingDir: "/app",
					StopSignal: "SIGQUIT",
					Volumes:    map[string]struct{}{},
					ExposedPorts: map[string]struct{}{
						"9000/tcp": {},
					},
					Labels: map[string]string{
						"io.zbuild": "true",
					},
				},
				Healthcheck: &image.HealthConfig{
					Test:     []string{"CMD", "http_proxy= test \"$(fcgi-client get 127.0.0.1:9000 /_ping)\" = \"pong\""},
					Interval: 10 * time.Second,
					Timeout:  1 * time.Second,
					Retries:  3,
				},
			},
		},
	}
}

func initBuildProdStageFromGitBasedBuildContextTC(t *testing.T, mockCtrl *gomock.Controller) buildTC {
	genericDef := loadGenericDef(t, "testdata/build/zbuild.yml")
	genericDef.RawLocks = loadDefLocks(t, "testdata/build/zbuild.lock")

	solver := mocks.NewMockStateSolver(mockCtrl)

	raw := loadRawTestdata(t, "testdata/composer/composer-symfony4.4.lock")
	solver.EXPECT().FromBuildContext(gomock.Any()).Times(1)
	solver.EXPECT().ReadFile(
		gomock.Any(), "composer.lock", gomock.Any(),
	).Return(raw, nil)

	kindHandler := php.NewPHPHandler()
	kindHandler.WithSolver(solver)

	return buildTC{
		handler: kindHandler,
		client:  llbtest.NewMockClient(mockCtrl),
		buildOpts: builddef.BuildOpts{
			Def:           &genericDef,
			Stage:         "prod",
			SessionID:     "<SESSION-ID>",
			LocalUniqueID: "x1htr02606a9rk8b0daewh9es",
			ContextName:   "git://github.com/some/repo",
		},
		expectedState: "testdata/build/from-git-context.json",
		expectedImage: &image.Image{
			Image: specs.Image{
				Architecture: "amd64",
				OS:           "linux",
				RootFS: specs.RootFS{
					Type: "layers",
				},
			},
			Config: image.ImageConfig{
				ImageConfig: specs.ImageConfig{
					User: "1000",
					Env: []string{
						"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
						"COMPOSER_HOME=/composer",
						"PHP_VERSION=7.3.13",
						"PHP_INI_DIR=/usr/local/etc/php",
					},
					Entrypoint: []string{"docker-php-entrypoint"},
					Cmd:        []string{"php-fpm"},
					WorkingDir: "/app",
					StopSignal: "SIGQUIT",
					Volumes:    map[string]struct{}{},
					ExposedPorts: map[string]struct{}{
						"9000/tcp": {},
					},
					Labels: map[string]string{
						"io.zbuild": "true",
					},
				},
				Healthcheck: &image.HealthConfig{
					Test:     []string{"CMD", "http_proxy= test \"$(fcgi-client get 127.0.0.1:9000 /_ping)\" = \"pong\""},
					Interval: 10 * time.Second,
					Timeout:  1 * time.Second,
					Retries:  3,
				},
			},
		},
	}
}

func TestBuild(t *testing.T) {
	testcases := map[string]func(*testing.T, *gomock.Controller) buildTC{
		"build LLB DAG for dev stage":                   initBuildLLBForDevStageTC,
		"build LLB DAG for prod stage":                  initBuildLLBForProdStageTC,
		"build prod stage from git-based build context": initBuildProdStageFromGitBasedBuildContextTC,
	}

	for tcname := range testcases {
		tcinit := testcases[tcname]

		t.Run(tcname, func(t *testing.T) {
			t.Parallel()

			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			tc := tcinit(t, mockCtrl)
			ctx := context.TODO()

			state, img, err := tc.handler.Build(ctx, tc.buildOpts)
			jsonState := llbtest.StateToJSON(t, state)

			if *flagTestdata {
				if tc.expectedState != "" {
					writeTestdata(t, tc.expectedState, jsonState)
				}
			}

			if tc.expectedErr != nil {
				if err == nil || tc.expectedErr.Error() != err.Error() {
					t.Fatalf("Expected error: %v\nGot: %v", tc.expectedErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			expectedState := loadTestdata(t, tc.expectedState)
			if expectedState != jsonState {
				tempfile := newTempFile(t)
				writeTestdata(t, tempfile, jsonState)

				t.Fatalf("Expected: <%s>\nGot: <%s>", tc.expectedState, tempfile)
			}

			img.Created = nil
			img.History = nil
			img.RootFS.DiffIDs = nil
			if diff := deep.Equal(img, tc.expectedImage); diff != nil {
				t.Fatal(diff)
			}
		})
	}
}

type debugConfigTC struct {
	handler     *php.PHPHandler
	buildOpts   builddef.BuildOpts
	expected    string
	expectedErr error
}

func initDebugDevStageTC(t *testing.T, mockCtrl *gomock.Controller) debugConfigTC {
	solver := mocks.NewMockStateSolver(mockCtrl)

	raw := loadRawTestdata(t, "testdata/debug-config/composer.lock")
	solver.EXPECT().FromBuildContext(gomock.Any()).Times(1)
	solver.EXPECT().ReadFile(
		gomock.Any(), "composer.lock", gomock.Any(),
	).Return(raw, nil)

	h := php.NewPHPHandler()
	h.WithSolver(solver)

	genericDef := loadGenericDef(t, "testdata/debug-config/zbuild.yml")
	genericDef.RawLocks = loadDefLocks(t, "testdata/debug-config/zbuild.lock")

	return debugConfigTC{
		handler: h,
		buildOpts: builddef.BuildOpts{
			Def:   &genericDef,
			Stage: "dev",
		},
		expected: "testdata/debug-config/dump-dev.yml",
	}
}

func initDebugProdStageTC(t *testing.T, mockCtrl *gomock.Controller) debugConfigTC {
	solver := mocks.NewMockStateSolver(mockCtrl)

	raw := loadRawTestdata(t, "testdata/debug-config/composer.lock")
	solver.EXPECT().FromBuildContext(gomock.Any()).Times(1)
	solver.EXPECT().ReadFile(
		gomock.Any(), "composer.lock", gomock.Any(),
	).Return(raw, nil)

	h := php.NewPHPHandler()
	h.WithSolver(solver)

	genericDef := loadGenericDef(t, "testdata/debug-config/zbuild.yml")
	genericDef.RawLocks = loadDefLocks(t, "testdata/debug-config/zbuild.lock")

	return debugConfigTC{
		handler: h,
		buildOpts: builddef.BuildOpts{
			Def:   &genericDef,
			Stage: "prod",
		},
		expected: "testdata/debug-config/dump-prod.yml",
	}
}

func TestDebugConfig(t *testing.T) {
	testcases := map[string]func(*testing.T, *gomock.Controller) debugConfigTC{
		"debug dev stage config":  initDebugDevStageTC,
		"debug prod stage config": initDebugProdStageTC,
	}

	for tcname := range testcases {
		tcinit := testcases[tcname]

		t.Run(tcname, func(t *testing.T) {
			t.Parallel()

			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			tc := tcinit(t, mockCtrl)

			dump, err := tc.handler.DebugConfig(tc.buildOpts)
			if tc.expectedErr != nil {
				if err == nil || err.Error() != tc.expectedErr.Error() {
					t.Fatalf("Expected error: %v\nGot: %v", tc.expectedErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			raw, err := yaml.Marshal(dump)
			if err != nil {
				t.Fatal(err)
			}

			if *flagTestdata {
				writeTestdata(t, tc.expected, string(raw))
				return
			}

			expected := loadTestdata(t, tc.expected)
			if expected != string(raw) {
				tempfile := newTempFile(t)
				writeTestdata(t, tempfile, string(raw))

				t.Fatalf("Expected: <%s>\nGot: <%s>", tc.expected, tempfile)
			}
		})
	}
}

func newTempFile(t *testing.T) string {
	file, err := ioutil.TempFile("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	return file.Name()
}
