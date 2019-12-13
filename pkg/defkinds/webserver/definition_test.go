package webserver_test

import (
	"io/ioutil"
	"testing"

	"github.com/NiR-/zbuild/pkg/builddef"
	"github.com/NiR-/zbuild/pkg/defkinds/webserver"
	"github.com/go-test/deep"
	"gopkg.in/yaml.v2"
)

// var flagTestdata = flag.Bool("testdata", false, "Use this flag to (re)generate testdata (dumps of LLB states)")

type newDefinitionTC struct {
	file        string
	lockFile    string
	expected    webserver.Definition
	expectedErr error
}

func initSuccessfullyParseRawDefinitionTC() newDefinitionTC {
	return newDefinitionTC{
		file:     "testdata/locks/definition.yml",
		lockFile: "testdata/locks/definition.lock",
		expected: webserver.Definition{
			Type:        "nginx",
			ConfigFile:  "./docker/nginx.conf",
			Healthcheck: true,
			SystemPackages: map[string]string{
				"curl": "*",
			},
			Assets: []webserver.AssetToCopy{
				{
					From: "/app/public",
					To:   "/app/public",
				},
			},
			Locks: webserver.DefinitionLocks{
				BaseImage: "docker.io/library/nginx:latest",
				SystemPackages: map[string]string{
					"curl": "7.64.0-4",
				},
			},
		},
	}
}

func TestNewKind(t *testing.T) {
	if *flagTestdata {
		return
	}

	testcases := map[string]func() newDefinitionTC{
		"successfully parse raw definition": initSuccessfullyParseRawDefinitionTC,
	}

	for tcname := range testcases {
		tcinit := testcases[tcname]

		t.Run(tcname, func(t *testing.T) {
			t.Parallel()
			tc := tcinit()

			generic := loadBuildDef(t, tc.file)
			if tc.lockFile != "" {
				generic.RawLocks = loadRawTestdata(t, tc.lockFile)
			}

			def, err := webserver.NewKind(generic)
			if tc.expectedErr != nil {
				if err == nil || err.Error() != tc.expectedErr.Error() {
					t.Fatalf("Expected error: %v\nGot: %v", tc.expectedErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if diff := deep.Equal(def, tc.expected); diff != nil {
				t.Fatal(diff)
			}
		})
	}
}

func loadRawTestdata(t *testing.T, filepath string) []byte {
	raw, err := ioutil.ReadFile(filepath)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func loadBuildDef(t *testing.T, filepath string) *builddef.BuildDef {
	raw := loadRawTestdata(t, filepath)

	var def builddef.BuildDef
	if err := yaml.Unmarshal(raw, &def); err != nil {
		t.Fatal(err)
	}

	return &def
}
