package builder_test

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"testing"

	"github.com/NiR-/zbuild/pkg/builddef"
	"github.com/NiR-/zbuild/pkg/builder"
	"github.com/NiR-/zbuild/pkg/image"
	"github.com/NiR-/zbuild/pkg/llbtest"
	"github.com/NiR-/zbuild/pkg/mocks"
	"github.com/NiR-/zbuild/pkg/registry"
	"github.com/NiR-/zbuild/pkg/statesolver"
	"github.com/go-test/deep"
	"github.com/golang/mock/gomock"
	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/frontend/gateway/client"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/twpayne/go-vfs"
	"github.com/twpayne/go-vfs/vfst"
	"golang.org/x/xerrors"
)

var flagTestdata = flag.Bool("testdata", false, "Use this flag to (re)generate testdata (lockfiles)")

type buildTC struct {
	client      client.Client
	solver      statesolver.StateSolver
	registry    *registry.KindRegistry
	expectedErr error
	expectedRes *client.Result
}

func TestBuilderBuild(t *testing.T) {
	if *flagTestdata {
		return
	}

	testcases := map[string]func(*testing.T, *gomock.Controller) buildTC{
		"build default stage and file":         initBuildDefaultStageAndFileTC,
		"build custom stage and file":          initBuildCustomStageAndFileTC,
		"build from git context":               initBuildFromGitContextTC,
		"build webserver stage":                initBuildWebserverStageTC,
		"fail to read zbuild.yml file":         failToReadYmlTC,
		"fail to find a suitable kind handler": failToFindASutableKindHandlerTC,
		"fail when kind handler fails":         failWhenKindHandlerFailsTC,
		"fail when lockfile is out-of-sync":    failWhenLockfileIsOutOfSyncTC,
	}

	for tcname := range testcases {
		tcinit := testcases[tcname]

		t.Run(tcname, func(t *testing.T) {
			t.Parallel()

			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			tc := tcinit(t, mockCtrl)
			b := builder.Builder{
				Registry: tc.registry,
			}

			outRes, outErr := b.Build(context.TODO(), tc.solver, tc.client)

			if tc.expectedErr != nil {
				if outErr == nil || outErr.Error() != tc.expectedErr.Error() {
					t.Fatalf("Expected error: %v\nGot: %v", tc.expectedErr.Error(), outErr.Error())
				}
				return
			}
			if outErr != nil {
				t.Fatalf("Unexpected error: %v", outErr)
			}
			if diff := deep.Equal(outRes, tc.expectedRes); diff != nil {
				t.Logf("expected metadata: %s", tc.expectedRes.Metadata)
				t.Logf("actual metadata: %s", outRes.Metadata)
				t.Fatal(diff)
			}
		})
	}
}

func initBuildDefaultStageAndFileTC(t *testing.T, mockCtrl *gomock.Controller) buildTC {
	c := llbtest.NewMockClient(mockCtrl)
	c.EXPECT().BuildOpts().AnyTimes().Return(client.BuildOpts{
		SessionID: "<SESSION-ID>",
		Opts: map[string]string{
			"context": "some-context-name",
		},
	})

	zbuildYml := loadRawTestdata(t, "testdata/build/zbuild.yml")
	zbuildLock := loadRawTestdata(t, "testdata/build/zbuild.lock")

	solver := mocks.NewMockStateSolver(mockCtrl)
	solver.EXPECT().FromContext(gomock.Any(), gomock.Any()).Times(1)

	solver.EXPECT().ReadFile(
		gomock.Any(), "zbuild.yml", gomock.Any(),
	).Return(zbuildYml, nil)

	solver.EXPECT().ReadFile(
		gomock.Any(), "zbuild.lock", gomock.Any(),
	).Return(zbuildLock, nil)

	ctx := context.TODO()
	buildOpts := builddef.BuildOpts{
		File:      "zbuild.yml",
		LockFile:  "zbuild.lock",
		Stage:     "dev",
		SessionID: "<SESSION-ID>",
		BuildContext: &builddef.Context{
			Source: "some-context-name",
			Type:   builddef.ContextTypeLocal,
		},
	}
	state := llb.State{}
	img := image.Image{
		Image: specs.Image{
			Author: "zbuild",
		},
	}
	handler := mocks.NewMockKindHandler(mockCtrl)
	handler.EXPECT().WithSolver(gomock.Any()).Times(1)
	handler.EXPECT().Build(
		ctx, MatchBuildOpts(buildOpts),
	).Return(state, &img, nil)

	registry := registry.NewKindRegistry()
	registry.Register("php", handler, false)

	refImage := llbtest.NewMockReference(mockCtrl)
	resImg := &client.Result{
		Refs: map[string]client.Reference{"linux/amd64": refImage},
		Ref:  refImage,
	}
	c.EXPECT().Solve(gomock.Any(), gomock.Any()).Return(resImg, nil)

	imgConfig := `{"author":"zbuild","architecture":"","os":"","rootfs":{"type":"","diff_ids":null},"config":{}}`
	return buildTC{
		client:   c,
		solver:   solver,
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

func initBuildFromGitContextTC(t *testing.T, mockCtrl *gomock.Controller) buildTC {
	c := llbtest.NewMockClient(mockCtrl)
	c.EXPECT().BuildOpts().AnyTimes().Return(client.BuildOpts{
		SessionID: "<SESSION-ID>",
		Opts: map[string]string{
			"context": "git://github.com/some/repo",
		},
	})

	zbuildYml := loadRawTestdata(t, "testdata/build/zbuild.yml")
	zbuildLock := loadRawTestdata(t, "testdata/build/zbuild.lock")

	solver := mocks.NewMockStateSolver(mockCtrl)
	solver.EXPECT().FromContext(gomock.Any(), gomock.Any()).Times(1)

	solver.EXPECT().ReadFile(
		gomock.Any(), "zbuild.yml", gomock.Any(),
	).Return(zbuildYml, nil)

	solver.EXPECT().ReadFile(
		gomock.Any(), "zbuild.lock", gomock.Any(),
	).Return(zbuildLock, nil)

	ctx := context.TODO()
	buildOpts := builddef.BuildOpts{
		File:      "zbuild.yml",
		LockFile:  "zbuild.lock",
		Stage:     "dev",
		SessionID: "<SESSION-ID>",
		BuildContext: &builddef.Context{
			Source: "git://github.com/some/repo",
			Type:   builddef.ContextTypeGit,
		},
	}
	state := llb.State{}
	img := image.Image{
		Image: specs.Image{
			Author: "zbuild",
		},
	}
	handler := mocks.NewMockKindHandler(mockCtrl)
	handler.EXPECT().WithSolver(gomock.Any()).Times(1)
	handler.EXPECT().Build(
		ctx, MatchBuildOpts(buildOpts),
	).Return(state, &img, nil)

	registry := registry.NewKindRegistry()
	registry.Register("php", handler, false)

	refImage := llbtest.NewMockReference(mockCtrl)
	resImg := &client.Result{
		Refs: map[string]client.Reference{"linux/amd64": refImage},
		Ref:  refImage,
	}
	c.EXPECT().Solve(gomock.Any(), gomock.Any()).Return(resImg, nil)

	imgConfig := `{"author":"zbuild","architecture":"","os":"","rootfs":{"type":"","diff_ids":null},"config":{}}`
	return buildTC{
		client:   c,
		solver:   solver,
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

func initBuildCustomStageAndFileTC(t *testing.T, mockCtrl *gomock.Controller) buildTC {
	c := llbtest.NewMockClient(mockCtrl)
	c.EXPECT().BuildOpts().AnyTimes().Return(client.BuildOpts{
		SessionID: "<SESSION-ID>",
		Opts: map[string]string{
			"filename": "api.zbuild.yml",
			"target":   "prod",
		},
	})

	zbuildYml := loadRawTestdata(t, "testdata/build/zbuild.yml")
	zbuildLock := loadRawTestdata(t, "testdata/build/zbuild.lock")

	solver := mocks.NewMockStateSolver(mockCtrl)
	solver.EXPECT().FromContext(gomock.Any(), gomock.Any()).Times(1)

	solver.EXPECT().ReadFile(
		gomock.Any(), "api.zbuild.yml", gomock.Any(),
	).Return(zbuildYml, nil)

	solver.EXPECT().ReadFile(
		gomock.Any(), "api.zbuild.lock", gomock.Any(),
	).Return(zbuildLock, nil)

	refImage := llbtest.NewMockReference(mockCtrl)
	resImg := &client.Result{
		Refs: map[string]client.Reference{"linux/amd64": refImage},
		Ref:  refImage,
	}
	c.EXPECT().Solve(gomock.Any(), gomock.Any()).Return(resImg, nil)

	ctx := context.TODO()
	buildOpts := builddef.BuildOpts{
		File:      "api.zbuild.yml",
		LockFile:  "api.zbuild.lock",
		Stage:     "prod",
		SessionID: "<SESSION-ID>",
		BuildContext: &builddef.Context{
			Source: "context",
			Type:   builddef.ContextTypeLocal,
		},
	}
	state := llb.State{}
	img := image.Image{
		Image: specs.Image{
			Author: "zbuild",
		},
	}
	handler := mocks.NewMockKindHandler(mockCtrl)
	handler.EXPECT().WithSolver(gomock.Any()).Times(1)
	handler.EXPECT().Build(
		ctx, MatchBuildOpts(buildOpts),
	).Return(state, &img, nil)

	registry := registry.NewKindRegistry()
	registry.Register("php", handler, false)

	imgConfig := `{"author":"zbuild","architecture":"","os":"","rootfs":{"type":"","diff_ids":null},"config":{}}`
	return buildTC{
		client:   c,
		solver:   solver,
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

func initBuildWebserverStageTC(t *testing.T, mockCtrl *gomock.Controller) buildTC {
	c := llbtest.NewMockClient(mockCtrl)
	c.EXPECT().BuildOpts().AnyTimes().Return(client.BuildOpts{
		SessionID: "<SESSION-ID>",
		Opts: map[string]string{
			"filename": "api.zbuild.yml",
			"target":   "webserver-prod",
		},
	})

	zbuildYml := loadRawTestdata(t, "testdata/build/zbuild.yml")
	zbuildLock := loadRawTestdata(t, "testdata/build/zbuild.lock")

	solver := mocks.NewMockStateSolver(mockCtrl)
	solver.EXPECT().FromContext(gomock.Any(), gomock.Any()).Times(1)

	solver.EXPECT().ReadFile(
		gomock.Any(), "api.zbuild.yml", gomock.Any(),
	).Return(zbuildYml, nil)

	solver.EXPECT().ReadFile(
		gomock.Any(), "api.zbuild.lock", gomock.Any(),
	).Return(zbuildLock, nil)

	refImage := llbtest.NewMockReference(mockCtrl)
	resImg := &client.Result{
		Refs: map[string]client.Reference{"linux/amd64": refImage},
		Ref:  refImage,
	}
	c.EXPECT().Solve(gomock.Any(), gomock.Any()).Return(resImg, nil)

	ctx := context.TODO()
	state := llb.State{}
	img := image.Image{Image: specs.Image{Author: "zbuild"}}
	phpHandler := mocks.NewMockKindHandler(mockCtrl)
	phpHandler.EXPECT().WithSolver(gomock.Any()).Times(1)
	phpHandler.EXPECT().Build(ctx,
		MatchBuildOpts(builddef.BuildOpts{
			File:      "api.zbuild.yml",
			LockFile:  "api.zbuild.lock",
			Stage:     "prod",
			SessionID: "<SESSION-ID>",
			BuildContext: &builddef.Context{
				Source: "context",
				Type:   builddef.ContextTypeLocal,
			},
		}),
	).Return(state, &img, nil)

	webHandler := mocks.NewMockKindHandler(mockCtrl)
	webHandler.EXPECT().WithSolver(gomock.Any()).Times(1)
	webHandler.EXPECT().Build(ctx,
		MatchBuildOpts(builddef.BuildOpts{
			File:        "api.zbuild.yml",
			LockFile:    "api.zbuild.lock",
			Stage:       "webserver",
			SessionID:   "<SESSION-ID>",
			SourceState: &llb.State{},
			BuildContext: &builddef.Context{
				Source: "context",
				Type:   builddef.ContextTypeLocal,
			},
		}),
	).Return(state, &img, nil)

	registry := registry.NewKindRegistry()
	registry.Register("php", phpHandler, true)
	registry.Register("webserver", webHandler, false)

	imgConfig := `{"author":"zbuild","architecture":"","os":"","rootfs":{"type":"","diff_ids":null},"config":{}}`
	return buildTC{
		client:   c,
		solver:   solver,
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

func failToReadYmlTC(t *testing.T, mockCtrl *gomock.Controller) buildTC {
	c := llbtest.NewMockClient(mockCtrl)
	c.EXPECT().BuildOpts().AnyTimes().Return(client.BuildOpts{
		SessionID: "<SESSION-ID>",
		Opts:      map[string]string{},
	})

	solver := mocks.NewMockStateSolver(mockCtrl)
	solver.EXPECT().FromContext(gomock.Any(), gomock.Any()).Times(1)

	solver.EXPECT().ReadFile(
		gomock.Any(), "zbuild.yml", gomock.Any(),
	).Return([]byte{}, statesolver.FileNotFound)

	return buildTC{
		client:      c,
		solver:      solver,
		registry:    registry.NewKindRegistry(),
		expectedErr: errors.New("zbuildfile not found"),
	}
}

func failToFindASutableKindHandlerTC(t *testing.T, mockCtrl *gomock.Controller) buildTC {
	c := llbtest.NewMockClient(mockCtrl)
	c.EXPECT().BuildOpts().AnyTimes().Return(client.BuildOpts{
		SessionID: "<SESSION-ID>",
		Opts: map[string]string{
			"context": "some-context-name",
		},
	})

	zbuildYml := loadRawTestdata(t, "testdata/build/zbuild.yml")
	zbuildLock := loadRawTestdata(t, "testdata/build/zbuild.lock")

	solver := mocks.NewMockStateSolver(mockCtrl)
	solver.EXPECT().FromContext(gomock.Any(), gomock.Any()).Times(1)

	solver.EXPECT().ReadFile(
		gomock.Any(), "zbuild.yml", gomock.Any(),
	).Return(zbuildYml, nil)

	solver.EXPECT().ReadFile(
		gomock.Any(), "zbuild.lock", gomock.Any(),
	).Return(zbuildLock, nil)

	handler := mocks.NewMockKindHandler(mockCtrl)
	registry := registry.NewKindRegistry()
	registry.Register("notphp", handler, false)

	return buildTC{
		client:      c,
		solver:      solver,
		registry:    registry,
		expectedErr: errors.New("kind \"php\" is not supported: unknown kind"),
	}
}

func failWhenKindHandlerFailsTC(t *testing.T, mockCtrl *gomock.Controller) buildTC {
	c := llbtest.NewMockClient(mockCtrl)
	c.EXPECT().BuildOpts().AnyTimes().Return(client.BuildOpts{
		SessionID: "<SESSION-ID>",
		Opts: map[string]string{
			"context": "some-context-name",
		},
	})

	zbuildYml := loadRawTestdata(t, "testdata/build/zbuild.yml")
	zbuildLock := loadRawTestdata(t, "testdata/build/zbuild.lock")

	solver := mocks.NewMockStateSolver(mockCtrl)
	solver.EXPECT().FromContext(gomock.Any(), gomock.Any()).Times(1)

	solver.EXPECT().ReadFile(
		gomock.Any(), "zbuild.yml", gomock.Any(),
	).Return(zbuildYml, nil)

	solver.EXPECT().ReadFile(
		gomock.Any(), "zbuild.lock", gomock.Any(),
	).Return(zbuildLock, nil)

	handler := mocks.NewMockKindHandler(mockCtrl)
	handler.EXPECT().WithSolver(gomock.Any()).Times(1)

	state := llb.State{}
	img := image.Image{}
	err := xerrors.New("some build error")
	handler.EXPECT().Build(gomock.Any(), gomock.Any()).Return(state, &img, err)

	registry := registry.NewKindRegistry()
	registry.Register("php", handler, false)

	return buildTC{
		client:      c,
		solver:      solver,
		registry:    registry,
		expectedErr: errors.New("some build error"),
	}
}

func failWhenLockfileIsOutOfSyncTC(t *testing.T, mockCtrl *gomock.Controller) buildTC {
	c := llbtest.NewMockClient(mockCtrl)
	c.EXPECT().BuildOpts().AnyTimes().Return(client.BuildOpts{
		SessionID: "<SESSION-ID>",
		Opts: map[string]string{
			"context": "some-context-name",
		},
	})

	zbuildYml := loadRawTestdata(t, "testdata/build/out-of-sync.yml")
	zbuildLock := loadRawTestdata(t, "testdata/build/out-of-sync.lock")

	solver := mocks.NewMockStateSolver(mockCtrl)
	solver.EXPECT().FromContext(gomock.Any(), gomock.Any()).Times(1)

	solver.EXPECT().ReadFile(
		gomock.Any(), "zbuild.yml", gomock.Any(),
	).Return(zbuildYml, nil)

	solver.EXPECT().ReadFile(
		gomock.Any(), "zbuild.lock", gomock.Any(),
	).Return(zbuildLock, nil)

	return buildTC{
		client:      c,
		solver:      solver,
		registry:    registry.NewKindRegistry(),
		expectedErr: builder.OutOfSyncLockfileError{},
	}
}

func MatchBuildOpts(expected builddef.BuildOpts) buildOptsMatcher {
	return buildOptsMatcher{expected}
}

type buildOptsMatcher struct {
	opts builddef.BuildOpts
}

func (m buildOptsMatcher) Matches(x interface{}) bool {
	diff := deep.Equal(m, x)
	return len(diff) > 0
}

func (m buildOptsMatcher) String() string {
	return fmt.Sprintf("{%s %s %s %s %s}",
		m.opts.File,
		m.opts.LockFile,
		m.opts.Stage,
		m.opts.SessionID,
		m.opts.BuildContext)
}

func loadRawTestdata(t *testing.T, filepath string) []byte {
	buf, err := ioutil.ReadFile(filepath)
	if err != nil {
		t.Fatal(err)
	}
	return buf
}

type updateLocksTC struct {
	builder      builder.Builder
	solver       statesolver.StateSolver
	zbuildfile   string
	lockfile     string
	lockfileVfst string
	expectedErr  error
}

func initUpdateLockfileTC(t *testing.T, mockCtrl *gomock.Controller) updateLocksTC {
	zbuildfile := "testdata/lock/zbuild.yml"
	lockfile := "testdata/lock/zbuild.lock"
	lockfileVfst := lockfile
	if !*flagTestdata {
		lockfileVfst = "/" + lockfileVfst
	}

	solver := mocks.NewMockStateSolver(mockCtrl)
	solver.EXPECT().FromContext(gomock.Any(), gomock.Any()).Times(1)

	zbuildYml := loadRawTestdata(t, zbuildfile)
	solver.EXPECT().ReadFile(
		gomock.Any(), zbuildfile, gomock.Any(),
	).Return(zbuildYml, nil)

	zbuildLock := loadRawTestdata(t, lockfile)
	solver.EXPECT().ReadFile(
		gomock.Any(), lockfileVfst, gomock.Any(),
	).Return(zbuildLock, nil)

	registry := registry.NewKindRegistry()
	handler := mocks.NewMockKindHandler(mockCtrl)
	handler.EXPECT().WithSolver(gomock.Any())
	registry.Register("webserver", handler, false)

	locks := stubLocks{map[string]interface{}{
		"foo": "bar",
	}}
	handler.EXPECT().UpdateLocks(
		gomock.Any(), gomock.Any(), gomock.Any(),
	).Return(locks, nil)

	return updateLocksTC{
		builder: builder.Builder{
			Registry: registry,
		},
		solver:       solver,
		zbuildfile:   zbuildfile,
		lockfile:     lockfile,
		lockfileVfst: lockfileVfst,
	}
}

func TestBuilderUpdateLocks(t *testing.T) {
	testcases := map[string]func(*testing.T, *gomock.Controller) updateLocksTC{
		"update lockfile": initUpdateLockfileTC,
	}

	for tcname := range testcases {
		tcinit := testcases[tcname]

		t.Run(tcname, func(t *testing.T) {
			t.Parallel()

			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()
			tc := tcinit(t, mockCtrl)

			var fs vfs.FS
			var cleanup func()

			// When tests are running with -testdata flag, a concrete
			// filesystem implementation is used. As such, the lockfile is
			// written by the Builder, instead of being handled here as it's
			// done for other test functions.
			if *flagTestdata {
				fs = vfs.OSFS
				cleanup = func() {}
			} else {
				var err error
				fs, cleanup, err = vfst.NewTestFS(map[string]interface{}{
					"/testdata/lock": &vfst.Dir{
						Perm: 0777,
						Entries: map[string]interface{}{
							"zbuild.lock": "",
						},
					},
				})
				if err != nil {
					t.Fatal(err)
				}
			}
			tc.builder.Filesystem = fs
			defer cleanup()

			opts := builddef.UpdateLocksOpts{
				BuildOpts: &builddef.BuildOpts{
					File:     tc.zbuildfile,
					LockFile: tc.lockfileVfst,
				},
			}
			err := tc.builder.UpdateLockFile(tc.solver, opts)
			if tc.expectedErr != nil {
				if err.Error() != tc.expectedErr.Error() {
					t.Fatalf("Expected err: %v\nGot: %v", tc.expectedErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if *flagTestdata {
				return
			}

			vfst.RunTests(t, fs, "lockfile",
				vfst.TestPath(tc.lockfileVfst,
					vfst.TestContents(loadRawTestdata(t, tc.lockfile)),
				),
			)
		})
	}
}

// stubLocks implements builddef.RawLocks
type stubLocks struct {
	locks map[string]interface{}
}

func (l stubLocks) RawLocks() map[string]interface{} {
	return l.locks
}
