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
				SourceDirs:     []string{},
				StatefulDirs:   []string{},
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

type mergeStageTC struct {
	base       func() nodejs.Stage
	overriding nodejs.Stage
	expected   func() nodejs.Stage
}

func emptyStage() nodejs.Stage {
	return nodejs.Stage{
		ExternalFiles:  []llbutils.ExternalFile{},
		SystemPackages: map[string]string{},
		GlobalPackages: map[string]string{},
		ConfigFiles:    map[string]string{},
		SourceDirs:     []string{},
		StatefulDirs:   []string{},
		PostInstall:    []string{},
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
				SystemPackages: map[string]string{
					"curl": "*",
				},
			}
		},
		overriding: nodejs.Stage{
			SystemPackages: map[string]string{
				"chromium": "*",
			},
		},
		expected: func() nodejs.Stage {
			s := emptyStage()
			s.SystemPackages = map[string]string{
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
			SystemPackages: map[string]string{
				"chromium": "*",
			},
		},
		expected: func() nodejs.Stage {
			s := emptyStage()
			s.SystemPackages = map[string]string{
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
				GlobalPackages: map[string]string{
					"puppeteer": "*",
				},
			}
		},
		overriding: nodejs.Stage{
			GlobalPackages: map[string]string{
				"api-platform/client-generator": "*",
			},
		},
		expected: func() nodejs.Stage {
			s := emptyStage()
			s.GlobalPackages = map[string]string{
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
			GlobalPackages: map[string]string{
				"puppeteer": "*",
			},
		},
		expected: func() nodejs.Stage {
			s := emptyStage()
			s.GlobalPackages = map[string]string{
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

func initMergeSourceDirsWithBaseTC() mergeStageTC {
	return mergeStageTC{
		base: func() nodejs.Stage {
			return nodejs.Stage{
				SourceDirs: []string{"lib/"},
			}
		},
		overriding: nodejs.Stage{
			SourceDirs: []string{"src/"},
		},
		expected: func() nodejs.Stage {
			s := emptyStage()
			s.SourceDirs = []string{"lib/", "src/"}
			return s
		},
	}
}

func initMergeSourceDirsWithoutBaseTC() mergeStageTC {
	return mergeStageTC{
		base: func() nodejs.Stage {
			return nodejs.Stage{}
		},
		overriding: nodejs.Stage{
			SourceDirs: []string{"src/"},
		},
		expected: func() nodejs.Stage {
			s := emptyStage()
			s.SourceDirs = []string{"src/"}
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
	baseHealthcheck := true
	overridingHealthcheck := false
	expectedHealthcheck := false

	return mergeStageTC{
		base: func() nodejs.Stage {
			return nodejs.Stage{
				Healthcheck: &baseHealthcheck,
			}
		},
		overriding: nodejs.Stage{
			Healthcheck: &overridingHealthcheck,
		},
		expected: func() nodejs.Stage {
			s := emptyStage()
			s.Healthcheck = &expectedHealthcheck
			return s
		},
	}
}

func initMergeHealthcheckWithoutBaseTC() mergeStageTC {
	overridingHealthcheck := true
	expectedHealthcheck := true

	return mergeStageTC{
		base: func() nodejs.Stage {
			return nodejs.Stage{}
		},
		overriding: nodejs.Stage{
			Healthcheck: &overridingHealthcheck,
		},
		expected: func() nodejs.Stage {
			s := emptyStage()
			s.Healthcheck = &expectedHealthcheck
			return s
		},
	}
}

func initMergePostInstallWithBaseTC() mergeStageTC {
	return mergeStageTC{
		base: func() nodejs.Stage {
			return nodejs.Stage{
				PostInstall: []string{"yarn run build"},
			}
		},
		overriding: nodejs.Stage{
			PostInstall: []string{"yarn run warmup"},
		},
		expected: func() nodejs.Stage {
			s := emptyStage()
			s.PostInstall = []string{"yarn run build", "yarn run warmup"}
			return s
		},
	}
}

func initMergePostInstallWithoutBaseTC() mergeStageTC {
	return mergeStageTC{
		base: func() nodejs.Stage {
			return nodejs.Stage{}
		},
		overriding: nodejs.Stage{
			PostInstall: []string{"yarn run warmup"},
		},
		expected: func() nodejs.Stage {
			s := emptyStage()
			s.PostInstall = []string{"yarn run warmup"}
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
		"merge source dirs with base":        initMergeSourceDirsWithBaseTC,
		"merge source dirs without base":     initMergeSourceDirsWithoutBaseTC,
		"merge stateful dirs with base":      initMergeStatefulDirsWithBaseTC,
		"merge stateful dirs without base":   initMergeStatefulDirsWithoutBaseTC,
		"merge healthcheck with base":        initMergeHealthcheckWithBaseTC,
		"merge healthcheck without base":     initMergeHealthcheckWithoutBaseTC,
		"merge post install with base":       initMergePostInstallWithBaseTC,
		"merge post install without base":    initMergePostInstallWithoutBaseTC,
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
