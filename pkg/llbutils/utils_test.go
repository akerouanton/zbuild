package llbutils_test

import (
	"context"
	"errors"
	"flag"
	"io/ioutil"
	"os"
	"testing"

	"github.com/NiR-/zbuild/pkg/llbtest"
	"github.com/NiR-/zbuild/pkg/llbutils"
	"github.com/go-test/deep"
	"github.com/golang/mock/gomock"
	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/frontend/gateway/client"
)

type solveStateTC struct {
	client      client.Client
	state       llb.State
	expectedRes *client.Result
	expectedRef client.Reference
	expectedErr error
}

func initSuccessfullySolveStateTC(mockCtrl *gomock.Controller) solveStateTC {
	ref := llbtest.NewMockReference(mockCtrl)
	res := &client.Result{
		Refs: map[string]client.Reference{"linux/amd64": ref},
		Ref:  ref,
	}

	c := llbtest.NewMockClient(mockCtrl)
	c.EXPECT().Solve(gomock.Any(), gomock.Any()).Return(res, nil)

	return solveStateTC{
		client:      c,
		state:       llb.State{},
		expectedRes: res,
		expectedRef: ref,
	}
}

func initReturnsAnErrorWhenSolveFailsTC(mockCtrl *gomock.Controller) solveStateTC {
	c := llbtest.NewMockClient(mockCtrl)
	c.EXPECT().Solve(gomock.Any(), gomock.Any()).Return(nil, errors.New("some error"))

	return solveStateTC{
		client:      c,
		state:       llb.State{},
		expectedErr: errors.New("some error"),
	}
}

func initReturnsAnErrorWhenResultAsNoSingleRefTC(mockCtrl *gomock.Controller) solveStateTC {
	ref := llbtest.NewMockReference(mockCtrl)
	res := &client.Result{
		Refs: map[string]client.Reference{"linux/amd64": ref},
	}

	c := llbtest.NewMockClient(mockCtrl)
	c.EXPECT().Solve(gomock.Any(), gomock.Any()).Return(res, nil)

	return solveStateTC{
		client:      c,
		state:       llb.State{},
		expectedErr: errors.New("failed to get a single ref for source: invalid map result"),
	}
}

func TestSolveState(t *testing.T) {
	if *flagTestdata {
		return
	}

	testcases := map[string]func(*gomock.Controller) solveStateTC{
		"successfully solve state":                      initSuccessfullySolveStateTC,
		"returns an error when solve fails":             initReturnsAnErrorWhenSolveFailsTC,
		"returns an error when result as no single ref": initReturnsAnErrorWhenResultAsNoSingleRefTC,
	}

	for tcname := range testcases {
		tcinit := testcases[tcname]

		t.Run(tcname, func(t *testing.T) {
			t.Parallel()

			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			tc := tcinit(mockCtrl)

			ctx := context.TODO()
			outRes, outRef, outErr := llbutils.SolveState(ctx, tc.client, tc.state)

			if tc.expectedErr != nil {
				if outErr == nil || outErr.Error() != tc.expectedErr.Error() {
					t.Fatalf("Expected: %v\nGot: %v", tc.expectedErr, outErr)
				}
				return
			}
			if diff := deep.Equal(outRef, tc.expectedRef); diff != nil {
				t.Fatal(diff)
			}
			if diff := deep.Equal(outRes, tc.expectedRes); diff != nil {
				t.Fatal(diff)
			}
		})
	}
}

type readFileTC struct {
	filepath    string
	ref         client.Reference
	found       bool
	expected    []byte
	expectedErr error
}

func initSuccessfullyReadFileContentTC(mockCtrl *gomock.Controller) readFileTC {
	filepath := "some/file.yml"
	expected := []byte("some file content")

	ref := llbtest.NewMockReference(mockCtrl)
	ref.EXPECT().ReadFile(gomock.Any(), client.ReadRequest{
		Filename: filepath,
	}).Return(expected, nil)

	return readFileTC{
		filepath: filepath,
		ref:      ref,
		found:    true,
		expected: expected,
	}
}

func initReturnsNoErrorsWhenFileNotFoundTC(mockCtrl *gomock.Controller) readFileTC {
	filepath := "some/file.yml"

	ref := llbtest.NewMockReference(mockCtrl)
	ref.EXPECT().ReadFile(gomock.Any(), gomock.Any()).Return([]byte{}, os.ErrNotExist)

	return readFileTC{
		filepath: filepath,
		ref:      ref,
		found:    false,
	}
}

func TestReadFile(t *testing.T) {
	if *flagTestdata {
		return
	}

	testcases := map[string]func(*gomock.Controller) readFileTC{
		"successfully read file content":        initSuccessfullyReadFileContentTC,
		"returns no errors when file not found": initReturnsNoErrorsWhenFileNotFoundTC,
	}

	for tcname := range testcases {
		tcinit := testcases[tcname]

		t.Run(tcname, func(t *testing.T) {
			t.Parallel()

			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			tc := tcinit(mockCtrl)

			ctx := context.TODO()
			out, ok, err := llbutils.ReadFile(ctx, tc.ref, tc.filepath)

			if tc.expectedErr != nil {
				if err == nil || err.Error() != tc.expectedErr.Error() {
					t.Fatalf("Expected: %v\nGot: %v", tc.expectedErr, err)
				}
				return
			}
			if tc.found != ok {
				t.Fatalf("Expected found: %t\nGot: %t", tc.found, ok)
			}
			if string(tc.expected) != string(out) {
				t.Fatalf("Expected content: %s\nGot: %s", string(tc.expected), string(out))
			}
		})
	}
}

var flagTestdata = flag.Bool("testdata", false, "Use this flag to (re)generate testdata (dumps of LLB states)")

func TestStateHelpers(t *testing.T) {
	testcases := map[string]struct {
		testdata string
		init     func(*testing.T) llb.State
	}{
		"ImageSource": {
			testdata: "testdata/image-source.json",
			init: func(_ *testing.T) llb.State {
				return llbutils.ImageSource("php:7.2", true)
			},
		},
		"Mkdir": {
			testdata: "testdata/mkdir.json",
			init: func(_ *testing.T) llb.State {
				state := llbutils.ImageSource("php:7.2", false)
				return llbutils.Mkdir(state, "1000:1000", "/app", "/usr/src/app")
			},
		},
		"Copy": {
			testdata: "testdata/copy.json",
			init: func(_ *testing.T) llb.State {
				src := llbutils.ImageSource("php:7.2", false)
				dest := llb.Scratch()
				return llbutils.Copy(src, "/etc/passwd", dest, "/etc/passwd2", "1000:1000")
			},
		},
		"InstallSystemPackages": {
			testdata: "testdata/install-system-packages.json",
			init: func(t *testing.T) llb.State {
				dest := llbutils.ImageSource("php:7.2", false)
				locks := map[string]string{
					"curl":            "curl-version",
					"ca-certficiates": "ca-certificates-version",
					"zlib1g-dev":      "zlib1g-dev-version",
				}
				state, err := llbutils.InstallSystemPackages(dest, llbutils.APT, locks)
				if err != nil {
					t.Fatal(err)
				}
				return state
			},
		},
		"CopyExternalFiles": {
			testdata: "testdata/copy-external-files.json",
			init: func(_ *testing.T) llb.State {
				dest := llb.Scratch()
				externalFiles := []llbutils.ExternalFile{
					{
						URL:         "https://blackfire.io/api/v1/releases/probe/php/linux/amd64/72",
						Compressed:  true,
						Pattern:     "blackfire-*.so",
						Destination: "/some/path",
						Mode:        0644,
					},
					llbutils.ExternalFile{
						URL:         "https://github.com/NiR-/fcgi-client/releases/download/v0.1.0/fcgi-client.phar",
						Destination: "/usr/local/bin/fcgi-client",
						Mode:        0750,
						Owner:       "1000:1000",
					},
					llbutils.ExternalFile{
						URL:         "https://github.com/NiR-/fcgi-client/releases/download/v0.2.0/fcgi-client.phar",
						Checksum:    "some-checksum",
						Destination: "/usr/local/bin/fcgi-client-0.2",
						Mode:        0750,
						Owner:       "1000:1000",
					},
				}
				return llbutils.CopyExternalFiles(dest, externalFiles)
			},
		},
	}

	for tcname := range testcases {
		tc := testcases[tcname]

		t.Run(tcname, func(t *testing.T) {
			t.Parallel()

			state := tc.init(t)
			jsonState := llbtest.StateToJSON(t, state)

			if *flagTestdata {
				writeTestdata(t, tc.testdata, jsonState)
				return
			}

			testdata := loadTestdata(t, tc.testdata)
			if diff := deep.Equal(jsonState, testdata); diff != nil {
				t.Fatal(diff)
			}
		})
	}
}

func writeTestdata(t *testing.T, filepath string, content string) {
	err := ioutil.WriteFile(filepath, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Could not write %q: %v", filepath, err)
	}
}

func loadTestdata(t *testing.T, filepath string) string {
	out, err := ioutil.ReadFile(filepath)
	if err != nil {
		t.Fatalf("Could not load %q: %v", filepath, err)
	}
	return string(out)
}
