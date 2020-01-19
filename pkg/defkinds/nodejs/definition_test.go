package nodejs_test

import (
	"errors"
	"io/ioutil"
	"strings"
	"testing"
	"time"

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

func initParseRawDefinitionWithoutStagesTC() newDefinitionTC {
	devStageDevMode := true
	prodStageDevMode := false

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
				SystemPackages: &builddef.VersionMap{
					"ca-certificates": "*",
				},
				ConfigFiles: map[string]string{
					".babelrc": ".babelrc",
				},
				GlobalPackages: &builddef.VersionMap{},
				Sources:        []string{"src/"},
				StatefulDirs:   []string{"uploads/"},
				Healthcheck: &builddef.HealthcheckConfig{
					HealthcheckHTTP: &builddef.HealthcheckHTTP{
						Path:     "/ping",
						Expected: "pong",
					},
					Type:     builddef.HealthcheckTypeHTTP,
					Interval: 10 * time.Second,
					Timeout:  1 * time.Second,
					Retries:  3,
				},
			},
			Version:    "12",
			BaseImage:  "docker.io/library/node:12-buster-slim",
			IsFrontend: true,
			Stages: nodejs.DerivedStageSet{
				"dev": {
					DeriveFrom: "base",
					Dev:        &devStageDevMode,
					Stage: nodejs.Stage{
						ExternalFiles:  []llbutils.ExternalFile{},
						SystemPackages: &builddef.VersionMap{},
						GlobalPackages: &builddef.VersionMap{},
						ConfigFiles:    map[string]string{},
						Sources:        []string{},
						StatefulDirs:   []string{},
					},
				},
				"prod": {
					DeriveFrom: "base",
					Dev:        &prodStageDevMode,
					Stage: nodejs.Stage{
						ExternalFiles:  []llbutils.ExternalFile{},
						SystemPackages: &builddef.VersionMap{},
						GlobalPackages: &builddef.VersionMap{},
						ConfigFiles:    map[string]string{},
						Sources:        []string{},
						StatefulDirs:   []string{},
					},
				},
			},
		},
	}
}

func initParseRawDefinitionWithStagesTC() newDefinitionTC {
	cmdDev := []string{"yarn run start-dev"}
	cmdProd := []string{"yarn run start"}
	cmdWorker := []string{"yarn run worker"}
	devStageDevMode := true
	prodStageDevMode := false

	return newDefinitionTC{
		file: "testdata/def/with-stages.yml",
		expected: nodejs.Definition{
			BaseStage: nodejs.Stage{
				ExternalFiles:  []llbutils.ExternalFile{},
				SystemPackages: &builddef.VersionMap{},
				ConfigFiles:    map[string]string{},
				GlobalPackages: &builddef.VersionMap{},
				Healthcheck: &builddef.HealthcheckConfig{
					HealthcheckHTTP: &builddef.HealthcheckHTTP{
						Path:     "/ping",
						Expected: "pong",
					},
					Type:     builddef.HealthcheckTypeHTTP,
					Interval: 10 * time.Second,
					Timeout:  1 * time.Second,
					Retries:  3,
				},
				Sources:      []string{},
				StatefulDirs: []string{},
			},
			Version:   "12",
			BaseImage: "docker.io/library/node:12-buster-slim",
			Stages: nodejs.DerivedStageSet{
				"dev": {
					Dev: &devStageDevMode,
					Stage: nodejs.Stage{
						Command:        &cmdDev,
						ExternalFiles:  []llbutils.ExternalFile{},
						SystemPackages: &builddef.VersionMap{},
						ConfigFiles:    map[string]string{},
						GlobalPackages: &builddef.VersionMap{},
						Sources:        []string{},
						StatefulDirs:   []string{},
					},
				},
				"prod": {
					Dev: &prodStageDevMode,
					Stage: nodejs.Stage{
						Command:        &cmdProd,
						ExternalFiles:  []llbutils.ExternalFile{},
						SystemPackages: &builddef.VersionMap{},
						ConfigFiles:    map[string]string{},
						GlobalPackages: &builddef.VersionMap{},
						Sources:        []string{},
						StatefulDirs:   []string{},
					},
				},
				"worker": {
					DeriveFrom: "prod",
					Stage: nodejs.Stage{
						Command: &cmdWorker,
						Healthcheck: &builddef.HealthcheckConfig{
							Type: builddef.HealthcheckTypeDisabled,
						},
					},
				},
			},
		},
	}
}

func initParseRawDefinitionWithWebserverTC() newDefinitionTC {
	devStageDevMode := true
	prodStageDevMode := false

	baseStage := emptyStage()
	baseStage.Healthcheck = &builddef.HealthcheckConfig{
		HealthcheckHTTP: &builddef.HealthcheckHTTP{
			Path:     "/ping",
			Expected: "pong",
		},
		Type:     builddef.HealthcheckTypeHTTP,
		Interval: 10 * time.Second,
		Timeout:  1 * time.Second,
		Retries:  3,
	}

	return newDefinitionTC{
		file: "testdata/def/with-webserver.yml",
		expected: nodejs.Definition{
			BaseStage:  baseStage,
			Version:    "12",
			BaseImage:  "docker.io/library/node:12-buster-slim",
			IsFrontend: true,
			Stages: nodejs.DerivedStageSet{
				"dev": {
					DeriveFrom: "base",
					Dev:        &devStageDevMode,
					Stage:      emptyStage(),
				},
				"prod": {
					DeriveFrom: "base",
					Dev:        &prodStageDevMode,
					Stage:      emptyStage(),
				},
			},
		},
	}
}

func initParseRawDefinitionWithCustomHealthcheckTC() newDefinitionTC {
	devStageDevMode := true
	prodStageDevMode := false

	return newDefinitionTC{
		file: "testdata/def/with-custom-http-healthcheck.yml",
		expected: nodejs.Definition{
			BaseStage: nodejs.Stage{
				ExternalFiles:  []llbutils.ExternalFile{},
				SystemPackages: &builddef.VersionMap{},
				GlobalPackages: &builddef.VersionMap{},
				ConfigFiles:    map[string]string{},
				Sources:        []string{},
				StatefulDirs:   []string{},
				Healthcheck: &builddef.HealthcheckConfig{
					HealthcheckHTTP: &builddef.HealthcheckHTTP{
						Path:     "/some-custom-path",
						Expected: "some-output",
					},
					Type:     builddef.HealthcheckTypeHTTP,
					Interval: 20 * time.Second,
					Timeout:  5 * time.Second,
					Retries:  6,
				},
			},
			Version:    "12",
			BaseImage:  "docker.io/library/node:12-buster-slim",
			IsFrontend: true,
			Stages: nodejs.DerivedStageSet{
				"dev": {
					DeriveFrom: "base",
					Dev:        &devStageDevMode,
					Stage:      emptyStage(),
				},
				"prod": {
					DeriveFrom: "base",
					Dev:        &prodStageDevMode,
					Stage:      emptyStage(),
				},
			},
		},
	}
}

func initFailToParseUnknownPropertiesTC() newDefinitionTC {
	return newDefinitionTC{
		file:        "testdata/def/invalid.yml",
		expectedErr: errors.New("could not decode build manifest: invalid config parameter: foo"),
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
		"without stages":                   initParseRawDefinitionWithoutStagesTC,
		"with stages":                      initParseRawDefinitionWithStagesTC,
		"with webserver":                   initParseRawDefinitionWithWebserverTC,
		"with custom healthcheck":          initParseRawDefinitionWithCustomHealthcheckTC,
		"fail to parse unknown properties": initFailToParseUnknownPropertiesTC,
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

	return resolveStageTC{
		file:  "testdata/def/without-stages.yml",
		stage: "dev",
		expected: nodejs.StageDefinition{
			Name:       "dev",
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
				SystemPackages: &builddef.VersionMap{
					"ca-certificates": "*",
				},
				GlobalPackages: &builddef.VersionMap{},
				Sources:        []string{"src/"},
				StatefulDirs:   []string{"uploads/"},
				ConfigFiles:    map[string]string{".babelrc": ".babelrc"},
				Healthcheck:    nil,
			},
		},
	}
}

func initSuccessfullyResolveWorkerStageTC() resolveStageTC {
	devMode := false
	cmd := []string{"yarn run worker"}

	return resolveStageTC{
		file:  "testdata/def/with-stages.yml",
		stage: "worker",
		expected: nodejs.StageDefinition{
			Name:    "worker",
			Version: "12",
			Dev:     &devMode,
			Stage: nodejs.Stage{
				ExternalFiles:  []llbutils.ExternalFile{},
				SystemPackages: &builddef.VersionMap{},
				ConfigFiles:    map[string]string{},
				GlobalPackages: &builddef.VersionMap{},
				Sources:        []string{},
				StatefulDirs:   []string{},
				Command:        &cmd,
				Healthcheck: &builddef.HealthcheckConfig{
					Type: builddef.HealthcheckTypeDisabled,
				},
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
				generic.RawLocks = loadDefLocks(t, tc.lockFile)
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

func loadDefLocks(t *testing.T, filepath string) map[string]interface{} {
	raw := loadRawTestdata(t, filepath)

	var locks map[string]interface{}
	if err := yaml.Unmarshal(raw, &locks); err != nil {
		t.Fatal(err)
	}

	return locks
}

type mergeStageTC struct {
	base       func() nodejs.Stage
	overriding nodejs.Stage
	expected   func() nodejs.Stage
}

func emptyStage() nodejs.Stage {
	return nodejs.Stage{
		ExternalFiles:  []llbutils.ExternalFile{},
		SystemPackages: &builddef.VersionMap{},
		GlobalPackages: &builddef.VersionMap{},
		ConfigFiles:    map[string]string{},
		Sources:        []string{},
		StatefulDirs:   []string{},
	}
}

func initMergeExternalFilesWithBaseTC() mergeStageTC {
	return mergeStageTC{
		base: func() nodejs.Stage {
			return nodejs.Stage{
				ExternalFiles: []llbutils.ExternalFile{
					{
						URL:         "https://github.com/some/tool",
						Destination: "/usr/local/bin/some-tool",
						Mode:        0750,
					},
				},
			}
		},
		overriding: nodejs.Stage{
			ExternalFiles: []llbutils.ExternalFile{
				{
					URL:         "https://github.com/some/other/tool",
					Destination: "/usr/local/bin/some-other-tool",
					Mode:        0750,
				},
			},
		},
		expected: func() nodejs.Stage {
			s := emptyStage()
			s.ExternalFiles = []llbutils.ExternalFile{
				{
					URL:         "https://github.com/some/tool",
					Destination: "/usr/local/bin/some-tool",
					Mode:        0750,
				},
				{
					URL:         "https://github.com/some/other/tool",
					Destination: "/usr/local/bin/some-other-tool",
					Mode:        0750,
				},
			}
			return s
		},
	}
}

func initMergeExternalFilesWithoutBaseTC() mergeStageTC {
	return mergeStageTC{
		base: func() nodejs.Stage {
			return nodejs.Stage{}
		},
		overriding: nodejs.Stage{
			ExternalFiles: []llbutils.ExternalFile{
				{
					URL:         "https://github.com/some/other/tool",
					Destination: "/usr/local/bin/some-other-tool",
					Mode:        0750,
				},
			},
		},
		expected: func() nodejs.Stage {
			s := emptyStage()
			s.ExternalFiles = []llbutils.ExternalFile{
				{
					URL:         "https://github.com/some/other/tool",
					Destination: "/usr/local/bin/some-other-tool",
					Mode:        0750,
				},
			}
			return s
		},
	}
}

func initMergeSystemPackagesWithBaseTC() mergeStageTC {
	return mergeStageTC{
		base: func() nodejs.Stage {
			return nodejs.Stage{
				SystemPackages: &builddef.VersionMap{
					"curl": "*",
				},
			}
		},
		overriding: nodejs.Stage{
			SystemPackages: &builddef.VersionMap{
				"chromium": "*",
			},
		},
		expected: func() nodejs.Stage {
			s := emptyStage()
			s.SystemPackages = &builddef.VersionMap{
				"curl":     "*",
				"chromium": "*",
			}
			return s
		},
	}
}

func initMergeSystemPackagesWithoutBaseTC() mergeStageTC {
	return mergeStageTC{
		base: func() nodejs.Stage {
			return nodejs.Stage{}
		},
		overriding: nodejs.Stage{
			SystemPackages: &builddef.VersionMap{
				"chromium": "*",
			},
		},
		expected: func() nodejs.Stage {
			s := emptyStage()
			s.SystemPackages = &builddef.VersionMap{
				"chromium": "*",
			}
			return s
		},
	}
}

func initMergeGlobalPackagesWithBaseTC() mergeStageTC {
	return mergeStageTC{
		base: func() nodejs.Stage {
			return nodejs.Stage{
				GlobalPackages: &builddef.VersionMap{
					"puppeteer": "*",
				},
			}
		},
		overriding: nodejs.Stage{
			GlobalPackages: &builddef.VersionMap{
				"api-platform/client-generator": "*",
			},
		},
		expected: func() nodejs.Stage {
			s := emptyStage()
			s.GlobalPackages = &builddef.VersionMap{
				"puppeteer":                     "*",
				"api-platform/client-generator": "*",
			}
			return s
		},
	}
}

func initMergeGlobalPackagesWithoutBaseTC() mergeStageTC {
	return mergeStageTC{
		base: func() nodejs.Stage {
			return nodejs.Stage{}
		},
		overriding: nodejs.Stage{
			GlobalPackages: &builddef.VersionMap{
				"puppeteer": "*",
			},
		},
		expected: func() nodejs.Stage {
			s := emptyStage()
			s.GlobalPackages = &builddef.VersionMap{
				"puppeteer": "*",
			}
			return s
		},
	}
}

func initMergeBuildCommandWithBaseTC() mergeStageTC {
	baseBuildCmd := "yarn run build"
	overridingCmd := "yarn run build:production"
	expectedCmd := "yarn run build:production"

	return mergeStageTC{
		base: func() nodejs.Stage {
			return nodejs.Stage{
				BuildCommand: &baseBuildCmd,
			}
		},
		overriding: nodejs.Stage{
			BuildCommand: &overridingCmd,
		},
		expected: func() nodejs.Stage {
			s := emptyStage()
			s.BuildCommand = &expectedCmd
			return s
		},
	}
}

func initMergeBuildCommandWithoutBaseTC() mergeStageTC {
	overridingCmd := "yarn run build:production"
	expectedCmd := "yarn run build:production"

	return mergeStageTC{
		base: func() nodejs.Stage {
			return nodejs.Stage{}
		},
		overriding: nodejs.Stage{
			BuildCommand: &overridingCmd,
		},
		expected: func() nodejs.Stage {
			s := emptyStage()
			s.BuildCommand = &expectedCmd
			return s
		},
	}
}

func initMergeCommandWithBaseTC() mergeStageTC {
	baseCmd := []string{"yarn start"}
	overridingCmd := []string{"yarn run start:production"}
	expectedCmd := []string{"yarn run start:production"}

	return mergeStageTC{
		base: func() nodejs.Stage {
			return nodejs.Stage{
				Command: &baseCmd,
			}
		},
		overriding: nodejs.Stage{
			Command: &overridingCmd,
		},
		expected: func() nodejs.Stage {
			s := emptyStage()
			s.Command = &expectedCmd
			return s
		},
	}
}

func initMergeCommandWithoutBaseTC() mergeStageTC {
	overridingCmd := []string{"yarn run start:production"}
	expectedCmd := []string{"yarn run start:production"}

	return mergeStageTC{
		base: func() nodejs.Stage {
			return nodejs.Stage{}
		},
		overriding: nodejs.Stage{
			Command: &overridingCmd,
		},
		expected: func() nodejs.Stage {
			s := emptyStage()
			s.Command = &expectedCmd
			return s
		},
	}
}

func initMergeConfigFilesWithBaseTC() mergeStageTC {
	return mergeStageTC{
		base: func() nodejs.Stage {
			return nodejs.Stage{
				ConfigFiles: map[string]string{
					".env": ".env",
				},
			}
		},
		overriding: nodejs.Stage{
			ConfigFiles: map[string]string{
				".env.production": ".env.production",
			},
		},
		expected: func() nodejs.Stage {
			s := emptyStage()
			s.ConfigFiles = map[string]string{
				".env":            ".env",
				".env.production": ".env.production",
			}
			return s
		},
	}
}

func initMergeConfigFilesWithoutBaseTC() mergeStageTC {
	return mergeStageTC{
		base: func() nodejs.Stage {
			return nodejs.Stage{}
		},
		overriding: nodejs.Stage{
			ConfigFiles: map[string]string{
				".env.production": ".env.production",
			},
		},
		expected: func() nodejs.Stage {
			s := emptyStage()
			s.ConfigFiles = map[string]string{
				".env.production": ".env.production",
			}
			return s
		},
	}
}

func initMergeSourcesWithBaseTC() mergeStageTC {
	return mergeStageTC{
		base: func() nodejs.Stage {
			return nodejs.Stage{
				Sources: []string{"lib/"},
			}
		},
		overriding: nodejs.Stage{
			Sources: []string{"src/"},
		},
		expected: func() nodejs.Stage {
			s := emptyStage()
			s.Sources = []string{"lib/", "src/"}
			return s
		},
	}
}

func initMergeSourcesWithoutBaseTC() mergeStageTC {
	return mergeStageTC{
		base: func() nodejs.Stage {
			return nodejs.Stage{}
		},
		overriding: nodejs.Stage{
			Sources: []string{"src/"},
		},
		expected: func() nodejs.Stage {
			s := emptyStage()
			s.Sources = []string{"src/"}
			return s
		},
	}
}

func initMergeStatefulDirsWithBaseTC() mergeStageTC {
	return mergeStageTC{
		base: func() nodejs.Stage {
			return nodejs.Stage{
				StatefulDirs: []string{"sessions/"},
			}
		},
		overriding: nodejs.Stage{
			StatefulDirs: []string{"uploads/"},
		},
		expected: func() nodejs.Stage {
			s := emptyStage()
			s.StatefulDirs = []string{"sessions/", "uploads/"}
			return s
		},
	}
}

func initMergeStatefulDirsWithoutBaseTC() mergeStageTC {
	return mergeStageTC{
		base: func() nodejs.Stage {
			return nodejs.Stage{}
		},
		overriding: nodejs.Stage{
			StatefulDirs: []string{"uploads/"},
		},
		expected: func() nodejs.Stage {
			s := emptyStage()
			s.StatefulDirs = []string{"uploads/"}
			return s
		},
	}
}

func initMergeHealthcheckWithBaseTC() mergeStageTC {
	return mergeStageTC{
		base: func() nodejs.Stage {
			return nodejs.Stage{
				Healthcheck: &builddef.HealthcheckConfig{
					HealthcheckHTTP: &builddef.HealthcheckHTTP{
						Path:     "/ping",
						Expected: "pong",
					},
					Type:     builddef.HealthcheckTypeHTTP,
					Interval: 10 * time.Second,
					Timeout:  1 * time.Second,
					Retries:  3,
				},
			}
		},
		overriding: nodejs.Stage{
			Healthcheck: &builddef.HealthcheckConfig{
				Type: builddef.HealthcheckTypeDisabled,
			},
		},
		expected: func() nodejs.Stage {
			s := emptyStage()
			s.Healthcheck = &builddef.HealthcheckConfig{
				Type: builddef.HealthcheckTypeDisabled,
			}
			return s
		},
	}
}

func initMergeHealthcheckWithoutBaseTC() mergeStageTC {
	return mergeStageTC{
		base: func() nodejs.Stage {
			return nodejs.Stage{}
		},
		overriding: nodejs.Stage{
			Healthcheck: &builddef.HealthcheckConfig{
				Type: builddef.HealthcheckTypeDisabled,
			},
		},
		expected: func() nodejs.Stage {
			s := emptyStage()
			s.Healthcheck = &builddef.HealthcheckConfig{
				Type: builddef.HealthcheckTypeDisabled,
			}
			return s
		},
	}
}

func initIgnoreNilHealthcheckTC() mergeStageTC {
	return mergeStageTC{
		base: func() nodejs.Stage {
			return nodejs.Stage{
				Healthcheck: &builddef.HealthcheckConfig{
					Type: builddef.HealthcheckTypeDisabled,
				},
			}
		},
		overriding: nodejs.Stage{},
		expected: func() nodejs.Stage {
			s := emptyStage()
			s.Healthcheck = &builddef.HealthcheckConfig{
				Type: builddef.HealthcheckTypeDisabled,
			}
			return s
		},
	}
}

func TestStageMerge(t *testing.T) {
	testcases := map[string]func() mergeStageTC{
		"merge external files with base":     initMergeExternalFilesWithBaseTC,
		"merge external files without base":  initMergeExternalFilesWithoutBaseTC,
		"merge system packages with base":    initMergeSystemPackagesWithBaseTC,
		"merge system packages without base": initMergeSystemPackagesWithoutBaseTC,
		"merge global packages with base":    initMergeGlobalPackagesWithBaseTC,
		"merge global packages without base": initMergeGlobalPackagesWithoutBaseTC,
		"merge build command with base":      initMergeBuildCommandWithBaseTC,
		"merge build command without base":   initMergeBuildCommandWithoutBaseTC,
		"merge command with base":            initMergeCommandWithBaseTC,
		"merge command without base":         initMergeCommandWithoutBaseTC,
		"merge config files with base":       initMergeConfigFilesWithBaseTC,
		"merge config files without base":    initMergeConfigFilesWithoutBaseTC,
		"merge sources with base":            initMergeSourcesWithBaseTC,
		"merge sources without base":         initMergeSourcesWithoutBaseTC,
		"merge stateful dirs with base":      initMergeStatefulDirsWithBaseTC,
		"merge stateful dirs without base":   initMergeStatefulDirsWithoutBaseTC,
		"merge healthcheck with base":        initMergeHealthcheckWithBaseTC,
		"merge healthcheck without base":     initMergeHealthcheckWithoutBaseTC,
		"ignore nil healthcheck":             initIgnoreNilHealthcheckTC,
	}

	for tcname := range testcases {
		tcinit := testcases[tcname]

		t.Run(tcname, func(t *testing.T) {
			tc := tcinit()
			base := tc.base()
			new := base.Merge(tc.overriding)

			if diff := deep.Equal(new, tc.expected()); diff != nil {
				t.Fatal(diff)
			}

			if diff := deep.Equal(base, tc.base()); diff != nil {
				t.Fatalf("Base stages don't match: %v", diff)
			}
		})
	}
}

type mergeDefinitionTC struct {
	base       func() nodejs.Definition
	overriding nodejs.Definition
	expected   func() nodejs.Definition
}

func initMergeBaseStageWithBaseTC() mergeDefinitionTC {
	return mergeDefinitionTC{
		base: func() nodejs.Definition {
			return nodejs.Definition{
				BaseStage: nodejs.Stage{
					Sources: []string{"src/"},
				},
			}
		},
		overriding: nodejs.Definition{
			BaseStage: nodejs.Stage{
				Sources: []string{"bin/"},
			},
		},
		expected: func() nodejs.Definition {
			baseStage := emptyStage()
			baseStage.Sources = []string{"src/", "bin/"}

			return nodejs.Definition{
				BaseStage: baseStage,
				Stages:    nodejs.DerivedStageSet{},
			}
		},
	}
}

func initMergeBaseStageWithoutBaseTC() mergeDefinitionTC {
	return mergeDefinitionTC{
		base: func() nodejs.Definition {
			return nodejs.Definition{}
		},
		overriding: nodejs.Definition{
			BaseStage: nodejs.Stage{
				Sources: []string{"bin/"},
			},
		},
		expected: func() nodejs.Definition {
			baseStage := emptyStage()
			baseStage.Sources = []string{"bin/"}

			return nodejs.Definition{
				BaseStage: baseStage,
				Stages:    nodejs.DerivedStageSet{},
			}
		},
	}
}

func initMergeBaseImageWithBaseTC() mergeDefinitionTC {
	return mergeDefinitionTC{
		base: func() nodejs.Definition {
			return nodejs.Definition{
				BaseImage: "docker.io/library/nodejs:latest",
			}
		},
		overriding: nodejs.Definition{
			BaseImage: "docker.io/library/nodejs:v11.5-alpine",
		},
		expected: func() nodejs.Definition {
			return nodejs.Definition{
				BaseImage: "docker.io/library/nodejs:v11.5-alpine",
				BaseStage: emptyStage(),
				Stages:    nodejs.DerivedStageSet{},
			}
		},
	}
}

func initMergeBaseImageWithoutBaseTC() mergeDefinitionTC {
	return mergeDefinitionTC{
		base: func() nodejs.Definition {
			return nodejs.Definition{}
		},
		overriding: nodejs.Definition{
			BaseImage: "docker.io/library/nodejs:v11.5-alpine",
		},
		expected: func() nodejs.Definition {
			return nodejs.Definition{
				BaseImage: "docker.io/library/nodejs:v11.5-alpine",
				BaseStage: emptyStage(),
				Stages:    nodejs.DerivedStageSet{},
			}
		},
	}
}

func initMergeVersionWithBaseTC() mergeDefinitionTC {
	return mergeDefinitionTC{
		base: func() nodejs.Definition {
			return nodejs.Definition{
				Version: "11.5",
			}
		},
		overriding: nodejs.Definition{
			Version: "12",
		},
		expected: func() nodejs.Definition {
			return nodejs.Definition{
				Version:   "12",
				BaseStage: emptyStage(),
				Stages:    nodejs.DerivedStageSet{},
			}
		},
	}
}

func initMergeVersionWithoutBaseTC() mergeDefinitionTC {
	return mergeDefinitionTC{
		base: func() nodejs.Definition {
			return nodejs.Definition{}
		},
		overriding: nodejs.Definition{
			Version: "12",
		},
		expected: func() nodejs.Definition {
			return nodejs.Definition{
				Version:   "12",
				BaseStage: emptyStage(),
				Stages:    nodejs.DerivedStageSet{},
			}
		},
	}
}

func initMergeStagesWithBaseTC() mergeDefinitionTC {
	return mergeDefinitionTC{
		base: func() nodejs.Definition {
			return nodejs.Definition{
				Stages: nodejs.DerivedStageSet{
					"dev": nodejs.DerivedStage{
						DeriveFrom: "base",
					},
				},
			}
		},
		overriding: nodejs.Definition{
			Stages: nodejs.DerivedStageSet{
				"dev": nodejs.DerivedStage{
					DeriveFrom: "prod",
				},
			},
		},
		expected: func() nodejs.Definition {
			return nodejs.Definition{
				Stages: nodejs.DerivedStageSet{
					"dev": nodejs.DerivedStage{
						DeriveFrom: "prod",
						Stage:      emptyStage(),
					},
				},
				BaseStage: emptyStage(),
			}
		},
	}
}

func initMergeStagesWithoutBaseTC() mergeDefinitionTC {
	return mergeDefinitionTC{
		base: func() nodejs.Definition {
			return nodejs.Definition{}
		},
		overriding: nodejs.Definition{
			Stages: nodejs.DerivedStageSet{
				"dev": nodejs.DerivedStage{
					DeriveFrom: "prod",
				},
			},
		},
		expected: func() nodejs.Definition {
			return nodejs.Definition{
				Stages: nodejs.DerivedStageSet{
					"dev": nodejs.DerivedStage{
						DeriveFrom: "prod",
					},
				},
				BaseStage: emptyStage(),
			}
		},
	}
}

func initMergeIsFrontendWithBaseTC() mergeDefinitionTC {
	return mergeDefinitionTC{
		base: func() nodejs.Definition {
			return nodejs.Definition{
				IsFrontend: true,
			}
		},
		overriding: nodejs.Definition{
			IsFrontend: false,
		},
		expected: func() nodejs.Definition {
			return nodejs.Definition{
				IsFrontend: false,
				BaseStage:  emptyStage(),
				Stages:     nodejs.DerivedStageSet{},
			}
		},
	}
}

func initMergeIsFrontendWithoutBaseTC() mergeDefinitionTC {
	return mergeDefinitionTC{
		base: func() nodejs.Definition {
			return nodejs.Definition{}
		},
		overriding: nodejs.Definition{
			IsFrontend: false,
		},
		expected: func() nodejs.Definition {
			return nodejs.Definition{
				IsFrontend: false,
				BaseStage:  emptyStage(),
				Stages:     nodejs.DerivedStageSet{},
			}
		},
	}
}

func TestDefinitionMerge(t *testing.T) {
	testcases := map[string]func() mergeDefinitionTC{
		"merge base stage with base":     initMergeBaseStageWithBaseTC,
		"merge base stage without base":  initMergeBaseStageWithoutBaseTC,
		"merge base image with base":     initMergeBaseImageWithBaseTC,
		"merge base image without base":  initMergeBaseImageWithoutBaseTC,
		"merge version with base":        initMergeVersionWithBaseTC,
		"merge version without base":     initMergeVersionWithoutBaseTC,
		"merge stages with base":         initMergeStagesWithBaseTC,
		"merge stages without base":      initMergeStagesWithoutBaseTC,
		"merge is frontend with base":    initMergeIsFrontendWithBaseTC,
		"merge is frontend without base": initMergeIsFrontendWithoutBaseTC,
	}

	for tcname := range testcases {
		tcinit := testcases[tcname]

		t.Run(tcname, func(t *testing.T) {
			t.Parallel()

			tc := tcinit()
			base := tc.base()
			new := base.Merge(tc.overriding)

			if diff := deep.Equal(new, tc.expected()); diff != nil {
				t.Fatal(diff)
			}

			if diff := deep.Equal(base, tc.base()); diff != nil {
				t.Fatalf("Base definition has been modified: %v", diff)
			}
		})
	}
}
