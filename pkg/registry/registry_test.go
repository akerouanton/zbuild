package registry_test

import (
	"context"
	"testing"

	"github.com/NiR-/zbuild/pkg/builddef"
	"github.com/NiR-/zbuild/pkg/image"
	"github.com/NiR-/zbuild/pkg/mocks"
	"github.com/NiR-/zbuild/pkg/pkgsolver"
	"github.com/NiR-/zbuild/pkg/registry"
	"github.com/NiR-/zbuild/pkg/statesolver"
	"github.com/golang/mock/gomock"
	"github.com/moby/buildkit/client/llb"
	"golang.org/x/xerrors"
)

type registryTC struct {
	registry    *registry.KindRegistry
	expectedErr error
}

func TestRegistry(t *testing.T) {
	testcases := map[string]func(*gomock.Controller) registryTC{
		"successfully find builder": successfullyFindBuilderTC,
		"fail to find builder":      failToFindBuilderTC,
	}

	for tcname := range testcases {
		tcinit := testcases[tcname]

		t.Run(tcname, func(t *testing.T) {
			t.Parallel()

			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			tc := tcinit(mockCtrl)

			_, err := tc.registry.FindHandler("some-kind")
			if tc.expectedErr != nil {
				if tc.expectedErr.Error() != err.Error() {
					t.Fatalf("Expected err: %v\nGot: %v\n", tc.expectedErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Unexpected error: %v\n", err)
			}
		})
	}
}

func successfullyFindBuilderTC(mockCtrl *gomock.Controller) registryTC {
	h := mocks.NewMockKindHandler(mockCtrl)

	reg := registry.NewKindRegistry()
	reg.Register("some-kind", h)

	return registryTC{
		registry: reg,
	}
}

func failToFindBuilderTC(_ *gomock.Controller) registryTC {
	return registryTC{
		registry:    registry.NewKindRegistry(),
		expectedErr: xerrors.New("kind \"some-kind\" is not supported: unknown kind"),
	}
}

type mockKindHandler struct {
	builtImage *image.Image
}

func (h mockKindHandler) WithSolver(solver statesolver.StateSolver) {
	return
}

func (h mockKindHandler) Build(ctx context.Context, opts builddef.BuildOpts) (llb.State, *image.Image, error) {
	state := llb.State{}
	return state, h.builtImage, nil
}

func (h mockKindHandler) UpdateLocks(ctx context.Context, pkgSolver pkgsolver.PackageSolver, genericDef *builddef.BuildDef) (builddef.Locks, error) {
	return mockLocks{}, nil
}

type mockLocks struct{}

func (l mockLocks) RawLocks() map[string]interface{} {
	return map[string]interface{}{}
}
