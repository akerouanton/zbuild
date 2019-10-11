package service_test

import (
	"context"
	"testing"
	
	"github.com/go-test/deep"
	"github.com/moby/buildkit/client/llb"
	"github.com/NiR-/webdf/pkg/image"
	"github.com/NiR-/webdf/pkg/service"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
)

type registryTC struct {
	name          string
	types         map[string]service.Builder
	service       *service.Service
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

			registry := service.NewTypeRegistry()

			for typeName, decoder := range tc.types {
				registry.Register(typeName, decoder)
			}

			svcBuilder, err := registry.FindBuilder(tc.service.Type)
			if tc.expectedErr != nil {
				if tc.expectedErr.Error() != err.Error() {
					t.Fatalf("Expected err: %v\nGot: %v\n", tc.expectedErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Unexpected error: %v\n", err)
			}

			_, img, err := svcBuilder(context.TODO(), tc.service, "1234")
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

	return registryTC{
		name: "it finds the requested service builder",
		types: map[string]service.Builder{
			"some-type": func(context.Context, *service.Service, string) (llb.State, *image.Image, error) {
				return llb.State{}, &expectedImage, nil
			},
		},
		service:       service.NewService("my-service", "some-type", map[string]interface{}{}),
		expectedImage: &expectedImage,
		expectedErr:   nil,
	}
}

func failToFindBuilderTC() registryTC {
	return registryTC{
		name:          "it fails to find the appropriate builder for the given service type",
		types:         map[string]service.Builder{},
		service:       service.NewService("my-service", "some-type", map[string]interface{}{}),
		expectedImage: &image.Image{},
		expectedErr:   service.ErrUnknownServiceType,
	}
}
