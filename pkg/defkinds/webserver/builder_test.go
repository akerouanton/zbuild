package webserver_test

import (
	"context"
	"flag"
	"io/ioutil"
	"testing"
	"time"

	"github.com/NiR-/zbuild/pkg/builddef"
	"github.com/NiR-/zbuild/pkg/defkinds/webserver"
	"github.com/NiR-/zbuild/pkg/image"
	"github.com/NiR-/zbuild/pkg/llbtest"
	"github.com/NiR-/zbuild/pkg/mocks"
	"github.com/go-test/deep"
	"github.com/golang/mock/gomock"
	"github.com/moby/buildkit/frontend/gateway/client"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"golang.org/x/xerrors"
)

var flagTestdata = flag.Bool("testdata", false, "Use this flag to (re)generate testdata (dumps of LLB states)")

type buildTC struct {
	handler       *webserver.WebserverHandler
	client        client.Client
	buildOpts     builddef.BuildOpts
	expectedState string
	expectedImage *image.Image
	expectedErr   error
}

func initBuildLLBTC(t *testing.T, mockCtrl *gomock.Controller) buildTC {
	genericDef := loadGenericDef(t, "testdata/build/zbuild.yml", "testdata/build/zbuild.lock")

	solver := mocks.NewMockStateSolver(mockCtrl)
	kindHandler := webserver.WebserverHandler{}
	kindHandler.WithSolver(solver)

	return buildTC{
		handler: &kindHandler,
		client:  llbtest.NewMockClient(mockCtrl),
		buildOpts: builddef.BuildOpts{
			Def:           &genericDef,
			Stage:         "",
			SessionID:     "<SESSION-ID>",
			LocalUniqueID: "x1htr02606a9rk8b0daewh9es",
			ContextName:   "context",
		},
		expectedState: "testdata/build/state.json",
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
						"NGINX_VERSION=1.17.6",
						"NJS_VERSION=0.3.7",
						"PKG_RELEASE=1~buster",
					},
					Entrypoint: []string{},
					Cmd:        []string{"nginx", "-g", "daemon off;"},
					StopSignal: "SIGSTOP",
					Volumes:    map[string]struct{}{},
					ExposedPorts: map[string]struct{}{
						"80/tcp": {},
					},
					Labels: map[string]string{
						"io.zbuild":  "true",
						"maintainer": "NGINX Docker Maintainers <docker-maint@nginx.com>",
					},
				},
				Healthcheck: &image.HealthConfig{
					Test:     []string{"CMD", "http_proxy= test \"$(curl --fail http://127.0.0.1/_ping)\" = \"pong\""},
					Interval: 10 * time.Second,
					Timeout:  1 * time.Second,
					Retries:  3,
				},
			},
		},
	}
}

func initBuildLLBFromGitContextTC(t *testing.T, mockCtrl *gomock.Controller) buildTC {
	genericDef := loadGenericDef(t, "testdata/build/zbuild.yml", "testdata/build/zbuild.lock")

	solver := mocks.NewMockStateSolver(mockCtrl)
	kindHandler := webserver.WebserverHandler{}
	kindHandler.WithSolver(solver)

	return buildTC{
		handler: &kindHandler,
		client:  llbtest.NewMockClient(mockCtrl),
		buildOpts: builddef.BuildOpts{
			Def:           &genericDef,
			Stage:         "",
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
						"NGINX_VERSION=1.17.6",
						"NJS_VERSION=0.3.7",
						"PKG_RELEASE=1~buster",
					},
					Entrypoint: []string{},
					Cmd:        []string{"nginx", "-g", "daemon off;"},
					StopSignal: "SIGSTOP",
					Volumes:    map[string]struct{}{},
					ExposedPorts: map[string]struct{}{
						"80/tcp": {},
					},
					Labels: map[string]string{
						"io.zbuild":  "true",
						"maintainer": "NGINX Docker Maintainers <docker-maint@nginx.com>",
					},
				},
				Healthcheck: &image.HealthConfig{
					Test:     []string{"CMD", "http_proxy= test \"$(curl --fail http://127.0.0.1/_ping)\" = \"pong\""},
					Interval: 10 * time.Second,
					Timeout:  1 * time.Second,
					Retries:  3,
				},
			},
		},
	}
}

func initFailToBuildWithAssetsWhenNoSourceInTheBuildOptsTC(t *testing.T, mockCtrl *gomock.Controller) buildTC {
	genericDef := loadGenericDef(t, "testdata/build/with-assets.yml", "testdata/build/with-assets.lock")

	solver := mocks.NewMockStateSolver(mockCtrl)
	kindHandler := webserver.WebserverHandler{}
	kindHandler.WithSolver(solver)

	return buildTC{
		handler: &kindHandler,
		client:  llbtest.NewMockClient(mockCtrl),
		buildOpts: builddef.BuildOpts{
			Def:           &genericDef,
			Stage:         "",
			SessionID:     "<SESSION-ID>",
			LocalUniqueID: "x1htr02606a9rk8b0daewh9es",
			ContextName:   "context",
			Source:        nil,
		},
		expectedErr: xerrors.New("no source state to copy assets from has been provided"),
	}
}

func TestBuild(t *testing.T) {
	testcases := map[string]func(*testing.T, *gomock.Controller) buildTC{
		"build LLB":                              initBuildLLBTC,
		"build LLB from git-based build context": initBuildLLBFromGitContextTC,
		"fail to build with assets but without source in build opts": initFailToBuildWithAssetsWhenNoSourceInTheBuildOptsTC,
	}

	for tcname := range testcases {
		tcinit := testcases[tcname]

		t.Run(tcname, func(t *testing.T) {
			t.Parallel()

			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			tc := tcinit(t, mockCtrl)
			ctx := context.Background()

			state, img, err := tc.handler.Build(ctx, tc.buildOpts)
			jsonState := llbtest.StateToJSON(t, state)

			if *flagTestdata {
				if tc.expectedState != "" {
					writeTestdata(t, tc.expectedState, jsonState)
				}
			}

			if tc.expectedErr != nil {
				if err == nil || err.Error() != tc.expectedErr.Error() {
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

func writeTestdata(t *testing.T, filepath string, content string) {
	err := ioutil.WriteFile(filepath, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Could not write %q: %v", filepath, err)
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
