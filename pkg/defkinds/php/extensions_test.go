package php_test

import (
	"flag"
	"io/ioutil"
	"testing"

	"github.com/NiR-/zbuild/pkg/defkinds/php"
	"github.com/NiR-/zbuild/pkg/llbtest"
	"github.com/go-test/deep"
	"github.com/moby/buildkit/client/llb"
)

type installExtensionsTC struct {
	extensions map[string]string
	testdata   string
}

func initInstallGDWithAllFormatsTC(t *testing.T) installExtensionsTC {
	return installExtensionsTC{
		extensions: map[string]string{
			"gd":          "*",
			"gd.freetype": "*",
			"gd.jpeg":     "*",
			"gd.webp":     "*",
		},
		testdata: "testdata/extensions/gd-with-all-formats.json",
	}
}

func initInstallPeclExtensionsTC(t *testing.T) installExtensionsTC {
	return installExtensionsTC{
		extensions: map[string]string{
			"redis": "*",
		},
		testdata: "testdata/extensions/pecl-extensions.json",
	}
}

var flagTestdata = flag.Bool("testdata", false, "Use this flag to (re)generate testdata (dumps of LLB states)")

func TestInstallExtensions(t *testing.T) {
	testcases := map[string]func(t *testing.T) installExtensionsTC{
		"successfully install gd extension with all supported formats": initInstallGDWithAllFormatsTC,
		"successfully install pecl extensions":                         initInstallPeclExtensionsTC,
	}

	for tcname := range testcases {
		tcinit := testcases[tcname]

		t.Run(tcname, func(t *testing.T) {
			t.Parallel()

			tc := tcinit(t)

			state := llb.Scratch()
			res := php.InstallExtensions(state, tc.extensions)
			jsonState := llbtest.StateToJSON(t, res)

			if *flagTestdata {
				writeTestdata(t, tc.testdata, jsonState)
				return
			}

			testdata := loadTestdata(t, tc.testdata)
			if diff := deep.Equal(testdata, jsonState); diff != nil {
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
