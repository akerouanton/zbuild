package builder_test

import (
	"context"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/moby/buildkit/frontend/gateway/client"
	"github.com/NiR-/webdf/pkg/llbtest"
	"github.com/NiR-/webdf/pkg/service"
)

type testCase struct {
	client client.Client
	registry service.TypeRegistry
	expectedErr error
	expectedRes *client.Result
}

func TestBuilder(t *testing.T) {
	testcases := map[string]func() testCase {
		"successfully build service image": successfullyBuildImageTC,
	}

	for tcname := range testcases {
		tcinit := testcases[tcname]

		t.Run(tcname, func (t *testing.T) {
			t.Parallel()

			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			tc := tcinit(mockCtrl)
			builder = builder.NewBuilder(tc.registry)
			
			outRes, outErr := builder.Build(context.TODO(), tc.client)

			if tc.expectedErr != nil && tc.expectedErr.Error() != outErr.Error() {
				t.Fatalf("Expected error: %+v\nGot: %+v\n", tc.expectedErr, outErr)
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

func successfullyBuildServiceImageTC(mockCtrl gomock.Controller) testCase {
	registry := service.NewTypeRegistry()
	registry.Register("foo-svc-type", func (rawConfig map[string]interface{}) (llb.State, *image.Image, error) {
		return llb.State{}, nil, nil
	})

	c := llbtest.NewMockClient(mockCtrl)
	c.EXPECT().BuildOpts().Return(client.BuildOpts{
		Opts: map[string]string{
			"service": "some-service-name",
		},
	})

	return testCase{
		client: c,
		registry: registry,

	}
}
