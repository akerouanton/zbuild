package registry_test

import (
	"context"
	"testing"

	"github.com/NiR-/webdf/pkg/builddef"
	"github.com/NiR-/webdf/pkg/image"
	"github.com/NiR-/webdf/pkg/llbtest"
	"github.com/NiR-/webdf/pkg/pkgsolver"
	"github.com/NiR-/webdf/pkg/registry"
	"github.com/go-test/deep"
	"github.com/golang/mock/gomock"
	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/frontend/gateway/client"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
)

type registryTC struct {
	name          string
	registry      *registry.TypeRegistry
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

			typeHandler, err := tc.registry.FindTypeHandler(tc.builddef.Type)
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

			_, img, err := typeHandler.Build(context.TODO(), c, buildOpts)
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
			Author: "webdf",
		},
	}

	reg := registry.NewTypeRegistry()
	reg.Register("some-type", mockTypeHandler{&expectedImage})

	return registryTC{
		name:     "it finds the requested service builder",
		registry: reg,
		builddef: builddef.BuildDef{
			Type:      "some-type",
			RawConfig: map[string]interface{}{},
			RawLocks:  []byte{},
		},
		expectedImage: &expectedImage,
	}
}

func failToFindBuilderTC() registryTC {
	return registryTC{
		name:     "it fails to find the appropriate builder for the given service type",
		registry: registry.NewTypeRegistry(),
		builddef: builddef.BuildDef{
			Type:      "some-type",
			RawConfig: map[string]interface{}{},
			RawLocks:  []byte{},
		},
		expectedErr: registry.ErrUnknownDefType,
	}
}

type mockTypeHandler struct {
	builtImage *image.Image
}

func (h mockTypeHandler) Build(ctx context.Context, c client.Client, opts builddef.BuildOpts) (llb.State, *image.Image, error) {
	state := llb.State{}
	return state, h.builtImage, nil
}

func (h mockTypeHandler) DebugLLB(buildOpts builddef.BuildOpts) (llb.State, error) {
	state := llb.State{}
	return state, nil
}

func (h mockTypeHandler) UpdateLocks(genericDef *builddef.BuildDef, pkgSolver pkgsolver.PackageSolver) (builddef.Locks, error) {
	return mockLocks{}, nil
}

type mockLocks struct{}

func (l mockLocks) RawLocks() ([]byte, error) {
	return []byte{}, nil
}
