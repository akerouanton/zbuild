package nodejs_test

import (
	"context"
	"testing"

	"github.com/NiR-/zbuild/pkg/builddef"
	"github.com/NiR-/zbuild/pkg/defkinds/nodejs"
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
	handler       *nodejs.NodeJSHandler
	client        client.Client
	buildOpts     builddef.BuildOpts
	expectedState string
	expectedImage *image.Image
	expectedErr   error
}

func initBuildLLBForDevStageTC(t *testing.T, mockCtrl *gomock.Controller) buildTC {
	genericDef := loadBuildDef(t, "testdata/build/zbuild.yml")
	genericDef.RawLocks = loadDefLocks(t, "testdata/build/zbuild.lock")

	solver := mocks.NewMockStateSolver(mockCtrl)
	solver.EXPECT().
		FileExists(gomock.Any(), "package-lock.json", gomock.Any()).
		Return(false, nil)

	kindHandler := nodejs.NodeJSHandler{}
	kindHandler.WithSolver(solver)

	return buildTC{
		handler: &kindHandler,
		client:  llbtest.NewMockClient(mockCtrl),
		buildOpts: builddef.BuildOpts{
			Def:           genericDef,
			Stage:         "dev",
			SessionID:     "<SESSION-ID>",
			LocalUniqueID: "x1htr02606a9rk8b0daewh9es",
			BuildContext: &builddef.Context{
				Source: "context",
				Type:   builddef.ContextTypeLocal,
			},
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
						"NODE_ENV=development",
					},
					Entrypoint: []string{"docker-entrypoint.sh"},
					Cmd:        []string{"node"},
					Volumes:    map[string]struct{}{},
					WorkingDir: "/app",
					Labels: map[string]string{
						"io.zbuild": "true",
					},
				},
			},
		},
	}
}

func initBuildLLBForProdFrontendStageTC(t *testing.T, mockCtrl *gomock.Controller) buildTC {
	genericDef := loadBuildDef(t, "testdata/build/frontend.yml")
	genericDef.RawLocks = loadDefLocks(t, "testdata/build/frontend.lock")

	solver := mocks.NewMockStateSolver(mockCtrl)
	solver.EXPECT().
		FileExists(gomock.Any(), "package-lock.json", gomock.Any()).
		Return(false, nil)

	kindHandler := nodejs.NodeJSHandler{}
	kindHandler.WithSolver(solver)

	return buildTC{
		handler: &kindHandler,
		client:  llbtest.NewMockClient(mockCtrl),
		buildOpts: builddef.BuildOpts{
			Def:           genericDef,
			Stage:         "prod",
			SessionID:     "<SESSION-ID>",
			LocalUniqueID: "x1htr02606a9rk8b0daewh9es",
			BuildContext: &builddef.Context{
				Source: "context",
				Type:   builddef.ContextTypeLocal,
			},
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
						"NODE_ENV=production",
					},
					Entrypoint: []string{"docker-entrypoint.sh"},
					Cmd:        []string{"node"},
					Volumes:    map[string]struct{}{},
					WorkingDir: "/app",
					Labels: map[string]string{
						"io.zbuild": "true",
					},
				},
			},
		},
	}
}

func initBuildLLBForWorkerStageTC(t *testing.T, mockCtrl *gomock.Controller) buildTC {
	genericDef := loadBuildDef(t, "testdata/build/zbuild.yml")
	genericDef.RawLocks = loadDefLocks(t, "testdata/build/zbuild.lock")

	solver := mocks.NewMockStateSolver(mockCtrl)
	solver.EXPECT().
		FileExists(gomock.Any(), "package-lock.json", gomock.Any()).
		Return(false, nil)

	kindHandler := nodejs.NodeJSHandler{}
	kindHandler.WithSolver(solver)

	return buildTC{
		handler: &kindHandler,
		client:  llbtest.NewMockClient(mockCtrl),
		buildOpts: builddef.BuildOpts{
			Def:           genericDef,
			Stage:         "worker",
			SessionID:     "<SESSION-ID>",
			LocalUniqueID: "x1htr02606a9rk8b0daewh9es",
			BuildContext: &builddef.Context{
				Source: "context",
				Type:   builddef.ContextTypeLocal,
			},
		},
		expectedState: "testdata/build/state-worker.json",
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
						"NODE_ENV=production",
					},
					// Following entrypoint is automatically defined by the
					// base image. Maybe it should not be kept? :thinking: @TODO
					Entrypoint: []string{"docker-entrypoint.sh"},
					Cmd:        []string{"bin/worker.js"},
					Volumes:    map[string]struct{}{},
					WorkingDir: "/app",
					Labels: map[string]string{
						"io.zbuild": "true",
					},
				},
				Healthcheck: &image.HealthConfig{
					Test: []string{"NONE"},
				},
			},
		},
	}
}

func initBuildLLBWithGitBuildContextTC(t *testing.T, mockCtrl *gomock.Controller) buildTC {
	genericDef := loadBuildDef(t, "testdata/build/zbuild.yml")
	genericDef.RawLocks = loadDefLocks(t, "testdata/build/zbuild.lock")

	solver := mocks.NewMockStateSolver(mockCtrl)
	solver.EXPECT().
		FileExists(gomock.Any(), "/sub/dir/package-lock.json", gomock.Any()).
		Return(false, nil)

	kindHandler := nodejs.NodeJSHandler{}
	kindHandler.WithSolver(solver)

	return buildTC{
		handler: &kindHandler,
		client:  llbtest.NewMockClient(mockCtrl),
		buildOpts: builddef.BuildOpts{
			Def:           genericDef,
			Stage:         "prod",
			SessionID:     "<SESSION-ID>",
			LocalUniqueID: "x1htr02606a9rk8b0daewh9es",
			BuildContext: &builddef.Context{
				Source: "git://github.com/some/repo",
				Type:   builddef.ContextTypeGit,
				GitContext: builddef.GitContext{
					Path: "sub/dir",
				},
			},
		},
		expectedState: "testdata/build/state-prod-from-git-build-context.json",
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
						"NODE_ENV=production",
					},
					Entrypoint: []string{"docker-entrypoint.sh"},
					Cmd:        []string{"node"},
					Volumes:    map[string]struct{}{},
					WorkingDir: "/app",
					Labels: map[string]string{
						"io.zbuild": "true",
					},
				},
				Healthcheck: &image.HealthConfig{
					Test: []string{"NONE"},
				},
			},
		},
	}
}

func initBuildLLBWithGitSourceContextTC(t *testing.T, mockCtrl *gomock.Controller) buildTC {
	genericDef := loadBuildDef(t, "testdata/build/with-git-context.yml")
	genericDef.RawLocks = loadDefLocks(t, "testdata/build/with-git-context.lock")

	solver := mocks.NewMockStateSolver(mockCtrl)
	solver.EXPECT().
		FileExists(gomock.Any(), "/client/package-lock.json", gomock.Any()).
		Return(false, nil)

	kindHandler := nodejs.NodeJSHandler{}
	kindHandler.WithSolver(solver)

	return buildTC{
		handler: &kindHandler,
		client:  llbtest.NewMockClient(mockCtrl),
		buildOpts: builddef.BuildOpts{
			Def:           genericDef,
			Stage:         "prod",
			SessionID:     "<SESSION-ID>",
			LocalUniqueID: "x1htr02606a9rk8b0daewh9es",
			BuildContext: &builddef.Context{
				Source: "context",
				Type:   builddef.ContextTypeLocal,
			},
		},
		expectedState: "testdata/build/with-git-context.json",
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
						"NODE_ENV=production",
					},
					Entrypoint: []string{"docker-entrypoint.sh"},
					Cmd:        []string{"node"},
					Volumes:    map[string]struct{}{},
					WorkingDir: "/app",
					Labels: map[string]string{
						"io.zbuild": "true",
					},
				},
				Healthcheck: &image.HealthConfig{
					Test: []string{"NONE"},
				},
			},
		},
	}
}

func initBuildLLBWithGitBuildAndSourceContextTC(t *testing.T, mockCtrl *gomock.Controller) buildTC {
	genericDef := loadBuildDef(t, "testdata/build/with-git-context.yml")
	genericDef.RawLocks = loadDefLocks(t, "testdata/build/with-git-context.lock")

	solver := mocks.NewMockStateSolver(mockCtrl)
	solver.EXPECT().
		FileExists(gomock.Any(), "/client/package-lock.json", gomock.Any()).
		Return(false, nil)

	kindHandler := nodejs.NodeJSHandler{}
	kindHandler.WithSolver(solver)

	return buildTC{
		handler: &kindHandler,
		client:  llbtest.NewMockClient(mockCtrl),
		buildOpts: builddef.BuildOpts{
			Def:           genericDef,
			Stage:         "prod",
			SessionID:     "<SESSION-ID>",
			LocalUniqueID: "x1htr02606a9rk8b0daewh9es",
			BuildContext: &builddef.Context{
				Source: "git://github.com/some/repo",
				Type:   builddef.ContextTypeGit,
				GitContext: builddef.GitContext{
					Path: "sub/dir",
				},
			},
		},
		expectedState: "testdata/build/with-git-source-and-build-context.json",
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
						"NODE_ENV=production",
					},
					Entrypoint: []string{"docker-entrypoint.sh"},
					Cmd:        []string{"node"},
					Volumes:    map[string]struct{}{},
					WorkingDir: "/app",
					Labels: map[string]string{
						"io.zbuild": "true",
					},
				},
				Healthcheck: &image.HealthConfig{
					Test: []string{"NONE"},
				},
			},
		},
	}
}

func initBuildLLBForNpmBasedProjectTC(t *testing.T, mockCtrl *gomock.Controller) buildTC {
	genericDef := loadBuildDef(t, "testdata/build/frontend.yml")
	genericDef.RawLocks = loadDefLocks(t, "testdata/build/frontend.lock")

	solver := mocks.NewMockStateSolver(mockCtrl)
	solver.EXPECT().
		FileExists(gomock.Any(), "package-lock.json", gomock.Any()).
		Return(true, nil)

	kindHandler := nodejs.NodeJSHandler{}
	kindHandler.WithSolver(solver)

	return buildTC{
		handler: &kindHandler,
		client:  llbtest.NewMockClient(mockCtrl),
		buildOpts: builddef.BuildOpts{
			Def:           genericDef,
			Stage:         "prod",
			SessionID:     "<SESSION-ID>",
			LocalUniqueID: "x1htr02606a9rk8b0daewh9es",
			BuildContext: &builddef.Context{
				Source: "context",
				Type:   builddef.ContextTypeLocal,
			},
		},
		expectedState: "testdata/build/state-prod-with-npm.json",
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
						"NODE_ENV=production",
					},
					Entrypoint: []string{"docker-entrypoint.sh"},
					Cmd:        []string{"node"},
					Volumes:    map[string]struct{}{},
					WorkingDir: "/app",
					Labels: map[string]string{
						"io.zbuild": "true",
					},
				},
			},
		},
	}
}

func initBuildLLBForAlpineBasedBaseImageTC(t *testing.T, mockCtrl *gomock.Controller) buildTC {
	genericDef := loadBuildDef(t, "testdata/build/alpine.yml")
	genericDef.RawLocks = loadDefLocks(t, "testdata/build/alpine.lock")

	solver := mocks.NewMockStateSolver(mockCtrl)
	solver.EXPECT().
		FileExists(gomock.Any(), "package-lock.json", gomock.Any()).
		Return(false, nil)

	kindHandler := nodejs.NodeJSHandler{}
	kindHandler.WithSolver(solver)

	return buildTC{
		handler: &kindHandler,
		client:  llbtest.NewMockClient(mockCtrl),
		buildOpts: builddef.BuildOpts{
			Def:           genericDef,
			Stage:         "prod",
			SessionID:     "<SESSION-ID>",
			LocalUniqueID: "x1htr02606a9rk8b0daewh9es",
			BuildContext: &builddef.Context{
				Source: "context",
				Type:   builddef.ContextTypeLocal,
			},
		},
		expectedState: "testdata/build/state-alpine-prod.json",
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
						"NODE_ENV=production",
					},
					Entrypoint: []string{"docker-entrypoint.sh"},
					Cmd:        []string{"node"},
					Volumes:    map[string]struct{}{},
					WorkingDir: "/app",
					Labels: map[string]string{
						"io.zbuild": "true",
					},
				},
			},
		},
	}
}

func initBuildLLBForWorkerStageWithCacheMountsTC(t *testing.T, mockCtrl *gomock.Controller) buildTC {
	genericDef := loadBuildDef(t, "testdata/build/with-cache-mounts.yml")
	genericDef.RawLocks = loadDefLocks(t, "testdata/build/with-cache-mounts.lock")

	solver := mocks.NewMockStateSolver(mockCtrl)
	solver.EXPECT().
		FileExists(gomock.Any(), "package-lock.json", gomock.Any()).
		Return(false, nil)

	kindHandler := nodejs.NodeJSHandler{}
	kindHandler.WithSolver(solver)

	return buildTC{
		handler: &kindHandler,
		client:  llbtest.NewMockClient(mockCtrl),
		buildOpts: builddef.BuildOpts{
			Def:              genericDef,
			Stage:            "worker",
			SessionID:        "<SESSION-ID>",
			LocalUniqueID:    "x1htr02606a9rk8b0daewh9es",
			WithCacheMounts:  true,
			CacheIDNamespace: "cache-ns",
			BuildContext: &builddef.Context{
				Source: "context",
				Type:   builddef.ContextTypeLocal,
			},
		},
		expectedState: "testdata/build/state-worker-with-cache-mounts.json",
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
						"NODE_ENV=production",
					},
					// Following entrypoint is automatically defined by the
					// base image. Maybe it should not be kept? :thinking: @TODO
					Entrypoint: []string{"docker-entrypoint.sh"},
					Cmd:        []string{"bin/worker.js"},
					Volumes:    map[string]struct{}{},
					WorkingDir: "/app",
					Labels: map[string]string{
						"io.zbuild": "true",
					},
				},
				Healthcheck: &image.HealthConfig{
					Test: []string{"NONE"},
				},
			},
		},
	}
}

func TestBuild(t *testing.T) {
	testcases := map[string]func(*testing.T, *gomock.Controller) buildTC{
		"build LLB DAG for dev stage":                     initBuildLLBForDevStageTC,
		"build LLB DAG for worker stage":                  initBuildLLBForWorkerStageTC,
		"build LLB DAG for prod frontend stage":           initBuildLLBForProdFrontendStageTC,
		"build LLB DAG with git build context":            initBuildLLBWithGitBuildContextTC,
		"build LLB DAG with git source context":           initBuildLLBWithGitSourceContextTC,
		"build LLB DAG with git build and source context": initBuildLLBWithGitBuildAndSourceContextTC,
		"build LLB DAG for npm-based project":             initBuildLLBForNpmBasedProjectTC,
		"build LLB DAG for alpine-based base image":       initBuildLLBForAlpineBasedBaseImageTC,
		"build LLB DAG for prod stage with cache mounts":  initBuildLLBForWorkerStageWithCacheMountsTC,
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
					return
				}
				panic("This test has no expectedState defined.")
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

			expectedState := loadRawTestdata(t, tc.expectedState)
			if string(expectedState) != jsonState {
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
	handler     *nodejs.NodeJSHandler
	buildOpts   builddef.BuildOpts
	expected    string
	expectedErr error
}

func initDebugDevStageTC(t *testing.T, mockCtrl *gomock.Controller) debugConfigTC {
	solver := mocks.NewMockStateSolver(mockCtrl)

	h := &nodejs.NodeJSHandler{}
	h.WithSolver(solver)

	genericDef := loadBuildDef(t, "testdata/debug-config/zbuild.yml")
	genericDef.RawLocks = loadDefLocks(t, "testdata/debug-config/zbuild.lock")

	return debugConfigTC{
		handler: h,
		buildOpts: builddef.BuildOpts{
			Def:   genericDef,
			Stage: "dev",
		},
		expected: "testdata/debug-config/dump-dev.yml",
	}
}

func initDebugProdStageTC(t *testing.T, mockCtrl *gomock.Controller) debugConfigTC {
	solver := mocks.NewMockStateSolver(mockCtrl)

	h := &nodejs.NodeJSHandler{}
	h.WithSolver(solver)

	genericDef := loadBuildDef(t, "testdata/debug-config/zbuild.yml")
	genericDef.RawLocks = loadDefLocks(t, "testdata/debug-config/zbuild.lock")

	return debugConfigTC{
		handler: h,
		buildOpts: builddef.BuildOpts{
			Def:   genericDef,
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

			expected := loadRawTestdata(t, tc.expected)
			if string(expected) != string(raw) {
				tempfile := newTempFile(t)
				writeTestdata(t, tempfile, string(raw))

				t.Fatalf("Expected: <%s>\nGot: <%s>", tc.expected, tempfile)
			}
		})
	}
}
