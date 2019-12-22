package nodejs_test

import (
	"errors"
	"io/ioutil"
	"strings"
	"testing"

	"github.com/NiR-/zbuild/pkg/builddef"
	"github.com/NiR-/zbuild/pkg/defkinds/nodejs"
	"github.com/NiR-/zbuild/pkg/llbutils"
	"github.com/go-test/deep"
	"github.com/golang/mock/gomock"
	"gopkg.in/yaml.v2"
)

type newDefinitionTC struct {
	file        string
	expected    nodejs.Definition
	expectedErr error
}

func initSuccessfullyParseRawDefinitionWithoutStagesTC() newDefinitionTC {
	devStageDevMode := true
	prodStageDevMode := false
	healthcheckDisabled := true

	return newDefinitionTC{
		file: "testdata/def/without-stages.yml",
		expected: nodejs.Definition{
			BaseStage: nodejs.Stage{
				ExternalFiles: []llbutils.ExternalFile{
					{
						URL:         "https://github.com/some/tool",
						Compressed:  true,
						Destination: "/usr/sbin/tool1",
						Checksum:    "some-checksum",
						Mode:        0640,
						// @TODO: this should be set automatically by the handler when not specified.
						Owner: "1000:1000",
					},
				},
				SystemPackages: map[string]string{
					"ca-certificates": "*",
				},
				ConfigFiles: map[string]string{
					".babelrc": ".babelrc",
				},
				SourceDirs:   []string{"src/"},
				StatefulDirs: []string{"uploads/"},
				Healthcheck:  &healthcheckDisabled,
				PostInstall:  []string{},
			},
			Version:    "12",
			BaseImage:  "docker.io/library/node:12-buster-slim",
			IsFrontend: true,
			Stages: map[string]nodejs.DerivedStage{
				"dev": {
					DeriveFrom: "base",
					Dev:        &devStageDevMode,
					Stage:      nodejs.Stage{},
				},
				"prod": {
					DeriveFrom: "base",
					Dev:        &prodStageDevMode,
					Stage:      nodejs.Stage{},
				},
			},
		},
	}
}

func initSuccessfullyParseRawDefinitionWithStagesTC() newDefinitionTC {
	cmdDev := []string{"yarn run start-dev"}
	cmdProd := []string{"yarn run start"}
	cmdWorker := []string{"yarn run worker"}
	devStageDevMode := true
	prodStageDevMode := false
	baseStageHealthcheck := true
	workerStageHealthcheck := false

	return newDefinitionTC{
		file: "testdata/def/with-stages.yml",
		expected: nodejs.Definition{
			BaseStage: nodejs.Stage{
				ExternalFiles:  []llbutils.ExternalFile{},
				SystemPackages: map[string]string{},
				ConfigFiles:    map[string]string{},
				Healthcheck:    &baseStageHealthcheck,
				PostInstall:    []string{},
			},
			Version:   "12",
			BaseImage: "docker.io/library/node:12-buster-slim",
			Stages: map[string]nodejs.DerivedStage{
				"dev": {
					Dev: &devStageDevMode,
					Stage: nodejs.Stage{
						Command: &cmdDev,
					},
				},
				"prod": {
					Dev: &prodStageDevMode,
					Stage: nodejs.Stage{
						Command: &cmdProd,
					},
				},
				"worker": {
					DeriveFrom: "prod",
					Stage: nodejs.Stage{
						Command:     &cmdWorker,
						Healthcheck: &workerStageHealthcheck,
					},
				},
			},
		},
	}
}

func initFailToParseUnknownPropertiesTC() newDefinitionTC {
	return newDefinitionTC{
		file:        "testdata/def/invalid.yml",
		expectedErr: errors.New("could not decode build manifest: 1 error(s) decoding:\n\n* '' has invalid keys: foo"),
	}
}

func initFailWhenBothVersionAndBaseImageAreDefinedTC() newDefinitionTC {
	return newDefinitionTC{
		file:        "testdata/def/with-version-and-base-image.yml",
		expectedErr: errors.New("you can't provide both version and base image parameters at the same time"),
	}
}

func TestNewKind(t *testing.T) {
	if *flagTestdata {
		return
	}

	testcases := map[string]func() newDefinitionTC{
		"successfully parse raw definition without stages":               initSuccessfullyParseRawDefinitionWithoutStagesTC,
		"successfully parse raw definition with stages":                  initSuccessfullyParseRawDefinitionWithStagesTC,
		"fail to parse unknown properties":                               initFailToParseUnknownPropertiesTC,
		"fail to load zbuildfile with both version and base image props": initFailWhenBothVersionAndBaseImageAreDefinedTC,
	}

	for tcname := range testcases {
		tcinit := testcases[tcname]

		t.Run(tcname, func(t *testing.T) {
			t.Parallel()
			tc := tcinit()

			generic := loadBuildDef(t, tc.file)
			def, err := nodejs.NewKind(generic)
			if tc.expectedErr != nil {
				if err == nil || strings.Trim(tc.expectedErr.Error(), " ") != strings.Trim(err.Error(), " ") {
					t.Fatalf("Expected: %v\nGot: %v", tc.expectedErr, err)
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

type resolveStageTC struct {
	file        string
	lockFile    string
	stage       string
	expected    nodejs.StageDefinition
	expectedErr error
}

func initSuccessfullyResolveDefaultDevStageTC() resolveStageTC {
	devMode := true
	healthckeck := false

	return resolveStageTC{
		file: "testdata/def/without-stages.yml",
		// lockFile: "testdata/def/without-stages.lock",
		stage: "dev",
		expected: nodejs.StageDefinition{
			Name:       "dev",
			BaseImage:  "docker.io/library/node:12-buster-slim",
			Version:    "12",
			Dev:        &devMode,
			IsFrontend: true,
			Stage: nodejs.Stage{
				ExternalFiles: []llbutils.ExternalFile{
					{
						URL:         "https://github.com/some/tool",
						Compressed:  true,
						Destination: "/usr/sbin/tool1",
						Checksum:    "some-checksum",
						Mode:        0640,
						Owner:       "1000:1000",
					},
				},
				SystemPackages: map[string]string{
					"ca-certificates": "*",
				},
				SourceDirs:   []string{"src/"},
				StatefulDirs: []string{"uploads/"},
				ConfigFiles:  map[string]string{".babelrc": ".babelrc"},
				Healthcheck:  &healthckeck,
				PostInstall:  []string{},
			},
		},
	}
}

func initSuccessfullyResolveWorkerStageTC() resolveStageTC {
	devMode := false
	healthckeckDisabled := false
	cmd := []string{"yarn run worker"}

	return resolveStageTC{
		file:  "testdata/def/with-stages.yml",
		stage: "worker",
		expected: nodejs.StageDefinition{
			Name:      "worker",
			BaseImage: "docker.io/library/node:12-buster-slim",
			Version:   "12",
			Dev:       &devMode,
			Stage: nodejs.Stage{
				ExternalFiles:  []llbutils.ExternalFile{},
				SystemPackages: map[string]string{},
				ConfigFiles:    map[string]string{},
				Healthcheck:    &healthckeckDisabled,
				PostInstall:    []string{},
				Command:        &cmd,
			},
		},
	}
}

func initFailToResolveUnknownStageTC() resolveStageTC {
	return resolveStageTC{
		file:        "testdata/def/with-stages.yml",
		stage:       "unknown",
		expectedErr: errors.New("stage \"unknown\" not found"),
	}
}

func initFailToResolveStageWithCyclicDepsTC() resolveStageTC {
	return resolveStageTC{
		file:        "testdata/def/cyclic-stage-deps.yml",
		stage:       "dev",
		expectedErr: errors.New(`there's a cyclic dependency between "dev" and itself`),
	}
}

func TestResolveStageDefinition(t *testing.T) {
	if *flagTestdata {
		return
	}

	testcases := map[string]func() resolveStageTC{
		"successfully resolve default dev stage": initSuccessfullyResolveDefaultDevStageTC,
		"successfully resolve worker stage":      initSuccessfullyResolveWorkerStageTC,
		"fail to resolve unknown stage":          initFailToResolveUnknownStageTC,
		"fail to resolve stage with cyclic deps": initFailToResolveStageWithCyclicDepsTC,
	}

	for tcname := range testcases {
		tcinit := testcases[tcname]

		t.Run(tcname, func(t *testing.T) {
			t.Parallel()

			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			tc := tcinit()

			generic := loadBuildDef(t, tc.file)
			if tc.lockFile != "" {
				generic.RawLocks = loadRawTestdata(t, tc.lockFile)
			}

			def, err := nodejs.NewKind(generic)
			if err != nil {
				t.Fatal(err)
			}

			stageDef, err := def.ResolveStageDefinition(tc.stage, false)
			if tc.expectedErr != nil {
				if err == nil || err.Error() != tc.expectedErr.Error() {
					t.Fatalf("Expected: %v\nGot: %v", tc.expectedErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if diff := deep.Equal(stageDef, tc.expected); diff != nil {
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
