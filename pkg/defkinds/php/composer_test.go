package php_test

import (
	"context"
	"testing"

	"github.com/NiR-/zbuild/pkg/builddef"
	"github.com/NiR-/zbuild/pkg/defkinds/php"
	"github.com/NiR-/zbuild/pkg/llbtest"
	"github.com/go-test/deep"
	"github.com/golang/mock/gomock"
	"github.com/moby/buildkit/frontend/gateway/client"
	"golang.org/x/xerrors"
)

func TestLoadPlatformReqsFromFS(t *testing.T) {
	if *flagTestdata {
		return
	}

	testcases := map[string]struct {
		basedir     string
		initial     php.StageDefinition
		expected    php.StageDefinition
		expectedErr error
	}{
		"successfully load and parse composer.lock file": {
			basedir: "testdata/composer/valid",
			initial: php.StageDefinition{
				Stage: php.Stage{
					Extensions: map[string]string{},
				},
			},
			expected: php.StageDefinition{
				Stage: php.Stage{
					Extensions: map[string]string{"mbstring": "*"},
				},
			},
		},
		"silently fail when composer.lock file does not exist": {
			basedir: "testdata/composer/nonexistant",
			initial: php.StageDefinition{
				Stage: php.Stage{
					Extensions: map[string]string{},
				},
			},
			expected: php.StageDefinition{
				Stage: php.Stage{
					Extensions: map[string]string{},
				},
			},
		},
		"fail to load broken composer.lock file": {
			basedir: "testdata/composer/broken",
			initial: php.StageDefinition{
				Stage: php.Stage{
					Extensions: map[string]string{},
				},
			},
			expectedErr: xerrors.New("could not unmarshal composer.lock: unexpected end of JSON input"),
		},
		"it does not change version constraints of extensions already defined": {
			basedir: "testdata/composer/valid",
			initial: php.StageDefinition{
				Stage: php.Stage{
					Extensions: map[string]string{"mbstring": "1.2.3"},
				},
			},
			expected: php.StageDefinition{
				Stage: php.Stage{
					Extensions: map[string]string{"mbstring": "1.2.3"},
				},
			},
		},
	}

	for tcname := range testcases {
		tc := testcases[tcname]

		t.Run(tcname, func(t *testing.T) {
			t.Parallel()

			stage := tc.initial
			err := php.LoadPlatformReqsFromFS(&stage, tc.basedir)

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

			if diff := deep.Equal(stage, tc.expected); diff != nil {
				t.Fatal(diff)
			}
		})
	}
}

type loadPlatformReqsFromContextTC struct {
	client      client.Client
	opts        builddef.BuildOpts
	initial     php.StageDefinition
	expected    php.StageDefinition
	expectedErr error
}

func initSuccessfullyLoadAndParseComposerLockTC(t *testing.T, mockCtrl *gomock.Controller) loadPlatformReqsFromContextTC {
	contextRef := llbtest.NewMockReference(mockCtrl)
	solvedContext := &client.Result{
		Refs: map[string]client.Reference{"linux/amd64": contextRef},
		Ref:  contextRef,
	}

	ctx := context.TODO()
	c := llbtest.NewMockClient(mockCtrl)
	c.EXPECT().Solve(ctx, gomock.Any()).Return(solvedContext, nil)

	rawLock := loadTestdata(t, "testdata/composer/valid/composer.lock")
	contextRef.EXPECT().ReadFile(ctx, client.ReadRequest{
		Filename: "composer.lock",
	}).Return([]byte(rawLock), nil)

	return loadPlatformReqsFromContextTC{
		client: c,
		opts:   builddef.BuildOpts{},
		initial: php.StageDefinition{
			Stage: php.Stage{
				Extensions: map[string]string{},
			},
		},
		expected: php.StageDefinition{
			Stage: php.Stage{
				Extensions: map[string]string{"mbstring": "*"},
			},
		},
	}
}

func initSilentlyFailWhenNoComposerLockTC(t *testing.T, mockCtrl *gomock.Controller) loadPlatformReqsFromContextTC {
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

	return loadPlatformReqsFromContextTC{
		client: c,
		opts:   builddef.BuildOpts{},
		initial: php.StageDefinition{
			Stage: php.Stage{
				Extensions: map[string]string{},
			},
		},
		expected: php.StageDefinition{
			Stage: php.Stage{
				Extensions: map[string]string{},
			},
		},
	}
}

func initFailToLoadBrokenComposerLockTC(t *testing.T, mockCtrl *gomock.Controller) loadPlatformReqsFromContextTC {
	contextRef := llbtest.NewMockReference(mockCtrl)
	solvedContext := &client.Result{
		Refs: map[string]client.Reference{"linux/amd64": contextRef},
		Ref:  contextRef,
	}

	ctx := context.TODO()
	c := llbtest.NewMockClient(mockCtrl)
	c.EXPECT().Solve(ctx, gomock.Any()).Return(solvedContext, nil)

	rawLock := loadTestdata(t, "testdata/composer/broken/composer.lock")
	contextRef.EXPECT().ReadFile(ctx, client.ReadRequest{
		Filename: "composer.lock",
	}).Return([]byte(rawLock), nil)

	return loadPlatformReqsFromContextTC{
		client: c,
		opts:   builddef.BuildOpts{},
		initial: php.StageDefinition{
			Stage: php.Stage{
				Extensions: map[string]string{},
			},
		},
		expectedErr: xerrors.New("could not unmarshal composer.lock: unexpected end of JSON input"),
	}
}

func initDontChangeExistingExtVersionConstraintsTC(t *testing.T, mockCtrl *gomock.Controller) loadPlatformReqsFromContextTC {
	contextRef := llbtest.NewMockReference(mockCtrl)
	solvedContext := &client.Result{
		Refs: map[string]client.Reference{"linux/amd64": contextRef},
		Ref:  contextRef,
	}

	ctx := context.TODO()
	c := llbtest.NewMockClient(mockCtrl)
	c.EXPECT().Solve(ctx, gomock.Any()).Return(solvedContext, nil)

	rawLock := loadTestdata(t, "testdata/composer/valid/composer.lock")
	contextRef.EXPECT().ReadFile(ctx, client.ReadRequest{
		Filename: "composer.lock",
	}).Return([]byte(rawLock), nil)

	return loadPlatformReqsFromContextTC{
		client: c,
		opts:   builddef.BuildOpts{},
		initial: php.StageDefinition{
			Stage: php.Stage{
				Extensions: map[string]string{"mbstring": "1.2.3"},
			},
		},
		expected: php.StageDefinition{
			Stage: php.Stage{
				Extensions: map[string]string{"mbstring": "1.2.3"},
			},
		},
	}
}

func TestLoadPlatformReqsFromContext(t *testing.T) {
	if *flagTestdata {
		return
	}

	testcases := map[string]func(*testing.T, *gomock.Controller) loadPlatformReqsFromContextTC{
		"successfully load and parse composer.lock":                            initSuccessfullyLoadAndParseComposerLockTC,
		"silently fail when composer.lock file does not exist":                 initSilentlyFailWhenNoComposerLockTC,
		"fail to load broken composer.lock file":                               initFailToLoadBrokenComposerLockTC,
		"it does not change version constraints of extensions already defined": initDontChangeExistingExtVersionConstraintsTC,
	}

	for tcname := range testcases {
		tcinit := testcases[tcname]

		t.Run(tcname, func(t *testing.T) {
			t.Parallel()

			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			tc := tcinit(t, mockCtrl)
			ctx := context.TODO()
			stage := tc.initial

			err := php.LoadPlatformReqsFromContext(ctx, tc.client, &stage, tc.opts)

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

			if diff := deep.Equal(stage, tc.expected); diff != nil {
				t.Fatal(diff)
			}
		})
	}
}
