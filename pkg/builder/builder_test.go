package builder_test

import (
	"context"
	"errors"
	"fmt"
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
	testcases := map[string]func(*gomock.Controller) testCase{
		"successfully build default stage and file":                 successfullyBuildDefaultStageAndFileTC,
		"successfully build custom stage and file":                  successfullyBuildCustomStageAndFileTC,
		"successfully build from git context":                       successfullyBuildFromGitContextTC,
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

			tc := tcinit(mockCtrl)
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

var (
	zbuildYml = []byte(`
kind: php
version: 7.2.29

extensions:
  intl: "*"`)

	zbuildLock = []byte(`
system_packages:
  libicu-dev: "52.1-8+deb8u7"
extensions:
  intl: "*"`)
)

func successfullyBuildDefaultStageAndFileTC(mockCtrl *gomock.Controller) testCase {
	c := llbtest.NewMockClient(mockCtrl)
	c.EXPECT().BuildOpts().AnyTimes().Return(client.BuildOpts{
		SessionID: "<SESSION-ID>",
		Opts: map[string]string{
			"context": "some-context-name",
		},
	})

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
		File:        "zbuild.yml",
		LockFile:    "zbuild.lock",
		Stage:       "dev",
		SessionID:   "<SESSION-ID>",
		ContextName: "some-context-name",
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
	registry.Register("php", handler)

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

func successfullyBuildFromGitContextTC(mockCtrl *gomock.Controller) testCase {
	c := llbtest.NewMockClient(mockCtrl)
	c.EXPECT().BuildOpts().AnyTimes().Return(client.BuildOpts{
		SessionID: "<SESSION-ID>",
		Opts: map[string]string{
			"context": "git://github.com/some/repo",
		},
	})

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
		File:        "zbuild.yml",
		LockFile:    "zbuild.lock",
		Stage:       "dev",
		SessionID:   "<SESSION-ID>",
		ContextName: "git://github.com/some/repo",
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
	registry.Register("php", handler)

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

func successfullyBuildCustomStageAndFileTC(mockCtrl *gomock.Controller) testCase {
	c := llbtest.NewMockClient(mockCtrl)
	c.EXPECT().BuildOpts().AnyTimes().Return(client.BuildOpts{
		SessionID: "<SESSION-ID>",
		Opts: map[string]string{
			"filename": "api.zbuild.yml",
			"target":   "prod",
		},
	})

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
		File:        "api.zbuild.yml",
		LockFile:    "api.zbuild.lock",
		Stage:       "prod",
		SessionID:   "<SESSION-ID>",
		ContextName: "context",
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
	registry.Register("php", handler)

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

func failToReadYmlTC(mockCtrl *gomock.Controller) testCase {
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

func failToReadLockTC(mockCtrl *gomock.Controller) testCase {
	c := llbtest.NewMockClient(mockCtrl)
	c.EXPECT().BuildOpts().AnyTimes().Return(client.BuildOpts{
		SessionID: "<SESSION-ID>",
		Opts: map[string]string{
			"contextkey": "some-context-name",
		},
	})

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
		File:        "zbuild.yml",
		LockFile:    "zbuild.lock",
		Stage:       "dev",
		SessionID:   "<SESSION-ID>",
		ContextName: "some-context-name",
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
	registry.Register("php", handler)

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

func failToFindASutableKindHandlerTC(mockCtrl *gomock.Controller) testCase {
	c := llbtest.NewMockClient(mockCtrl)
	c.EXPECT().BuildOpts().AnyTimes().Return(client.BuildOpts{
		SessionID: "<SESSION-ID>",
		Opts: map[string]string{
			"context": "some-context-name",
		},
	})

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
	registry.Register("notphp", handler)

	return testCase{
		client:      c,
		solver:      solver,
		registry:    registry,
		expectedErr: errors.New("kind \"php\" is not supported: unknown kind"),
	}
}

func failWhenKindHandlerFailsTC(mockCtrl *gomock.Controller) testCase {
	c := llbtest.NewMockClient(mockCtrl)
	c.EXPECT().BuildOpts().AnyTimes().Return(client.BuildOpts{
		SessionID: "<SESSION-ID>",
		Opts: map[string]string{
			"context": "some-context-name",
		},
	})

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
	registry.Register("php", handler)

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
		opts.ContextName == m.opts.ContextName
}

func (m buildOptsMatcher) String() string {
	return fmt.Sprintf("{%s %s %s %s %s}",
		m.opts.File,
		m.opts.LockFile,
		m.opts.Stage,
		m.opts.SessionID,
		m.opts.ContextName)
}
