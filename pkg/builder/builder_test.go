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
	"github.com/NiR-/webdf/pkg/pkgsolver"
	"github.com/NiR-/webdf/pkg/registry"
	"github.com/go-test/deep"
	"github.com/golang/mock/gomock"
	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/frontend/gateway/client"
)

type testCase struct {
	client      client.Client
	registry    *registry.TypeRegistry
	expectedErr error
	expectedRes *client.Result
}

func TestBuilder(t *testing.T) {
	testcases := map[string]func(*gomock.Controller) testCase{
		"successfully build service image":                successfullyBuildServiceImageTC,
		"fail to resolve build context":                   failToResolveBuildContextTC,
		"fail to read webdf.yml file":                     failToReadYmlTC,
		"fail to read webdf.lock file":                    failToReadLockTC,
		"fail to find a suitable type handler":            failToFindASutableTypeHandlerTC,
		"fail when type handler fails":                    failWhenTypeHandlerFailsTC,
		"fail when type builder returns unsolvable state": failWhenTypeHandlerReturnsUnsolvableState,
	}

	for tcname := range testcases {
		tcinit := testcases[tcname]

		t.Run(tcname, func(t *testing.T) {
			t.Parallel()

			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			tc := tcinit(mockCtrl)
			b := builder.Builder{tc.registry}

			outRes, outErr := b.Build(context.TODO(), tc.client)

			if tc.expectedErr != nil && !strings.HasPrefix(outErr.Error(), tc.expectedErr.Error()) {
				t.Fatalf("Expected error: %+v\nGot: %+v\n", tc.expectedErr.Error(), outErr.Error())
			}
			if tc.expectedErr == nil && outErr != nil {
				t.Fatalf("Error not expected but got one: %+v\n", outErr)
			}
			if diff := deep.Equal(tc.expectedRes, outRes); diff != nil {
				t.Fatal(diff)
			}
		})
	}
}

var (
	webdfYml = []byte(`
type: php
version: 7.0.29

extensions:
  intl: "*"`)

	webdfLock = []byte(`
system_packages:
  libicu-dev: "52.1-8+deb8u7"
extensions:
  intl: "*"`)
)

func successfullyBuildServiceImageTC(mockCtrl *gomock.Controller) testCase {
	registry := registry.NewTypeRegistry()
	registry.Register("php", mockTypeHandler{})

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

	return testCase{
		client:   c,
		registry: registry,
		expectedRes: &client.Result{
			Refs: map[string]client.Reference{"linux/amd64": refImage},
			Ref:  refImage,
			Metadata: map[string][]byte{
				"containerimage.config": []byte("null"),
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
		registry:    registry.NewTypeRegistry(),
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
		registry:    registry.NewTypeRegistry(),
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
		registry:    registry.NewTypeRegistry(),
		expectedErr: errors.New("could not load webdf.lock from build context: some error"),
	}
}

func failToFindASutableTypeHandlerTC(mockCtrl *gomock.Controller) testCase {
	registry := registry.NewTypeRegistry()
	registry.Register("notphp", mockTypeHandler{})

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

	return testCase{
		client:      c,
		registry:    registry,
		expectedErr: errors.New("unknown service type"),
	}
}

func failWhenTypeHandlerFailsTC(mockCtrl *gomock.Controller) testCase {
	registry := registry.NewTypeRegistry()
	registry.Register("php", mockTypeHandler{failing: true})

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

	return testCase{
		client:      c,
		registry:    registry,
		expectedErr: errors.New("some build error"),
	}
}

func failWhenTypeHandlerReturnsUnsolvableState(mockCtrl *gomock.Controller) testCase {
	registry := registry.NewTypeRegistry()
	registry.Register("php", mockTypeHandler{})

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

	return testCase{
		client:      c,
		registry:    registry,
		expectedErr: errors.New("failed to execute solve request: some solver error"),
	}
}

type mockTypeHandler struct {
	failing bool
}

func (h mockTypeHandler) Build(ctx context.Context, c client.Client, opts builddef.BuildOpts) (llb.State, *image.Image, error) {
	state := llb.State{}
	if h.failing {
		return state, nil, errors.New("some build error")
	}
	return state, nil, nil
}

func (h mockTypeHandler) UpdateLocks(*builddef.BuildDef, []string, pkgsolver.PackageSolver) (builddef.Locks, error) {
	return mockLocks{}, nil
}

type mockLocks struct{}

func (l mockLocks) RawLocks() ([]byte, error) {
	return []byte{}, nil
}
