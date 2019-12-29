package nodejs_test

import (
	"context"
	"testing"

	"github.com/NiR-/zbuild/pkg/builddef"
	"github.com/NiR-/zbuild/pkg/defkinds/nodejs"
	"github.com/NiR-/zbuild/pkg/llbtest"
	"github.com/NiR-/zbuild/pkg/mocks"
	"github.com/golang/mock/gomock"
	"github.com/moby/buildkit/frontend/gateway/client"
	"gopkg.in/yaml.v2"
)

type buildTC struct {
	handler       *nodejs.NodeJSHandler
	client        client.Client
	buildOpts     builddef.BuildOpts
	expectedState string
	// @TODO: test image metadata
	// expectedImage *image.Image
	expectedErr error
}

func initBuildLLBForDevStageTC(t *testing.T, mockCtrl *gomock.Controller) buildTC {
	genericDef := loadGenericDef(t, "testdata/build/zbuild.yml", "testdata/build/zbuild.lock")

	solver := mocks.NewMockStateSolver(mockCtrl)
	kindHandler := nodejs.NodeJSHandler{}
	kindHandler.WithSolver(solver)

	return buildTC{
		handler: &kindHandler,
		client:  llbtest.NewMockClient(mockCtrl),
		buildOpts: builddef.BuildOpts{
			Def:           &genericDef,
			Stage:         "dev",
			SessionID:     "<SESSION-ID>",
			LocalUniqueID: "x1htr02606a9rk8b0daewh9es",
			ContextName:   "context",
		},
		expectedState: "testdata/build/state-dev.json",
	}
}

func initBuildLLBForWorkerStageTC(t *testing.T, mockCtrl *gomock.Controller) buildTC {
	genericDef := loadGenericDef(t, "testdata/build/zbuild.yml", "testdata/build/zbuild.lock")

	solver := mocks.NewMockStateSolver(mockCtrl)
	kindHandler := nodejs.NodeJSHandler{}
	kindHandler.WithSolver(solver)

	return buildTC{
		handler: &kindHandler,
		client:  llbtest.NewMockClient(mockCtrl),
		buildOpts: builddef.BuildOpts{
			Def:           &genericDef,
			Stage:         "worker",
			SessionID:     "<SESSION-ID>",
			LocalUniqueID: "x1htr02606a9rk8b0daewh9es",
			ContextName:   "context",
		},
		expectedState: "testdata/build/state-worker.json",
	}
}

func initBuildLLBForProdWebserverStageTC(t *testing.T, mockCtrl *gomock.Controller) buildTC {
	genericDef := loadGenericDef(t, "testdata/build/frontend.yml", "testdata/build/frontend.lock")

	solver := mocks.NewMockStateSolver(mockCtrl)
	kindHandler := nodejs.NodeJSHandler{}
	kindHandler.WithSolver(solver)

	return buildTC{
		handler: &kindHandler,
		client:  llbtest.NewMockClient(mockCtrl),
		buildOpts: builddef.BuildOpts{
			Def:           &genericDef,
			Stage:         "webserver-prod",
			SessionID:     "<SESSION-ID>",
			LocalUniqueID: "x1htr02606a9rk8b0daewh9es",
			ContextName:   "context",
		},
		expectedState: "testdata/build/state-webserver.json",
	}
}

func TestBuild(t *testing.T) {
	testcases := map[string]func(*testing.T, *gomock.Controller) buildTC{
		"build LLB DAG for dev stage":            initBuildLLBForDevStageTC,
		"build LLB DAG for worker stage":         initBuildLLBForWorkerStageTC,
		"build LLB DAG for prod webserver stage": initBuildLLBForProdWebserverStageTC,
	}

	for tcname := range testcases {
		tcinit := testcases[tcname]

		t.Run(tcname, func(t *testing.T) {
			t.Parallel()

			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			tc := tcinit(t, mockCtrl)
			ctx := context.TODO()

			state, _, err := tc.handler.Build(ctx, tc.buildOpts)
			jsonState := llbtest.StateToJSON(t, state)

			if *flagTestdata {
				if tc.expectedState != "" {
					writeTestdata(t, tc.expectedState, jsonState)
				}
			}

			if tc.expectedErr != nil {
				if err == nil || tc.expectedErr.Error() != err.Error() {
					t.Fatalf("Expected error: %v\nGot: %v", tc.expectedErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			// @TODO: uncomment
			/* img.Created = nil
			if diff := deep.Equal(img, tc.expectedImage); diff != nil {
				t.Fatal(diff)
			} */

			expectedState := loadRawTestdata(t, tc.expectedState)
			if string(expectedState) != jsonState {
				tempfile := newTempFile(t)
				writeTestdata(t, tempfile, jsonState)

				t.Fatalf("Expected: <%s>\nGot: <%s>", tc.expectedState, tempfile)
			}
		})
	}
}

type debugConfigTC struct {
	handler     *nodejs.NodeJSHandler
	buildOpts   builddef.BuildOpts
	expected    string
	expectedErr error
}

func initDebugDevStageTC(t *testing.T, mockCtrl *gomock.Controller) debugConfigTC {
	solver := mocks.NewMockStateSolver(mockCtrl)

	h := &nodejs.NodeJSHandler{}
	h.WithSolver(solver)

	genericDef := loadGenericDef(t, "testdata/debug-config/zbuild.yml",
		"testdata/debug-config/zbuild.lock")

	return debugConfigTC{
		handler: h,
		buildOpts: builddef.BuildOpts{
			Def:   &genericDef,
			Stage: "dev",
		},
		expected: "testdata/debug-config/dump-dev.yml",
	}
}

func initDebugProdStageTC(t *testing.T, mockCtrl *gomock.Controller) debugConfigTC {
	solver := mocks.NewMockStateSolver(mockCtrl)

	h := &nodejs.NodeJSHandler{}
	h.WithSolver(solver)

	genericDef := loadGenericDef(t, "testdata/debug-config/zbuild.yml",
		"testdata/debug-config/zbuild.lock")

	return debugConfigTC{
		handler: h,
		buildOpts: builddef.BuildOpts{
			Def:   &genericDef,
			Stage: "prod",
		},
		expected: "testdata/debug-config/dump-prod.yml",
	}
}

func initDebugWebserverProdStageTC(t *testing.T, mockCtrl *gomock.Controller) debugConfigTC {
	solver := mocks.NewMockStateSolver(mockCtrl)

	h := &nodejs.NodeJSHandler{}
	h.WithSolver(solver)

	genericDef := loadGenericDef(t, "testdata/debug-config/zbuild.yml",
		"testdata/debug-config/zbuild.lock")

	return debugConfigTC{
		handler: h,
		buildOpts: builddef.BuildOpts{
			Def:   &genericDef,
			Stage: "webserver-prod",
		},
		expected: "testdata/debug-config/dump-webserver-prod.yml",
	}
}

func TestDebugConfig(t *testing.T) {
	testcases := map[string]func(*testing.T, *gomock.Controller) debugConfigTC{
		"debug dev stage config":            initDebugDevStageTC,
		"debug prod stage config":           initDebugProdStageTC,
		"debug webserver-prod stage config": initDebugWebserverProdStageTC,
	}

	for tcname := range testcases {
		tcinit := testcases[tcname]

		t.Run(tcname, func(t *testing.T) {
			t.Parallel()

			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			tc := tcinit(t, mockCtrl)

			dump, err := tc.handler.DebugConfig(tc.buildOpts)
			if tc.expectedErr != nil {
				if err == nil || err.Error() != tc.expectedErr.Error() {
					t.Fatalf("Expected error: %v\nGot: %v", tc.expectedErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			raw, err := yaml.Marshal(dump)
			if err != nil {
				t.Fatal(err)
			}

			if *flagTestdata {
				writeTestdata(t, tc.expected, string(raw))
				return
			}

			expected := loadRawTestdata(t, tc.expected)
			if string(expected) != string(raw) {
				t.Fatalf("Expected: %s\nGot: %s", expected, string(raw))
			}
		})
	}
}
