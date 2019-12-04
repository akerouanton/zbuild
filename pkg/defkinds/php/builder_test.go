package php_test

import (
	"context"
	"io/ioutil"
	"testing"

	"github.com/NiR-/webdf/pkg/builddef"
	"github.com/NiR-/webdf/pkg/defkinds/php"
	"github.com/NiR-/webdf/pkg/llbtest"
	"github.com/NiR-/webdf/pkg/mocks"
	"github.com/golang/mock/gomock"
	"github.com/moby/buildkit/frontend/gateway/client"
	"golang.org/x/xerrors"
)

type buildTC struct {
	handler       php.PHPHandler
	client        client.Client
	buildOpts     builddef.BuildOpts
	expectedState string
	// expectedImage *image.Image
	expectedErr error
}

func initBuildLLBForDevStageTC(t *testing.T, mockCtrl *gomock.Controller) buildTC {
	contextRef := llbtest.NewMockReference(mockCtrl)
	solvedContext := &client.Result{
		Refs: map[string]client.Reference{"linux/amd64": contextRef},
		Ref:  contextRef,
	}

	ctx := context.TODO()
	c := llbtest.NewMockClient(mockCtrl)
	c.EXPECT().Solve(ctx, gomock.Any()).Return(solvedContext, nil)

	contextRef.EXPECT().ReadFile(ctx, client.ReadRequest{
		Filename: "composer.lock",
	}).Return([]byte{}, xerrors.New("file does not exist"))

	fetcher := mocks.NewMockFileFetcher(mockCtrl)
	genericDef := loadGenericDef(t, "testdata/build/webdf.yml", "testdata/build/webdf.lock")

	return buildTC{
		handler: php.NewPHPHandler(fetcher),
		client:  c,
		buildOpts: builddef.BuildOpts{
			Def:           &genericDef,
			Stage:         "dev",
			SessionID:     "<SESSION-ID>",
			LocalUniqueID: "x1htr02606a9rk8b0daewh9es",
		},
		expectedState: "testdata/build/state-dev.json",
	}
}

func initBuildLLBForProdStageTC(t *testing.T, mockCtrl *gomock.Controller) buildTC {
	contextRef := llbtest.NewMockReference(mockCtrl)
	solvedContext := &client.Result{
		Refs: map[string]client.Reference{"linux/amd64": contextRef},
		Ref:  contextRef,
	}

	ctx := context.TODO()
	c := llbtest.NewMockClient(mockCtrl)
	c.EXPECT().Solve(ctx, gomock.Any()).Return(solvedContext, nil)

	contextRef.EXPECT().ReadFile(ctx, client.ReadRequest{
		Filename: "composer.lock",
	}).Return([]byte{}, xerrors.New("file does not exist"))

	fetcher := mocks.NewMockFileFetcher(mockCtrl)
	genericDef := loadGenericDef(t, "testdata/build/webdf.yml", "testdata/build/webdf.lock")

	return buildTC{
		handler: php.NewPHPHandler(fetcher),
		client:  c,
		buildOpts: builddef.BuildOpts{
			Def:           &genericDef,
			Stage:         "prod",
			SessionID:     "<SESSION-ID>",
			LocalUniqueID: "x1htr02606a9rk8b0daewh9es",
		},
		expectedState: "testdata/build/state-prod.json",
	}
}

func TestBuild(t *testing.T) {
	testcases := map[string]func(*testing.T, *gomock.Controller) buildTC{
		"build LLB DAG for dev stage":  initBuildLLBForDevStageTC,
		"build LLB DAG for prod stage": initBuildLLBForProdStageTC,
	}

	for tcname := range testcases {
		tcinit := testcases[tcname]

		t.Run(tcname, func(t *testing.T) {
			t.Parallel()

			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			tc := tcinit(t, mockCtrl)
			ctx := context.TODO()

			state, _, err := tc.handler.Build(ctx, tc.client, tc.buildOpts)
			jsonState := llbtest.StateToJSON(t, state)

			if *flagTestdata {
				if tc.expectedState != "" {
					writeTestdata(t, tc.expectedState, jsonState)
				}
			}

			if tc.expectedErr != nil {
				if err == nil {
					t.Fatalf("Expected error: %v\nGot: <nil>", tc.expectedErr)
				}
				if tc.expectedErr.Error() != err.Error() {
					t.Fatalf("Expected error: %v\nGot: %v", tc.expectedErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			/* img.Created = nil
			if diff := deep.Equal(img, tc.expectedImage); diff != nil {
				t.Fatal(diff)
			} */

			expectedState := loadTestdata(t, tc.expectedState)
			if expectedState != jsonState {
				tempfile := newTempFile(t)
				writeTestdata(t, tempfile, jsonState)

				t.Fatalf("Expected: <%s>\nGot: <%s>", tc.expectedState, tempfile)
			}
		})
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

type debugTC struct {
	handler       php.PHPHandler
	buildOpts     builddef.BuildOpts
	expectedState string
	expectedErr   error
}

func initDebugLLBForDevStageTC(t *testing.T, mockCtrl *gomock.Controller) debugTC {
	fetcher := mocks.NewMockFileFetcher(mockCtrl)
	genericDef := loadGenericDef(t, "testdata/build/webdf.yml", "testdata/build/webdf.lock")

	return debugTC{
		handler: php.NewPHPHandler(fetcher),
		buildOpts: builddef.BuildOpts{
			Def:           &genericDef,
			Stage:         "dev",
			LocalUniqueID: "x1htr02606a9rk8b0daewh9es",
		},
		expectedState: "testdata/build/state-dev.json",
	}
}

func initDebugLLBForProdStageTC(t *testing.T, mockCtrl *gomock.Controller) debugTC {
	fetcher := mocks.NewMockFileFetcher(mockCtrl)
	genericDef := loadGenericDef(t, "testdata/build/webdf.yml", "testdata/build/webdf.lock")

	return debugTC{
		handler: php.NewPHPHandler(fetcher),
		buildOpts: builddef.BuildOpts{
			Def:           &genericDef,
			Stage:         "prod",
			LocalUniqueID: "x1htr02606a9rk8b0daewh9es",
		},
		expectedState: "testdata/build/state-prod.json",
	}
}

func TestDebugLLB(t *testing.T) {
	if *flagTestdata {
		return
	}

	testcases := map[string]func(*testing.T, *gomock.Controller) debugTC{
		"debug LLB DAG for dev stage":  initDebugLLBForDevStageTC,
		"debug LLB DAG for prod stage": initDebugLLBForProdStageTC,
	}

	for tcname := range testcases {
		tcinit := testcases[tcname]

		t.Run(tcname, func(t *testing.T) {
			t.Parallel()

			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			tc := tcinit(t, mockCtrl)

			state, err := tc.handler.DebugLLB(tc.buildOpts)
			jsonState := llbtest.StateToJSON(t, state)

			if tc.expectedErr != nil {
				if err == nil {
					t.Fatalf("Expected error: %v\nGot: <nil>", tc.expectedErr)
				}
				if tc.expectedErr.Error() != err.Error() {
					t.Fatalf("Expected error: %v\nGot: %v", tc.expectedErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			expectedState := loadTestdata(t, tc.expectedState)
			if expectedState != jsonState {
				tempfile := newTempFile(t)
				writeTestdata(t, tempfile, jsonState)

				t.Fatalf("Expected: <%s>\nGot: <%s>", tc.expectedState, tempfile)
			}
		})
	}
}
