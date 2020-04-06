package php_test

import (
	"flag"
	"io/ioutil"
	"testing"

	"github.com/NiR-/zbuild/pkg/builddef"
	"github.com/NiR-/zbuild/pkg/defkinds/php"
	"github.com/NiR-/zbuild/pkg/llbtest"
	"github.com/go-test/deep"
	"github.com/moby/buildkit/client/llb"
)

type installExtensionsTC struct {
	stageDef php.StageDefinition
	expected string
}

func initInstallGDWithAllFormatsTC(t *testing.T) installExtensionsTC {
	return installExtensionsTC{
		expected: "testdata/extensions/gd-with-all-formats.json",
		stageDef: php.StageDefinition{
			MajMinVersion: "7.2",
			StageLocks: php.StageLocks{
				Extensions: map[string]string{
					"gd":          "*",
					"gd.freetype": "*",
					"gd.jpeg":     "*",
					"gd.webp":     "*",
				},
			},
		},
	}
}

func initInstallPeclExtensionsTC(t *testing.T) installExtensionsTC {
	return installExtensionsTC{
		expected: "testdata/extensions/pecl-extensions.json",
		stageDef: php.StageDefinition{
			MajMinVersion: "7.3",
			StageLocks: php.StageLocks{
				Extensions: map[string]string{
					"redis": "*",
				},
			},
		},
	}
}

func initInstallPeclExtensionsOnAlpineTC(t *testing.T) installExtensionsTC {
	return installExtensionsTC{
		expected: "testdata/extensions/pecl-extensions-for-alpine.json",
		stageDef: php.StageDefinition{
			MajMinVersion: "7.3",
			DefLocks: php.DefinitionLocks{
				OSRelease: builddef.OSRelease{
					Name: "alpine",
				},
			},
			StageLocks: php.StageLocks{
				Extensions: map[string]string{
					"memcached": "*",
				},
			},
		},
	}
}

func initStateDontChangeWhenNoExtToInstallTC(t *testing.T) installExtensionsTC {
	return installExtensionsTC{
		expected: "testdata/extensions/skip-install.json",
		stageDef: php.StageDefinition{
			MajMinVersion: "7.3",
			StageLocks: php.StageLocks{
				Extensions: map[string]string{},
			},
		},
	}
}

var flagTestdata = flag.Bool("testdata", false, "Use this flag to (re)generate testdata (dumps of LLB states)")

func TestInstallExtensions(t *testing.T) {
	testcases := map[string]func(t *testing.T) installExtensionsTC{
		"install gd extension with all supported formats":       initInstallGDWithAllFormatsTC,
		"install pecl extensions":                               initInstallPeclExtensionsTC,
		"install pecl extensions on alpine":                     initInstallPeclExtensionsOnAlpineTC,
		"llb.State don't change when there's no ext to install": initStateDontChangeWhenNoExtToInstallTC,
	}

	for tcname := range testcases {
		tcinit := testcases[tcname]

		t.Run(tcname, func(t *testing.T) {
			t.Parallel()

			tc := tcinit(t)

			state := llb.Scratch()
			res := php.InstallExtensions(tc.stageDef, state, builddef.BuildOpts{})
			jsonState := llbtest.StateToJSON(t, res)

			if *flagTestdata {
				writeTestdata(t, tc.expected, jsonState)
				return
			}

			testdata := loadTestdata(t, tc.expected)
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
