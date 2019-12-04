package registry_test

import (
	"context"
	"testing"

	"github.com/NiR-/zbuild/pkg/builddef"
	"github.com/NiR-/zbuild/pkg/image"
	"github.com/NiR-/zbuild/pkg/llbtest"
	"github.com/NiR-/zbuild/pkg/pkgsolver"
	"github.com/NiR-/zbuild/pkg/registry"
	"github.com/go-test/deep"
	"github.com/golang/mock/gomock"
	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/frontend/gateway/client"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
)

type registryTC struct {
	name          string
	registry      *registry.KindRegistry
	builddef      builddef.BuildDef
	expectedImage *image.Image
	expectedErr   error
}

func TestRegistry(t *testing.T) {
	testcases := []registryTC{
		successfullyFindBuilderTC(),
		failToFindBuilderTC(),
	}

	for tid := range testcases {
		tc := testcases[tid]

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			handler, err := tc.registry.FindHandler(tc.builddef.Kind)
			if tc.expectedErr != nil {
				if tc.expectedErr.Error() != err.Error() {
					t.Fatalf("Expected err: %v\nGot: %v\n", tc.expectedErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Unexpected error: %v\n", err)
			}

			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			c := llbtest.NewMockClient(mockCtrl)
			buildOpts := builddef.BuildOpts{
				Def:       &tc.builddef,
				SessionID: "sessid",
				Stage:     "base",
			}

			_, img, err := handler.Build(context.TODO(), c, buildOpts)
			if err != nil {
				t.Fatalf("Unexpected error: %v\n", err)
			}
			if diff := deep.Equal(img, tc.expectedImage); diff != nil {
				t.Error(diff)
			}
		})
	}
}

func successfullyFindBuilderTC() registryTC {
	expectedImage := image.Image{
		Image: specs.Image{
			Author: "zbuild",
		},
	}

	reg := registry.NewKindRegistry()
	reg.Register("some-kind", mockKindHandler{&expectedImage})

	return registryTC{
		name:     "it finds the requested service builder",
		registry: reg,
		builddef: builddef.BuildDef{
			Kind:      "some-kind",
			RawConfig: map[string]interface{}{},
			RawLocks:  []byte{},
		},
		expectedImage: &expectedImage,
	}
}

func failToFindBuilderTC() registryTC {
	return registryTC{
		name:     "it fails to find the appropriate builder for the given kind",
		registry: registry.NewKindRegistry(),
		builddef: builddef.BuildDef{
			Kind:      "some-kind",
			RawConfig: map[string]interface{}{},
			RawLocks:  []byte{},
		},
		expectedErr: registry.ErrUnknownDefKind,
	}
}

type mockKindHandler struct {
	builtImage *image.Image
}

func (h mockKindHandler) Build(ctx context.Context, c client.Client, opts builddef.BuildOpts) (llb.State, *image.Image, error) {
	state := llb.State{}
	return state, h.builtImage, nil
}

func (h mockKindHandler) DebugLLB(buildOpts builddef.BuildOpts) (llb.State, error) {
	state := llb.State{}
	return state, nil
}

func (h mockKindHandler) UpdateLocks(genericDef *builddef.BuildDef, pkgSolver pkgsolver.PackageSolver) (builddef.Locks, error) {
	return mockLocks{}, nil
}

type mockLocks struct{}

func (l mockLocks) RawLocks() ([]byte, error) {
	return []byte{}, nil
}
