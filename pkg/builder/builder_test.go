package builder_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/NiR-/webdf/pkg/builddef"
	"github.com/NiR-/webdf/pkg/builder"
	"github.com/NiR-/webdf/pkg/image"
	"github.com/NiR-/webdf/pkg/llbtest"
	"github.com/NiR-/webdf/pkg/mocks"
	"github.com/NiR-/webdf/pkg/registry"
	"github.com/go-test/deep"
	"github.com/golang/mock/gomock"
	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/frontend/gateway/client"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"golang.org/x/xerrors"
)

type testCase struct {
	client      client.Client
	registry    *registry.KindRegistry
	expectedErr error
	expectedRes *client.Result
}

func TestBuilder(t *testing.T) {
	testcases := map[string]func(*gomock.Controller) testCase{
		"successfully build default stage and file":       successfullyBuildDefaultStageAndFileTC,
		"successfully build custom stage and file":        successfullyBuildCustomStageAndFileTC,
		"fail to resolve build context":                   failToResolveBuildContextTC,
		"fail to read webdf.yml file":                     failToReadYmlTC,
		"fail to read webdf.lock file":                    failToReadLockTC,
		"fail to find a suitable kind handler":            failToFindASutableKindHandlerTC,
		"fail when kind handler fails":                    failWhenKindHandlerFailsTC,
		"fail when kind builder returns unsolvable state": failWhenKindHandlerReturnsUnsolvableState,
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

			outRes, outErr := b.Build(context.TODO(), tc.client)

			if tc.expectedErr != nil {
				if !strings.HasPrefix(outErr.Error(), tc.expectedErr.Error()) {
					t.Fatalf("Expected error: %v\nGot: %v\n", tc.expectedErr.Error(), outErr.Error())
				}
				return
			}

			if tc.expectedErr == nil && outErr != nil {
				t.Fatalf("Error not expected but got one: %v\n", outErr)
			}
			if diff := deep.Equal(tc.expectedRes, outRes); diff != nil {
				t.Logf("expected metadata: %s", tc.expectedRes.Metadata)
				t.Logf("actual metadata: %s", outRes.Metadata)
				t.Fatal(diff)
			}
		})
	}
}

var (
	webdfYml = []byte(`
kind: php
version: 7.0.29

extensions:
  intl: "*"`)

	webdfLock = []byte(`
system_packages:
  libicu-dev: "52.1-8+deb8u7"
extensions:
  intl: "*"`)
)

func successfullyBuildDefaultStageAndFileTC(mockCtrl *gomock.Controller) testCase {
	c := llbtest.NewMockClient(mockCtrl)
	c.EXPECT().BuildOpts().AnyTimes().Return(client.BuildOpts{
		SessionID: "sessid",
		Opts:      map[string]string{},
	})

	refBuildCtx := llbtest.NewMockReference(mockCtrl)
	resBuildCtx := &client.Result{
		Refs: map[string]client.Reference{"linux/amd64": refBuildCtx},
		Ref:  refBuildCtx,
	}
	c.EXPECT().Solve(gomock.Any(), gomock.Any()).Return(resBuildCtx, nil)

	readYmlReq := client.ReadRequest{Filename: "webdf.yml"}
	refBuildCtx.EXPECT().ReadFile(gomock.Any(), gomock.Eq(readYmlReq)).Return(webdfYml, nil)

	readLockReq := client.ReadRequest{Filename: "webdf.lock"}
	refBuildCtx.EXPECT().ReadFile(gomock.Any(), gomock.Eq(readLockReq)).Return(webdfLock, nil)

	refImage := llbtest.NewMockReference(mockCtrl)
	resImg := &client.Result{
		Refs: map[string]client.Reference{"linux/amd64": refImage},
		Ref:  refImage,
	}
	c.EXPECT().Solve(gomock.Any(), gomock.Any()).Return(resImg, nil)

	ctx := context.TODO()
	buildOpts := builddef.BuildOpts{
		File:      "webdf.yml",
		LockFile:  "webdf.lock",
		Stage:     "dev",
		SessionID: "sessid",
	}
	state := llb.State{}
	img := image.Image{
		Image: specs.Image{
			Author: "webdf",
		},
	}
	handler := mocks.NewMockKindHandler(mockCtrl)
	handler.EXPECT().Build(
		ctx, c, MatchBuildOpts(buildOpts),
	).Return(state, &img, nil)

	registry := registry.NewKindRegistry()
	registry.Register("php", handler)

	imgConfig := `{"author":"webdf","architecture":"","os":"","rootfs":{"type":"","diff_ids":null},"config":{}}`
	return testCase{
		client:   c,
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
		SessionID: "sessid",
		Opts: map[string]string{
			"dockerfilekey": "api.webdf.yml",
			"target":        "prod",
		},
	})

	refBuildCtx := llbtest.NewMockReference(mockCtrl)
	resBuildCtx := &client.Result{
		Refs: map[string]client.Reference{"linux/amd64": refBuildCtx},
		Ref:  refBuildCtx,
	}
	c.EXPECT().Solve(gomock.Any(), gomock.Any()).Return(resBuildCtx, nil)

	readYmlReq := client.ReadRequest{Filename: "api.webdf.yml"}
	refBuildCtx.EXPECT().ReadFile(gomock.Any(), gomock.Eq(readYmlReq)).Return(webdfYml, nil)

	readLockReq := client.ReadRequest{Filename: "api.webdf.lock"}
	refBuildCtx.EXPECT().ReadFile(gomock.Any(), gomock.Eq(readLockReq)).Return(webdfLock, nil)

	refImage := llbtest.NewMockReference(mockCtrl)
	resImg := &client.Result{
		Refs: map[string]client.Reference{"linux/amd64": refImage},
		Ref:  refImage,
	}
	c.EXPECT().Solve(gomock.Any(), gomock.Any()).Return(resImg, nil)

	ctx := context.TODO()
	buildOpts := builddef.BuildOpts{
		File:      "api.webdf.yml",
		LockFile:  "api.webdf.lock",
		Stage:     "prod",
		SessionID: "sessid",
	}
	state := llb.State{}
	img := image.Image{
		Image: specs.Image{
			Author: "webdf",
		},
	}
	handler := mocks.NewMockKindHandler(mockCtrl)
	handler.EXPECT().Build(
		ctx, c, MatchBuildOpts(buildOpts),
	).Return(state, &img, nil)

	registry := registry.NewKindRegistry()
	registry.Register("php", handler)

	imgConfig := `{"author":"webdf","architecture":"","os":"","rootfs":{"type":"","diff_ids":null},"config":{}}`
	return testCase{
		client:   c,
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

func failToResolveBuildContextTC(mockCtrl *gomock.Controller) testCase {
	c := llbtest.NewMockClient(mockCtrl)
	c.EXPECT().BuildOpts().AnyTimes().Return(client.BuildOpts{
		SessionID: "sessid",
		Opts:      map[string]string{},
	})

	err := errors.New("some error")
	c.EXPECT().Solve(gomock.Any(), gomock.Any()).Return(nil, err)

	return testCase{
		client:      c,
		registry:    registry.NewKindRegistry(),
		expectedErr: errors.New("failed to resolve build context: failed to execute solve request: some error"),
	}
}

func failToReadYmlTC(mockCtrl *gomock.Controller) testCase {
	c := llbtest.NewMockClient(mockCtrl)
	c.EXPECT().BuildOpts().AnyTimes().Return(client.BuildOpts{
		SessionID: "sessid",
		Opts:      map[string]string{},
	})

	refBuildCtx := llbtest.NewMockReference(mockCtrl)
	resBuildCtx := &client.Result{
		Refs: map[string]client.Reference{"linux/amd64": refBuildCtx},
		Ref:  refBuildCtx,
	}
	c.EXPECT().Solve(gomock.Any(), gomock.Any()).Return(resBuildCtx, nil)

	readYmlReq := client.ReadRequest{Filename: "webdf.yml"}
	err := errors.New("some error")
	refBuildCtx.EXPECT().ReadFile(gomock.Any(), gomock.Eq(readYmlReq)).Return([]byte{}, err)

	return testCase{
		client:      c,
		registry:    registry.NewKindRegistry(),
		expectedErr: errors.New("could not load webdf.yml from build context: some error"),
	}
}

func failToReadLockTC(mockCtrl *gomock.Controller) testCase {
	c := llbtest.NewMockClient(mockCtrl)
	c.EXPECT().BuildOpts().AnyTimes().Return(client.BuildOpts{
		SessionID: "sessid",
		Opts:      map[string]string{},
	})

	refBuildCtx := llbtest.NewMockReference(mockCtrl)
	resBuildCtx := &client.Result{
		Refs: map[string]client.Reference{"linux/amd64": refBuildCtx},
		Ref:  refBuildCtx,
	}
	c.EXPECT().Solve(gomock.Any(), gomock.Any()).Return(resBuildCtx, nil)

	readYmlReq := client.ReadRequest{Filename: "webdf.yml"}
	refBuildCtx.EXPECT().ReadFile(gomock.Any(), gomock.Eq(readYmlReq)).Return(webdfYml, nil)

	readLockReq := client.ReadRequest{Filename: "webdf.lock"}
	err := errors.New("some error")
	refBuildCtx.EXPECT().ReadFile(gomock.Any(), gomock.Eq(readLockReq)).Return([]byte{}, err)

	return testCase{
		client:      c,
		registry:    registry.NewKindRegistry(),
		expectedErr: errors.New("could not load webdf.lock from build context: some error"),
	}
}

func failToFindASutableKindHandlerTC(mockCtrl *gomock.Controller) testCase {
	c := llbtest.NewMockClient(mockCtrl)
	c.EXPECT().BuildOpts().AnyTimes().Return(client.BuildOpts{
		SessionID: "sessid",
		Opts:      map[string]string{},
	})

	refBuildCtx := llbtest.NewMockReference(mockCtrl)
	resBuildCtx := &client.Result{
		Refs: map[string]client.Reference{"linux/amd64": refBuildCtx},
		Ref:  refBuildCtx,
	}
	c.EXPECT().Solve(gomock.Any(), gomock.Any()).Return(resBuildCtx, nil)

	readYmlReq := client.ReadRequest{Filename: "webdf.yml"}
	refBuildCtx.EXPECT().ReadFile(gomock.Any(), gomock.Eq(readYmlReq)).Return(webdfYml, nil)

	readLockReq := client.ReadRequest{Filename: "webdf.lock"}
	refBuildCtx.EXPECT().ReadFile(gomock.Any(), gomock.Eq(readLockReq)).Return(webdfLock, nil)

	handler := mocks.NewMockKindHandler(mockCtrl)
	registry := registry.NewKindRegistry()
	registry.Register("notphp", handler)

	return testCase{
		client:      c,
		registry:    registry,
		expectedErr: errors.New("unknown kind"),
	}
}

func failWhenKindHandlerFailsTC(mockCtrl *gomock.Controller) testCase {
	c := llbtest.NewMockClient(mockCtrl)
	c.EXPECT().BuildOpts().AnyTimes().Return(client.BuildOpts{
		SessionID: "sessid",
		Opts:      map[string]string{},
	})

	refBuildCtx := llbtest.NewMockReference(mockCtrl)
	resBuildCtx := &client.Result{
		Refs: map[string]client.Reference{"linux/amd64": refBuildCtx},
		Ref:  refBuildCtx,
	}
	c.EXPECT().Solve(gomock.Any(), gomock.Any()).Return(resBuildCtx, nil)

	readYmlReq := client.ReadRequest{Filename: "webdf.yml"}
	refBuildCtx.EXPECT().ReadFile(gomock.Any(), gomock.Eq(readYmlReq)).Return(webdfYml, nil)

	readLockReq := client.ReadRequest{Filename: "webdf.lock"}
	refBuildCtx.EXPECT().ReadFile(gomock.Any(), gomock.Eq(readLockReq)).Return(webdfLock, nil)

	state := llb.State{}
	img := image.Image{}
	err := xerrors.New("some build error")
	handler := mocks.NewMockKindHandler(mockCtrl)
	handler.EXPECT().Build(gomock.Any(), c, gomock.Any()).Return(state, &img, err)

	registry := registry.NewKindRegistry()
	registry.Register("php", handler)

	return testCase{
		client:      c,
		registry:    registry,
		expectedErr: errors.New("some build error"),
	}
}

func failWhenKindHandlerReturnsUnsolvableState(mockCtrl *gomock.Controller) testCase {
	c := llbtest.NewMockClient(mockCtrl)
	c.EXPECT().BuildOpts().AnyTimes().Return(client.BuildOpts{
		SessionID: "sessid",
		Opts:      map[string]string{},
	})

	refBuildCtx := llbtest.NewMockReference(mockCtrl)
	resBuildCtx := &client.Result{
		Refs: map[string]client.Reference{"linux/amd64": refBuildCtx},
		Ref:  refBuildCtx,
	}
	c.EXPECT().Solve(gomock.Any(), gomock.Any()).Return(resBuildCtx, nil)

	readYmlReq := client.ReadRequest{Filename: "webdf.yml"}
	refBuildCtx.EXPECT().ReadFile(gomock.Any(), gomock.Eq(readYmlReq)).Return(webdfYml, nil)

	readLockReq := client.ReadRequest{Filename: "webdf.lock"}
	refBuildCtx.EXPECT().ReadFile(gomock.Any(), gomock.Eq(readLockReq)).Return(webdfLock, nil)

	c.EXPECT().Solve(gomock.Any(), gomock.Any()).Return(nil, errors.New("some solver error"))

	state := llb.State{}
	img := image.Image{
		Image: specs.Image{
			Author: "webdf",
		},
	}
	handler := mocks.NewMockKindHandler(mockCtrl)
	handler.EXPECT().Build(gomock.Any(), c, gomock.Any()).Return(state, &img, nil)

	registry := registry.NewKindRegistry()
	registry.Register("php", handler)

	return testCase{
		client:      c,
		registry:    registry,
		expectedErr: errors.New("failed to execute solve request: some solver error"),
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
		opts.Stage == m.opts.Stage
}

func (m buildOptsMatcher) String() string {
	return "opts.SessionID && opts.File && opts.LockFile && opts.Stage"
}
