package llbgraph_test

import (
	"flag"
	"io/ioutil"
	"testing"

	"github.com/NiR-/zbuild/pkg/llbgraph"
	"github.com/NiR-/zbuild/pkg/llbutils"
	"github.com/moby/buildkit/client/llb"
)

var flagTestdata = flag.Bool("testdata", false, "Use this flag to (re)generate testdata (dumps of LLB states)")

type TC struct {
	deffile     string
	expected    string
	expectedErr error
}

func TestStateToDotGraph(t *testing.T) {
	testcases := map[string]TC{
		"transform Op source": {
			deffile:  "testdata/op-source.json",
			expected: "testdata/op-source.dot",
		},
		"transform Op file": {
			deffile:  "testdata/op-file.json",
			expected: "testdata/op-file.dot",
		},
		"transform Op exec": {
			deffile:  "testdata/op-exec.json",
			expected: "testdata/op-exec.dot",
		},
		"transform nodejs webserver-prod into dotgraph": {
			deffile:  "testdata/nodejs-webserver-prod.json",
			expected: "testdata/nodejs-webserver-prod.dot",
		},
		"transform nodejs prod into dotgraph": {
			deffile:  "testdata/nodejs-prod.json",
			expected: "testdata/nodejs-prod.dot",
		},
	}

	for tcname := range testcases {
		tc := testcases[tcname]
		t.Run(tcname, func(t *testing.T) {
			t.Parallel()

			def := loadDefFile(t, tc.deffile)
			opts := llbgraph.GraphOpts{}

			g, err := llbgraph.ToDotGraph(def, opts)
			if tc.expectedErr != nil {
				if err == nil || err.Error() != tc.expectedErr.Error() {
					t.Fatalf("Expected error: %+v\nGot: %+v", tc.expectedErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Unexpected error: %+v", err)
			}

			dotgraph, err := g.MarshalText()
			if err != nil {
				t.Fatal(err)
			}

			if *flagTestdata {
				writeRawTestdata(t, tc.expected, dotgraph)
			}

			expected := loadRawTestdata(t, tc.expected)
			if string(dotgraph) != string(expected) {
				t.Fatalf("Expected: %s\nGot: %s", string(expected), string(dotgraph))
			}
		})
	}
}

func writeRawTestdata(t *testing.T, path string, raw []byte) {
	if err := ioutil.WriteFile(path, raw, 0640); err != nil {
		t.Fatal(err)
	}
}

func loadRawTestdata(t *testing.T, path string) []byte {
	raw, err := ioutil.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func loadDefFile(t *testing.T, defpath string) *llb.Definition {
	raw := loadRawTestdata(t, defpath)
	def, err := llbutils.JSONToDefinition(raw)
	if err != nil {
		t.Fatal(err)
	}
	return def
}
