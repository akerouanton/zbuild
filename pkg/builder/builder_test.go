package builder_test

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"testing"

	"github.com/NiR-/zbuild/pkg/builddef"
	"github.com/NiR-/zbuild/pkg/builder"
	"github.com/NiR-/zbuild/pkg/image"
	"github.com/NiR-/zbuild/pkg/llbtest"
	"github.com/NiR-/zbuild/pkg/mocks"
	"github.com/NiR-/zbuild/pkg/registry"
	"github.com/NiR-/zbuild/pkg/statesolver"
	"github.com/go-test/deep"
	"github.com/golang/mock/gomock"
	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/frontend/gateway/client"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"golang.org/x/xerrors"
)

type testCase struct {
	client      client.Client
	solver      statesolver.StateSolver
	registry    *registry.KindRegistry
	expectedErr error
	expectedRes *client.Result
}

func TestBuilder(t *testing.T) {
	testcases := map[string]func(*testing.T, *gomock.Controller) testCase{
		"build default stage and file":                              initBuildDefaultStageAndFileTC,
		"build custom stage and file":                               initBuildCustomStageAndFileTC,
		"build from git context":                                    initBuildFromGitContextTC,
		"build webserver stage":                                     initBuildWebserverStageTC,
		"fail to read zbuild.yml file":                              failToReadYmlTC,
		"failing to read zbuild.lock file doesn't prevent building": failToReadLockTC,
		"fail to find a suitable kind handler":                      failToFindASutableKindHandlerTC,
		"fail when kind handler fails":                              failWhenKindHandlerFailsTC,
	}

	for tcname := range testcases {
		tcinit := testcases[tcname]

		t.Run(tcname, func(t *testing.T) {
			t.Parallel()

			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			tc := tcinit(t, mockCtrl)
			b := builder.Builder{
				Registry: tc.registry,
			}

			outRes, outErr := b.Build(context.TODO(), tc.solver, tc.client)

			if tc.expectedErr != nil {
				if outErr == nil || outErr.Error() != tc.expectedErr.Error() {
					t.Fatalf("Expected error: %v\nGot: %v", tc.expectedErr.Error(), outErr.Error())
				}
				return
			}
			if outErr != nil {
				t.Fatalf("Unexpected error: %v", outErr)
			}
			if diff := deep.Equal(outRes, tc.expectedRes); diff != nil {
				t.Logf("expected metadata: %s", tc.expectedRes.Metadata)
				t.Logf("actual metadata: %s", outRes.Metadata)
				t.Fatal(diff)
			}
		})
	}
}

func initBuildDefaultStageAndFileTC(t *testing.T, mockCtrl *gomock.Controller) testCase {
	c := llbtest.NewMockClient(mockCtrl)
	c.EXPECT().BuildOpts().AnyTimes().Return(client.BuildOpts{
		SessionID: "<SESSION-ID>",
		Opts: map[string]string{
			"context": "some-context-name",
		},
	})

	zbuildYml := loadRawTestdata(t, "testdata/zbuild.yml")
	zbuildLock := loadRawTestdata(t, "testdata/zbuild.lock")

	solver := mocks.NewMockStateSolver(mockCtrl)
	solver.EXPECT().FromBuildContext(gomock.Any()).Times(1)

	solver.EXPECT().ReadFile(
		gomock.Any(), "zbuild.yml", gomock.Any(),
	).Return(zbuildYml, nil)

	solver.EXPECT().ReadFile(
		gomock.Any(), "zbuild.lock", gomock.Any(),
	).Return(zbuildLock, nil)

	ctx := context.TODO()
	buildOpts := builddef.BuildOpts{
		File:          "zbuild.yml",
		LockFile:      "zbuild.lock",
		Stage:         "dev",
		SessionID:     "<SESSION-ID>",
		SourceContext: "some-context-name",
		ConfigContext: "some-context-name",
	}
	state := llb.State{}
	img := image.Image{
		Image: specs.Image{
			Author: "zbuild",
		},
	}
	handler := mocks.NewMockKindHandler(mockCtrl)
	handler.EXPECT().WithSolver(gomock.Any()).Times(1)
	handler.EXPECT().Build(
		ctx, MatchBuildOpts(buildOpts),
	).Return(state, &img, nil)

	registry := registry.NewKindRegistry()
	registry.Register("php", handler, false)

	refImage := llbtest.NewMockReference(mockCtrl)
	resImg := &client.Result{
		Refs: map[string]client.Reference{"linux/amd64": refImage},
		Ref:  refImage,
	}
	c.EXPECT().Solve(gomock.Any(), gomock.Any()).Return(resImg, nil)

	imgConfig := `{"author":"zbuild","architecture":"","os":"","rootfs":{"type":"","diff_ids":null},"config":{}}`
	return testCase{
		client:   c,
		solver:   solver,
		registry: registry,
		expectedRes: &client.Result{
			Refs: map[string]client.Reference{"linux/amd64": refImage},
			Ref:  refImage,
			Metadata: map[string][]byte{
				"containerimage.config": []byte(imgConfig),
			},
		},
	}
}

func initBuildFromGitContextTC(t *testing.T, mockCtrl *gomock.Controller) testCase {
	c := llbtest.NewMockClient(mockCtrl)
	c.EXPECT().BuildOpts().AnyTimes().Return(client.BuildOpts{
		SessionID: "<SESSION-ID>",
		Opts: map[string]string{
			"context":                  "git://github.com/some/repo",
			"build-arg:config-context": "git://github.com/some/repo",
		},
	})

	zbuildYml := loadRawTestdata(t, "testdata/zbuild.yml")
	zbuildLock := loadRawTestdata(t, "testdata/zbuild.lock")

	solver := mocks.NewMockStateSolver(mockCtrl)
	solver.EXPECT().FromBuildContext(gomock.Any()).Times(1)

	solver.EXPECT().ReadFile(
		gomock.Any(), "zbuild.yml", gomock.Any(),
	).Return(zbuildYml, nil)

	solver.EXPECT().ReadFile(
		gomock.Any(), "zbuild.lock", gomock.Any(),
	).Return(zbuildLock, nil)

	ctx := context.TODO()
	buildOpts := builddef.BuildOpts{
		// @TODO: define the root dir from the context as the base path of the zbuildfile
		File:          "zbuild.yml",
		LockFile:      "zbuild.lock",
		Stage:         "dev",
		SessionID:     "<SESSION-ID>",
		SourceContext: "git://github.com/some/repo",
		ConfigContext: "git://github.com/some/repo",
	}
	state := llb.State{}
	img := image.Image{
		Image: specs.Image{
			Author: "zbuild",
		},
	}
	handler := mocks.NewMockKindHandler(mockCtrl)
	handler.EXPECT().WithSolver(gomock.Any()).Times(1)
	handler.EXPECT().Build(
		ctx, MatchBuildOpts(buildOpts),
	).Return(state, &img, nil)

	registry := registry.NewKindRegistry()
	registry.Register("php", handler, false)

	refImage := llbtest.NewMockReference(mockCtrl)
	resImg := &client.Result{
		Refs: map[string]client.Reference{"linux/amd64": refImage},
		Ref:  refImage,
	}
	c.EXPECT().Solve(gomock.Any(), gomock.Any()).Return(resImg, nil)

	imgConfig := `{"author":"zbuild","architecture":"","os":"","rootfs":{"type":"","diff_ids":null},"config":{}}`
	return testCase{
		client:   c,
		solver:   solver,
		registry: registry,
		expectedRes: &client.Result{
			Refs: map[string]client.Reference{"linux/amd64": refImage},
			Ref:  refImage,
			Metadata: map[string][]byte{
				"containerimage.config": []byte(imgConfig),
			},
		},
	}
}

func initBuildCustomStageAndFileTC(t *testing.T, mockCtrl *gomock.Controller) testCase {
	c := llbtest.NewMockClient(mockCtrl)
	c.EXPECT().BuildOpts().AnyTimes().Return(client.BuildOpts{
		SessionID: "<SESSION-ID>",
		Opts: map[string]string{
			"filename":                 "api.zbuild.yml",
			"target":                   "prod",
			"build-arg:config-context": "some-context-name",
		},
	})

	zbuildYml := loadRawTestdata(t, "testdata/zbuild.yml")
	zbuildLock := loadRawTestdata(t, "testdata/zbuild.lock")

	solver := mocks.NewMockStateSolver(mockCtrl)
	solver.EXPECT().FromBuildContext(gomock.Any()).Times(1)

	solver.EXPECT().ReadFile(
		gomock.Any(), "api.zbuild.yml", gomock.Any(),
	).Return(zbuildYml, nil)

	solver.EXPECT().ReadFile(
		gomock.Any(), "api.zbuild.lock", gomock.Any(),
	).Return(zbuildLock, nil)

	refImage := llbtest.NewMockReference(mockCtrl)
	resImg := &client.Result{
		Refs: map[string]client.Reference{"linux/amd64": refImage},
		Ref:  refImage,
	}
	c.EXPECT().Solve(gomock.Any(), gomock.Any()).Return(resImg, nil)

	ctx := context.TODO()
	buildOpts := builddef.BuildOpts{
		File:          "api.zbuild.yml",
		LockFile:      "api.zbuild.lock",
		Stage:         "prod",
		SessionID:     "<SESSION-ID>",
		SourceContext: "context",
		ConfigContext: "some-context-name",
	}
	state := llb.State{}
	img := image.Image{
		Image: specs.Image{
			Author: "zbuild",
		},
	}
	handler := mocks.NewMockKindHandler(mockCtrl)
	handler.EXPECT().WithSolver(gomock.Any()).Times(1)
	handler.EXPECT().Build(
		ctx, MatchBuildOpts(buildOpts),
	).Return(state, &img, nil)

	registry := registry.NewKindRegistry()
	registry.Register("php", handler, false)

	imgConfig := `{"author":"zbuild","architecture":"","os":"","rootfs":{"type":"","diff_ids":null},"config":{}}`
	return testCase{
		client:   c,
		solver:   solver,
		registry: registry,
		expectedRes: &client.Result{
			Refs: map[string]client.Reference{"linux/amd64": refImage},
			Ref:  refImage,
			Metadata: map[string][]byte{
				"containerimage.config": []byte(imgConfig),
			},
		},
	}
}

func initBuildWebserverStageTC(t *testing.T, mockCtrl *gomock.Controller) testCase {
	c := llbtest.NewMockClient(mockCtrl)
	c.EXPECT().BuildOpts().AnyTimes().Return(client.BuildOpts{
		SessionID: "<SESSION-ID>",
		Opts: map[string]string{
			"filename": "api.zbuild.yml",
			"target":   "webserver-prod",
		},
	})

	zbuildYml := loadRawTestdata(t, "testdata/zbuild.yml")
	zbuildLock := loadRawTestdata(t, "testdata/zbuild.lock")

	solver := mocks.NewMockStateSolver(mockCtrl)
	solver.EXPECT().FromBuildContext(gomock.Any()).Times(1)

	solver.EXPECT().ReadFile(
		gomock.Any(), "api.zbuild.yml", gomock.Any(),
	).Return(zbuildYml, nil)

	solver.EXPECT().ReadFile(
		gomock.Any(), "api.zbuild.lock", gomock.Any(),
	).Return(zbuildLock, nil)

	refImage := llbtest.NewMockReference(mockCtrl)
	resImg := &client.Result{
		Refs: map[string]client.Reference{"linux/amd64": refImage},
		Ref:  refImage,
	}
	c.EXPECT().Solve(gomock.Any(), gomock.Any()).Return(resImg, nil)

	ctx := context.TODO()
	state := llb.State{}
	img := image.Image{Image: specs.Image{Author: "zbuild"}}
	phpHandler := mocks.NewMockKindHandler(mockCtrl)
	phpHandler.EXPECT().WithSolver(gomock.Any()).Times(1)
	phpHandler.EXPECT().Build(ctx,
		MatchBuildOpts(builddef.BuildOpts{
			File:          "api.zbuild.yml",
			LockFile:      "api.zbuild.lock",
			Stage:         "prod",
			SessionID:     "<SESSION-ID>",
			SourceContext: "context",
			ConfigContext: "context",
		}),
	).Return(state, &img, nil)

	webHandler := mocks.NewMockKindHandler(mockCtrl)
	webHandler.EXPECT().WithSolver(gomock.Any()).Times(1)
	webHandler.EXPECT().Build(ctx,
		MatchBuildOpts(builddef.BuildOpts{
			File:          "api.zbuild.yml",
			LockFile:      "api.zbuild.lock",
			Stage:         "webserver",
			SessionID:     "<SESSION-ID>",
			SourceContext: "context",
			ConfigContext: "context",
			SourceState:   &llb.State{},
		}),
	).Return(state, &img, nil)

	registry := registry.NewKindRegistry()
	registry.Register("php", phpHandler, true)
	registry.Register("webserver", webHandler, false)

	imgConfig := `{"author":"zbuild","architecture":"","os":"","rootfs":{"type":"","diff_ids":null},"config":{}}`
	return testCase{
		client:   c,
		solver:   solver,
		registry: registry,
		expectedRes: &client.Result{
			Refs: map[string]client.Reference{"linux/amd64": refImage},
			Ref:  refImage,
			Metadata: map[string][]byte{
				"containerimage.config": []byte(imgConfig),
			},
		},
	}
}

func failToReadYmlTC(t *testing.T, mockCtrl *gomock.Controller) testCase {
	c := llbtest.NewMockClient(mockCtrl)
	c.EXPECT().BuildOpts().AnyTimes().Return(client.BuildOpts{
		SessionID: "<SESSION-ID>",
		Opts:      map[string]string{},
	})

	solver := mocks.NewMockStateSolver(mockCtrl)
	solver.EXPECT().FromBuildContext(gomock.Any()).Times(1)

	solver.EXPECT().ReadFile(
		gomock.Any(), "zbuild.yml", gomock.Any(),
	).Return([]byte{}, statesolver.FileNotFound)

	return testCase{
		client:      c,
		solver:      solver,
		registry:    registry.NewKindRegistry(),
		expectedErr: errors.New("zbuildfile not found"),
	}
}

func failToReadLockTC(t *testing.T, mockCtrl *gomock.Controller) testCase {
	c := llbtest.NewMockClient(mockCtrl)
	c.EXPECT().BuildOpts().AnyTimes().Return(client.BuildOpts{
		SessionID: "<SESSION-ID>",
		Opts: map[string]string{
			"contextkey": "some-context-name",
		},
	})

	zbuildYml := loadRawTestdata(t, "testdata/zbuild.yml")

	solver := mocks.NewMockStateSolver(mockCtrl)
	solver.EXPECT().FromBuildContext(gomock.Any()).Times(1)

	solver.EXPECT().ReadFile(
		gomock.Any(), "zbuild.yml", gomock.Any(),
	).Return(zbuildYml, nil)

	solver.EXPECT().ReadFile(
		gomock.Any(), "zbuild.lock", gomock.Any(),
	).Return([]byte{}, statesolver.FileNotFound)

	ctx := context.TODO()
	buildOpts := builddef.BuildOpts{
		File:          "zbuild.yml",
		LockFile:      "zbuild.lock",
		Stage:         "dev",
		SessionID:     "<SESSION-ID>",
		SourceContext: "some-context-name",
		ConfigContext: "some-context-name",
	}
	state := llb.State{}
	img := image.Image{
		Image: specs.Image{
			Author: "zbuild",
		},
	}
	handler := mocks.NewMockKindHandler(mockCtrl)
	handler.EXPECT().WithSolver(gomock.Any()).Times(1)
	handler.EXPECT().Build(
		ctx, MatchBuildOpts(buildOpts),
	).Return(state, &img, nil)

	registry := registry.NewKindRegistry()
	registry.Register("php", handler, false)

	refImage := llbtest.NewMockReference(mockCtrl)
	resImg := &client.Result{
		Refs: map[string]client.Reference{"linux/amd64": refImage},
		Ref:  refImage,
	}
	c.EXPECT().Solve(gomock.Any(), gomock.Any()).Return(resImg, nil)

	imgConfig := `{"author":"zbuild","architecture":"","os":"","rootfs":{"type":"","diff_ids":null},"config":{}}`
	return testCase{
		client:   c,
		solver:   solver,
		registry: registry,
		expectedRes: &client.Result{
			Refs: map[string]client.Reference{"linux/amd64": refImage},
			Ref:  refImage,
			Metadata: map[string][]byte{
				"containerimage.config": []byte(imgConfig),
			},
		},
	}
}

func failToFindASutableKindHandlerTC(t *testing.T, mockCtrl *gomock.Controller) testCase {
	c := llbtest.NewMockClient(mockCtrl)
	c.EXPECT().BuildOpts().AnyTimes().Return(client.BuildOpts{
		SessionID: "<SESSION-ID>",
		Opts: map[string]string{
			"context": "some-context-name",
		},
	})

	zbuildYml := loadRawTestdata(t, "testdata/zbuild.yml")
	zbuildLock := loadRawTestdata(t, "testdata/zbuild.lock")

	solver := mocks.NewMockStateSolver(mockCtrl)
	solver.EXPECT().FromBuildContext(gomock.Any()).Times(1)

	solver.EXPECT().ReadFile(
		gomock.Any(), "zbuild.yml", gomock.Any(),
	).Return(zbuildYml, nil)

	solver.EXPECT().ReadFile(
		gomock.Any(), "zbuild.lock", gomock.Any(),
	).Return(zbuildLock, nil)

	handler := mocks.NewMockKindHandler(mockCtrl)
	registry := registry.NewKindRegistry()
	registry.Register("notphp", handler, false)

	return testCase{
		client:      c,
		solver:      solver,
		registry:    registry,
		expectedErr: errors.New("kind \"php\" is not supported: unknown kind"),
	}
}

func failWhenKindHandlerFailsTC(t *testing.T, mockCtrl *gomock.Controller) testCase {
	c := llbtest.NewMockClient(mockCtrl)
	c.EXPECT().BuildOpts().AnyTimes().Return(client.BuildOpts{
		SessionID: "<SESSION-ID>",
		Opts: map[string]string{
			"context": "some-context-name",
		},
	})

	zbuildYml := loadRawTestdata(t, "testdata/zbuild.yml")
	zbuildLock := loadRawTestdata(t, "testdata/zbuild.lock")

	solver := mocks.NewMockStateSolver(mockCtrl)
	solver.EXPECT().FromBuildContext(gomock.Any()).Times(1)

	solver.EXPECT().ReadFile(
		gomock.Any(), "zbuild.yml", gomock.Any(),
	).Return(zbuildYml, nil)

	solver.EXPECT().ReadFile(
		gomock.Any(), "zbuild.lock", gomock.Any(),
	).Return(zbuildLock, nil)

	handler := mocks.NewMockKindHandler(mockCtrl)
	handler.EXPECT().WithSolver(gomock.Any()).Times(1)

	state := llb.State{}
	img := image.Image{}
	err := xerrors.New("some build error")
	handler.EXPECT().Build(gomock.Any(), gomock.Any()).Return(state, &img, err)

	registry := registry.NewKindRegistry()
	registry.Register("php", handler, false)

	return testCase{
		client:      c,
		solver:      solver,
		registry:    registry,
		expectedErr: errors.New("some build error"),
	}
}

func MatchBuildOpts(expected builddef.BuildOpts) buildOptsMatcher {
	return buildOptsMatcher{expected}
}

type buildOptsMatcher struct {
	opts builddef.BuildOpts
}

func (m buildOptsMatcher) Matches(x interface{}) bool {
	opts, ok := x.(builddef.BuildOpts)
	if !ok {
		return false
	}
	return opts.SessionID == m.opts.SessionID &&
		opts.File == m.opts.File &&
		opts.LockFile == m.opts.LockFile &&
		opts.Stage == m.opts.Stage &&
		opts.SourceContext == m.opts.SourceContext &&
		opts.ConfigContext == m.opts.ConfigContext
}

func (m buildOptsMatcher) String() string {
	return fmt.Sprintf("{%s %s %s %s %s %s}",
		m.opts.File,
		m.opts.LockFile,
		m.opts.Stage,
		m.opts.SessionID,
		m.opts.SourceContext,
		m.opts.ConfigContext)
}

func loadRawTestdata(t *testing.T, filepath string) []byte {
	buf, err := ioutil.ReadFile(filepath)
	if err != nil {
		t.Fatal(err)
	}
	return buf
}
