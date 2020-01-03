package php_test

import (
	"errors"
	"testing"

	"github.com/NiR-/zbuild/pkg/builddef"
	"github.com/NiR-/zbuild/pkg/defkinds/php"
	"github.com/NiR-/zbuild/pkg/defkinds/webserver"
	"github.com/NiR-/zbuild/pkg/llbutils"
	"github.com/go-test/deep"
	"github.com/golang/mock/gomock"
	"golang.org/x/xerrors"
	"gopkg.in/yaml.v2"
)

type newDefinitionTC struct {
	file        string
	lockFile    string
	expected    php.Definition
	expectedErr error
}

func initSuccessfullyParseRawDefinitionWithoutStagesTC() newDefinitionTC {
	file := "testdata/def/without-stages.yml"
	lockFile := "testdata/def/without-stages.lock"

	isFPM := true
	iniFile := "docker/app/php.ini"
	fpmConfigFile := "docker/app/fpm.conf"
	healthcheck := true
	isDev := true
	isNotDev := false
	inferMode := false

	devStage := emptyStage()
	prodStage := emptyStage()

	return newDefinitionTC{
		file:     file,
		lockFile: lockFile,
		expected: php.Definition{
			BaseStage: php.Stage{
				ExternalFiles:  []llbutils.ExternalFile{},
				SystemPackages: map[string]string{},
				FPM:            &isFPM,
				Extensions: map[string]string{
					"intl":      "*",
					"pdo_mysql": "*",
					"soap":      "*",
				},
				GlobalDeps: map[string]string{},
				ConfigFiles: php.PHPConfigFiles{
					IniFile:       &iniFile,
					FPMConfigFile: &fpmConfigFile,
				},
				ComposerDumpFlags: &php.ComposerDumpFlags{
					APCU:                  false,
					ClassmapAuthoritative: true,
				},
				Sources:      []string{"./src"},
				Integrations: []string{"blackfire"},
				StatefulDirs: []string{"./public/uploads"},
				Healthcheck:  &healthcheck,
				PostInstall: []string{
					"some more commands",
					"another one",
				},
			},
			Version:       "7.4.0",
			MajMinVersion: "7.4",
			BaseImage:     "docker.io/library/php:7.4-fpm-buster",
			Infer:         &inferMode,
			Stages: map[string]php.DerivedStage{
				"dev": {
					DeriveFrom: "base",
					Dev:        &isDev,
					Stage:      devStage,
				},
				"prod": {
					DeriveFrom: "base",
					Dev:        &isNotDev,
					Stage:      prodStage,
				},
			},
			Locks: php.DefinitionLocks{
				Stages: map[string]php.StageLocks{
					"dev": {
						SystemPackages: map[string]string{
							"git":        "1:2.1.4-2.1+deb8u7",
							"libicu-dev": "52.1-8+deb8u7",
						},
						Extensions: map[string]string{
							"intl":      "*",
							"pdo_mysql": "*",
							"soap":      "*",
						},
					},
				},
			},
		},
	}
}

func initSuccessfullyParseRawDefinitionWithStagesTC() newDefinitionTC {
	iniDevFile := "docker/app/php.dev.ini"
	iniProdFile := "docker/app/php.prod.ini"
	fpmConfigFile := "docker/app/fpm.conf"
	healthcheckEnabled := true
	healthcheckDisabled := false
	devStageDevMode := true
	prodStageDevMode := false
	isFPM := true
	isNotFPM := false
	workerCmd := []string{"bin/worker"}
	inferMode := true

	devStage := emptyStage()
	devStage.ConfigFiles = php.PHPConfigFiles{
		IniFile: &iniDevFile,
	}

	prodStage := emptyStage()
	prodStage.ConfigFiles = php.PHPConfigFiles{
		IniFile: &iniProdFile,
	}
	prodStage.Healthcheck = &healthcheckEnabled
	prodStage.Integrations = []string{"blackfire"}

	return newDefinitionTC{
		file:     "testdata/def/merge-all.yml",
		lockFile: "",
		expected: php.Definition{
			BaseStage: php.Stage{
				ExternalFiles:  []llbutils.ExternalFile{},
				SystemPackages: map[string]string{},
				FPM:            &isFPM,
				Extensions: map[string]string{
					"intl":      "*",
					"pdo_mysql": "*",
					"soap":      "*",
				},
				GlobalDeps: map[string]string{},
				ConfigFiles: php.PHPConfigFiles{
					FPMConfigFile: &fpmConfigFile,
				},
				ComposerDumpFlags: &php.ComposerDumpFlags{
					APCU:                  false,
					ClassmapAuthoritative: true,
				},
				Sources:      []string{"generated/"},
				Integrations: []string{},
				StatefulDirs: []string{"public/uploads"},
				Healthcheck:  &healthcheckDisabled,
				PostInstall:  []string{"echo some command"},
			},
			Version:       "7.4.0",
			MajMinVersion: "7.4",
			BaseImage:     "docker.io/library/php:7.4-fpm-buster",
			Infer:         &inferMode,
			Stages: map[string]php.DerivedStage{
				"dev": {
					DeriveFrom: "",
					Dev:        &devStageDevMode,
					Stage:      devStage,
				},
				"prod": {
					DeriveFrom: "",
					Dev:        &prodStageDevMode,
					Stage:      prodStage,
				},
				"worker": {
					DeriveFrom: "prod",
					Stage: php.Stage{
						ConfigFiles: php.PHPConfigFiles{},
						ComposerDumpFlags: &php.ComposerDumpFlags{
							APCU:                  true,
							ClassmapAuthoritative: false,
						},
						Sources:      []string{"worker/"},
						StatefulDirs: []string{"data/imports"},
						PostInstall:  []string{"echo some other command"},
						FPM:          &isNotFPM,
						Command:      &workerCmd,
					},
				},
			},
			Locks: php.DefinitionLocks{},
		},
	}
}

func initFailToParseUnknownPropertiesTC() newDefinitionTC {
	return newDefinitionTC{
		file:        "testdata/def/with-invalid-properties.yml",
		lockFile:    "",
		expectedErr: errors.New("could not decode build manifest: 1 error(s) decoding:\n\n* '' has invalid keys: foo"),
	}
}

func TestNewKind(t *testing.T) {
	if *flagTestdata {
		return
	}

	testcases := map[string]func() newDefinitionTC{
		"successfully parse raw definition without stages": initSuccessfullyParseRawDefinitionWithoutStagesTC,
		"successfully parse raw definition with stages":    initSuccessfullyParseRawDefinitionWithStagesTC,
		"fail to parse unknown properties":                 initFailToParseUnknownPropertiesTC,
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

			def, err := php.NewKind(generic)
			if tc.expectedErr != nil {
				if err == nil || tc.expectedErr.Error() != err.Error() {
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
	file               string
	lockFile           string
	stage              string
	composerLockLoader func(*php.StageDefinition) error
	expected           php.StageDefinition
	expectedErr        error
}

func initSuccessfullyResolveDefaultDevStageTC(t *testing.T, mockCtrl *gomock.Controller) resolveStageTC {
	file := "testdata/def/without-stages.yml"
	lockFile := "testdata/def/without-stages.lock"

	isFPM := true
	healthckeck := false
	phpIni := "docker/app/php.ini"
	fpmConfigFile := "docker/app/fpm.conf"

	return resolveStageTC{
		file:     file,
		lockFile: lockFile,
		stage:    "dev",
		composerLockLoader: func(stageDef *php.StageDefinition) error {
			return nil
		},
		expected: php.StageDefinition{
			Name:           "dev",
			BaseImage:      "docker.io/library/php:7.4-fpm-buster",
			Version:        "7.4.0",
			MajMinVersion:  "7.4",
			Infer:          false,
			Dev:            true,
			LockedPackages: map[string]string{},
			PlatformReqs:   map[string]string{},
			Stage: php.Stage{
				ExternalFiles: []llbutils.ExternalFile{
					{
						URL:         "https://blackfire.io/api/v1/releases/probe/php/linux/amd64/72",
						Compressed:  true,
						Pattern:     "blackfire-*.so",
						Destination: "/usr/local/lib/php/extensions/no-debug-non-zts-20190902/blackfire.so",
						Mode:        0644,
					},
				},
				SystemPackages: map[string]string{},
				FPM:            &isFPM,
				Extensions: map[string]string{
					"intl":      "*",
					"pdo_mysql": "*",
					"soap":      "*",
				},
				GlobalDeps:   map[string]string{},
				Sources:      []string{"./src"},
				StatefulDirs: []string{"./public/uploads"},
				ConfigFiles: php.PHPConfigFiles{
					IniFile:       &phpIni,
					FPMConfigFile: &fpmConfigFile,
				},
				ComposerDumpFlags: &php.ComposerDumpFlags{
					APCU:                  false,
					ClassmapAuthoritative: true,
				},
				Integrations: []string{"blackfire"},
				Healthcheck:  &healthckeck,
				PostInstall:  []string{"some more commands", "another one"},
			},
		},
	}
}

func initSuccessfullyResolveWorkerStageTC(t *testing.T, mockCtrl *gomock.Controller) resolveStageTC {
	isNotFPM := false
	healthcheckDisabled := false
	workerCmd := []string{"bin/worker"}

	return resolveStageTC{
		file:     "testdata/def/worker.yml",
		lockFile: "",
		stage:    "prod",
		composerLockLoader: mockComposerLockLoader(
			map[string]string{
				"clue/stream-filter": "v1.4.0",
			},
			map[string]string{
				"mbstring": "*",
			},
		),
		expected: php.StageDefinition{
			Name:          "prod",
			BaseImage:     "docker.io/library/php:7.4-cli-buster",
			Version:       "7.4.0",
			MajMinVersion: "7.4",
			Infer:         true,
			Dev:           false,
			LockedPackages: map[string]string{
				"clue/stream-filter": "v1.4.0",
			},
			PlatformReqs: map[string]string{
				"mbstring": "*",
			},
			Stage: php.Stage{
				ExternalFiles: []llbutils.ExternalFile{},
				SystemPackages: map[string]string{
					"zlib1g-dev":   "*",
					"unzip":        "*",
					"git":          "*",
					"libpcre3-dev": "*",
					"libzip-dev":   "*",
				},
				FPM:     &isNotFPM,
				Command: &workerCmd,
				Extensions: map[string]string{
					"mbstring": "*",
					"zip":      "*",
				},
				GlobalDeps: map[string]string{},
				ComposerDumpFlags: &php.ComposerDumpFlags{
					APCU:                  false,
					ClassmapAuthoritative: true,
				},
				Sources:      []string{"bin/", "src/"},
				Integrations: []string{},
				StatefulDirs: []string{},
				Healthcheck:  &healthcheckDisabled,
				PostInstall:  []string{},
			},
		},
	}
}

func initFailToResolveUnknownStageTC(t *testing.T, mockCtrl *gomock.Controller) resolveStageTC {
	file := "testdata/def/without-stages.yml"
	lockFile := "testdata/def/without-stages.lock"

	composerLockLoader := mockComposerLockLoader(map[string]string{}, map[string]string{})

	return resolveStageTC{
		file:               file,
		lockFile:           lockFile,
		stage:              "foo",
		composerLockLoader: composerLockLoader,
		expectedErr:        errors.New(`stage "foo" not found`),
	}
}

func initFailToResolveStageWithCyclicDepsTC(t *testing.T, mockCtrl *gomock.Controller) resolveStageTC {
	composerLockLoader := mockComposerLockLoader(map[string]string{}, map[string]string{})

	return resolveStageTC{
		file:               "testdata/def/cyclic-stage-deps.yml",
		lockFile:           "",
		stage:              "dev",
		composerLockLoader: composerLockLoader,
		expectedErr:        errors.New(`there's a cyclic dependency between "dev" and itself`),
	}
}

func initRemoveDefaultExtensionsTC(t *testing.T, mockCtrl *gomock.Controller) resolveStageTC {
	fpm := true
	healthcheck := false

	composerLockLoader := mockComposerLockLoader(map[string]string{}, map[string]string{})

	return resolveStageTC{
		file:               "testdata/def/remove-default-exts.yml",
		lockFile:           "",
		stage:              "dev",
		composerLockLoader: composerLockLoader,
		expected: php.StageDefinition{
			Name:           "dev",
			BaseImage:      "docker.io/library/php:7.4-fpm-buster",
			Version:        "7.4",
			MajMinVersion:  "7.4",
			Infer:          true,
			Dev:            true,
			LockedPackages: map[string]string{},
			PlatformReqs:   map[string]string{},
			Stage: php.Stage{
				ExternalFiles: []llbutils.ExternalFile{},
				SystemPackages: map[string]string{
					"zlib1g-dev":    "*",
					"unzip":         "*",
					"git":           "*",
					"libpcre3-dev":  "*",
					"libsodium-dev": "*",
					"libzip-dev":    "*",
				},
				FPM: &fpm,
				Extensions: map[string]string{
					"zip":        "*",
					"mbstring":   "*",
					"reflection": "*",
					"sodium":     "*",
					"spl":        "*",
					"standard":   "*",
					"filter":     "*",
					"json":       "*",
					"session":    "*",
				},
				GlobalDeps:  map[string]string{},
				ConfigFiles: php.PHPConfigFiles{},
				ComposerDumpFlags: &php.ComposerDumpFlags{
					ClassmapAuthoritative: true,
				},
				Sources:      []string{},
				Integrations: []string{},
				StatefulDirs: []string{},
				Healthcheck:  &healthcheck,
				PostInstall:  []string{},
			},
		},
	}
}

// This TC ensures that the extensions infered from composer.lock aren't
// erasing version constraints defined in the zbuildfile.
func initPreservePredefinedExtensionConstraintsTC(t *testing.T, mockCtrl *gomock.Controller) resolveStageTC {
	fpm := true
	healthcheck := false

	return resolveStageTC{
		file:     "testdata/def/with-predefined-extension.yml",
		lockFile: "",
		stage:    "dev",
		composerLockLoader: mockComposerLockLoader(
			map[string]string{},
			map[string]string{
				"redis": "*",
			},
		),
		expected: php.StageDefinition{
			Name:           "dev",
			BaseImage:      "docker.io/library/php:7.4-fpm-buster",
			Version:        "7.4",
			MajMinVersion:  "7.4",
			Infer:          true,
			Dev:            true,
			LockedPackages: map[string]string{},
			PlatformReqs: map[string]string{
				"redis": "*",
			},
			Stage: php.Stage{
				ExternalFiles: []llbutils.ExternalFile{},
				SystemPackages: map[string]string{
					"zlib1g-dev":   "*",
					"unzip":        "*",
					"git":          "*",
					"libpcre3-dev": "*",
					"libzip-dev":   "*",
				},
				FPM: &fpm,
				Extensions: map[string]string{
					"zip":   "*",
					"redis": "^5.1",
				},
				GlobalDeps:  map[string]string{},
				ConfigFiles: php.PHPConfigFiles{},
				ComposerDumpFlags: &php.ComposerDumpFlags{
					ClassmapAuthoritative: true,
				},
				Sources:      []string{},
				Integrations: []string{},
				StatefulDirs: []string{},
				Healthcheck:  &healthcheck,
				PostInstall:  []string{},
			},
		},
	}
}

func TestResolveStageDefinition(t *testing.T) {
	if *flagTestdata {
		return
	}

	testcases := map[string]func(*testing.T, *gomock.Controller) resolveStageTC{
		"successfully resolve default dev stage":    initSuccessfullyResolveDefaultDevStageTC,
		"successfully resolve worker stage":         initSuccessfullyResolveWorkerStageTC,
		"fail to resolve unknown stage":             initFailToResolveUnknownStageTC,
		"fail to resolve stage with cyclic deps":    initFailToResolveStageWithCyclicDepsTC,
		"remove default extensions":                 initRemoveDefaultExtensionsTC,
		"preserve predefined extension constraints": initPreservePredefinedExtensionConstraintsTC,
	}

	for tcname := range testcases {
		tcinit := testcases[tcname]

		t.Run(tcname, func(t *testing.T) {
			t.Parallel()

			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			tc := tcinit(t, mockCtrl)

			generic := loadBuildDef(t, tc.file)
			if tc.lockFile != "" {
				generic.RawLocks = loadRawTestdata(t, tc.lockFile)
			}

			def, err := php.NewKind(generic)
			if err != nil {
				t.Fatal(err)
			}

			stageDef, err := def.ResolveStageDefinition(tc.stage, tc.composerLockLoader, false)
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

func TestComposerDumpFlags(t *testing.T) {
	testcases := map[string]struct {
		obj         php.ComposerDumpFlags
		expected    string
		expectedErr error
	}{
		"with apcu optimization": {
			obj:      php.ComposerDumpFlags{APCU: true},
			expected: "--no-dev --optimize --apcu",
		},
		"with authoritative classmap": {
			obj:      php.ComposerDumpFlags{ClassmapAuthoritative: true},
			expected: "--no-dev --optimize --classmap-authoritative",
		},
		"with no particular optimization": {
			obj:      php.ComposerDumpFlags{},
			expected: "--no-dev --optimize",
		},
		"fail when both optimizations are enabled": {
			obj:         php.ComposerDumpFlags{APCU: true, ClassmapAuthoritative: true},
			expectedErr: xerrors.New("you can't use both --apcu and --classmap-authoritative flags. See https://getcomposer.org/doc/articles/autoloader-optimization.md"),
		},
	}

	for tcname := range testcases {
		tc := testcases[tcname]

		t.Run(tcname, func(t *testing.T) {
			t.Parallel()

			out, err := tc.obj.Flags()
			if tc.expectedErr != nil {
				if err == nil || err.Error() != tc.expectedErr.Error() {
					t.Fatalf("Expected error: %v\nGot: %v", tc.expectedErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if out != tc.expected {
				t.Fatalf("Expected: %s\nGot: %s", tc.expected, out)
			}
		})
	}
}

func loadBuildDef(t *testing.T, filepath string) *builddef.BuildDef {
	raw := loadRawTestdata(t, filepath)

	var def builddef.BuildDef
	if err := yaml.Unmarshal(raw, &def); err != nil {
		t.Fatal(err)
	}

	return &def
}

// @TODO: use a proper ComposerLock struct
func mockComposerLockLoader(
	lockedPackages map[string]string,
	platformReqs map[string]string,
) func(*php.StageDefinition) error {
	return func(stageDef *php.StageDefinition) error {
		stageDef.LockedPackages = lockedPackages
		stageDef.PlatformReqs = platformReqs
		return nil
	}
}

type mergeDefinitionTC struct {
	base       func() php.Definition
	overriding func() php.Definition
	expected   func() php.Definition
}

func TestMergeDefinition(t *testing.T) {
	testcases := map[string]mergeDefinitionTC{
		"merge base stage with base": {
			base: func() php.Definition {
				return php.Definition{
					BaseStage: php.Stage{
						Sources: []string{"src/"},
					},
				}
			},
			overriding: func() php.Definition {
				return php.Definition{
					BaseStage: php.Stage{
						Sources: []string{"bin/"},
					},
				}
			},
			expected: func() php.Definition {
				baseStage := emptyStage()
				baseStage.Sources = []string{"src/", "bin/"}

				return php.Definition{
					BaseStage: baseStage,
					Stages:    php.DerivedStageSet{},
				}
			},
		},
		"merge base stage without base": {
			base: func() php.Definition {
				return php.Definition{}
			},
			overriding: func() php.Definition {
				return php.Definition{
					BaseStage: php.Stage{
						Sources: []string{"bin/"},
					},
				}
			},
			expected: func() php.Definition {
				baseStage := emptyStage()
				baseStage.Sources = []string{"bin/"}

				return php.Definition{
					BaseStage: baseStage,
					Stages:    php.DerivedStageSet{},
				}
			},
		},
		"merge base image with base": {
			base: func() php.Definition {
				return php.Definition{
					BaseImage: "docker.io/library/php:7.3-fpm-buster",
				}
			},
			overriding: func() php.Definition {
				return php.Definition{
					BaseImage: "docker.io/library/php:7.4-fpm-buster",
				}
			},
			expected: func() php.Definition {
				return php.Definition{
					BaseImage: "docker.io/library/php:7.4-fpm-buster",
					BaseStage: emptyStage(),
					Stages:    php.DerivedStageSet{},
				}
			},
		},
		"merge base image without base": {
			base: func() php.Definition {
				return php.Definition{}
			},
			overriding: func() php.Definition {
				return php.Definition{
					BaseImage: "docker.io/library/php:7.4-fpm-buster",
				}
			},
			expected: func() php.Definition {
				return php.Definition{
					BaseImage: "docker.io/library/php:7.4-fpm-buster",
					BaseStage: emptyStage(),
					Stages:    php.DerivedStageSet{},
				}
			},
		},
		"merge version with base": {
			base: func() php.Definition {
				return php.Definition{
					Version: "7.3",
				}
			},
			overriding: func() php.Definition {
				return php.Definition{
					Version: "7.4",
				}
			},
			expected: func() php.Definition {
				return php.Definition{
					Version:   "7.4",
					BaseStage: emptyStage(),
					Stages:    php.DerivedStageSet{},
				}
			},
		},
		"merge version without base": {
			base: func() php.Definition {
				return php.Definition{}
			},
			overriding: func() php.Definition {
				return php.Definition{
					Version: "7.4",
				}
			},
			expected: func() php.Definition {
				return php.Definition{
					Version:   "7.4",
					BaseStage: emptyStage(),
					Stages:    php.DerivedStageSet{},
				}
			},
		},
		"merge infer with base": {
			base: func() php.Definition {
				infer := true
				return php.Definition{
					Infer: &infer,
				}
			},
			overriding: func() php.Definition {
				infer := false
				return php.Definition{
					Infer: &infer,
				}
			},
			expected: func() php.Definition {
				infer := false
				return php.Definition{
					Infer:     &infer,
					BaseStage: emptyStage(),
					Stages:    php.DerivedStageSet{},
				}
			},
		},
		"merge infer without base": {
			base: func() php.Definition {
				return php.Definition{}
			},
			overriding: func() php.Definition {
				infer := true
				return php.Definition{
					Infer: &infer,
				}
			},
			expected: func() php.Definition {
				infer := true
				return php.Definition{
					Infer:     &infer,
					BaseStage: emptyStage(),
					Stages:    php.DerivedStageSet{},
				}
			},
		},
		"ignore nil infer": {
			base: func() php.Definition {
				infer := true
				return php.Definition{
					Infer: &infer,
				}
			},
			overriding: func() php.Definition {
				return php.Definition{}
			},
			expected: func() php.Definition {
				infer := true
				return php.Definition{
					Infer:     &infer,
					BaseStage: emptyStage(),
					Stages:    php.DerivedStageSet{},
				}
			},
		},
		"merge stages with base": {
			base: func() php.Definition {
				return php.Definition{
					Stages: php.DerivedStageSet{
						"staging": php.DerivedStage{
							DeriveFrom: "dev",
						},
					},
				}
			},
			overriding: func() php.Definition {
				return php.Definition{
					Stages: php.DerivedStageSet{
						"staging": php.DerivedStage{
							DeriveFrom: "prod",
						},
					},
				}
			},
			expected: func() php.Definition {
				return php.Definition{
					BaseStage: emptyStage(),
					Stages: php.DerivedStageSet{
						"staging": php.DerivedStage{
							DeriveFrom: "prod",
							Stage:      emptyStage(),
						},
					},
				}
			},
		},
		"merge stages without base": {
			base: func() php.Definition {
				return php.Definition{
					Stages: php.DerivedStageSet{},
				}
			},
			overriding: func() php.Definition {
				return php.Definition{
					Stages: php.DerivedStageSet{
						"staging": php.DerivedStage{
							DeriveFrom: "prod",
						},
					},
				}
			},
			expected: func() php.Definition {
				return php.Definition{
					BaseStage: emptyStage(),
					Stages: php.DerivedStageSet{
						"staging": php.DerivedStage{
							DeriveFrom: "prod",
						},
					},
				}
			},
		},
		"merge webserver with base": {
			base: func() php.Definition {
				configFile := "nginx.conf"
				return php.Definition{
					Webserver: &webserver.Definition{
						ConfigFile: &configFile,
					},
				}
			},
			overriding: func() php.Definition {
				configFile := "docker/nginx.conf"
				return php.Definition{
					Webserver: &webserver.Definition{
						ConfigFile: &configFile,
					},
				}
			},
			expected: func() php.Definition {
				configFile := "docker/nginx.conf"
				return php.Definition{
					Webserver: &webserver.Definition{
						ConfigFile:     &configFile,
						SystemPackages: map[string]string{},
					},
					BaseStage: emptyStage(),
					Stages:    php.DerivedStageSet{},
				}
			},
		},
		"merge webserver without base": {
			base: func() php.Definition {
				return php.Definition{}
			},
			overriding: func() php.Definition {
				configFile := "docker/nginx.conf"
				return php.Definition{
					Webserver: &webserver.Definition{
						ConfigFile: &configFile,
					},
				}
			},
			expected: func() php.Definition {
				configFile := "docker/nginx.conf"
				return php.Definition{
					Webserver: &webserver.Definition{
						ConfigFile:     &configFile,
						SystemPackages: map[string]string{},
					},
					BaseStage: emptyStage(),
					Stages:    php.DerivedStageSet{},
				}
			},
		},
	}

	for tcname := range testcases {
		tc := testcases[tcname]

		t.Run(tcname, func(t *testing.T) {
			t.Parallel()

			base := tc.base()
			new := base.Merge(tc.overriding())

			if diff := deep.Equal(new, tc.expected()); diff != nil {
				t.Fatal(diff)
			}

			if diff := deep.Equal(base, tc.base()); diff != nil {
				t.Fatalf("Base stages don't match: %v", diff)
			}
		})
	}
}

type mergeStageTC struct {
	base       func() php.Stage
	overriding php.Stage
	expected   func() php.Stage
}

func emptyStage() php.Stage {
	return php.Stage{
		ExternalFiles:  []llbutils.ExternalFile{},
		SystemPackages: map[string]string{},
		Extensions:     map[string]string{},
		GlobalDeps:     map[string]string{},
		ConfigFiles:    php.PHPConfigFiles{},
		Sources:        []string{},
		Integrations:   []string{},
		StatefulDirs:   []string{},
		PostInstall:    []string{},
	}
}

func initMergeExternalFilesWithBaseTC() mergeStageTC {
	return mergeStageTC{
		base: func() php.Stage {
			return php.Stage{
				ExternalFiles: []llbutils.ExternalFile{
					{
						URL:         "https://github.com/some/tool",
						Destination: "/usr/local/bin/some-tool",
						Mode:        0750,
					},
				},
			}
		},
		overriding: php.Stage{
			ExternalFiles: []llbutils.ExternalFile{
				{
					URL:         "https://github.com/some/other/tool",
					Destination: "/usr/local/bin/some-other-tool",
					Mode:        0750,
				},
			},
		},
		expected: func() php.Stage {
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
		base: func() php.Stage {
			return php.Stage{}
		},
		overriding: php.Stage{
			ExternalFiles: []llbutils.ExternalFile{
				{
					URL:         "https://github.com/some/other/tool",
					Destination: "/usr/local/bin/some-other-tool",
					Mode:        0750,
				},
			},
		},
		expected: func() php.Stage {
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
		base: func() php.Stage {
			return php.Stage{
				SystemPackages: map[string]string{
					"curl": "*",
				},
			}
		},
		overriding: php.Stage{
			SystemPackages: map[string]string{
				"chromium": "*",
			},
		},
		expected: func() php.Stage {
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
		base: func() php.Stage {
			return php.Stage{}
		},
		overriding: php.Stage{
			SystemPackages: map[string]string{
				"chromium": "*",
			},
		},
		expected: func() php.Stage {
			s := emptyStage()
			s.SystemPackages = map[string]string{
				"chromium": "*",
			}
			return s
		},
	}
}

func initMergeFPMWithBaseTC() mergeStageTC {
	baseFPM := true
	overridingFPM := false
	expectedFPM := false

	return mergeStageTC{
		base: func() php.Stage {
			return php.Stage{
				FPM: &baseFPM,
			}
		},
		overriding: php.Stage{
			FPM: &overridingFPM,
		},
		expected: func() php.Stage {
			s := emptyStage()
			s.FPM = &expectedFPM
			return s
		},
	}
}

func initMergeFPMWithoutBaseTC() mergeStageTC {
	overridingFPM := true
	expectedFPM := true

	return mergeStageTC{
		base: func() php.Stage {
			return php.Stage{}
		},
		overriding: php.Stage{
			FPM: &overridingFPM,
		},
		expected: func() php.Stage {
			s := emptyStage()
			s.FPM = &expectedFPM
			return s
		},
	}
}

func initMergeCommandWithBaseTC() mergeStageTC {
	baseCmd := []string{"bin/some-worker"}
	overridingCmd := []string{"bin/some-other-worker"}
	expectedCmd := []string{"bin/some-other-worker"}

	return mergeStageTC{
		base: func() php.Stage {
			return php.Stage{
				Command: &baseCmd,
			}
		},
		overriding: php.Stage{
			Command: &overridingCmd,
		},
		expected: func() php.Stage {
			s := emptyStage()
			s.Command = &expectedCmd
			return s
		},
	}
}

func initMergeCommandWithoutBaseTC() mergeStageTC {
	overridingCmd := []string{"bin/some-other-worker"}
	expectedCmd := []string{"bin/some-other-worker"}

	return mergeStageTC{
		base: func() php.Stage {
			return php.Stage{}
		},
		overriding: php.Stage{
			Command: &overridingCmd,
		},
		expected: func() php.Stage {
			s := emptyStage()
			s.Command = &expectedCmd
			return s
		},
	}
}

func initMergeExtensionsWithBaseTC() mergeStageTC {
	return mergeStageTC{
		base: func() php.Stage {
			return php.Stage{
				Extensions: map[string]string{
					"apcu": "*",
				},
			}
		},
		overriding: php.Stage{
			Extensions: map[string]string{
				"opcache": "*",
			},
		},
		expected: func() php.Stage {
			s := emptyStage()
			s.Extensions = map[string]string{
				"apcu":    "*",
				"opcache": "*",
			}
			return s
		},
	}
}

func initMergeExtensionsWithoutBaseTC() mergeStageTC {
	return mergeStageTC{
		base: func() php.Stage {
			return php.Stage{}
		},
		overriding: php.Stage{
			Extensions: map[string]string{
				"opcache": "*",
			},
		},
		expected: func() php.Stage {
			s := emptyStage()
			s.Extensions = map[string]string{
				"opcache": "*",
			}
			return s
		},
	}
}

func initMergeConfigFilesWithBaseTC() mergeStageTC {
	baseIniFile := "php.dev.ini"
	baseFpmConf := "fpm.dev.conf"
	overridingIniFile := "php.prod.ini"
	overridingFpmConf := "fpm.prod.conf"
	expectedIniFile := "php.prod.ini"
	expectedFpmConf := "fpm.prod.conf"

	return mergeStageTC{
		base: func() php.Stage {
			return php.Stage{
				ConfigFiles: php.PHPConfigFiles{
					IniFile:       &baseIniFile,
					FPMConfigFile: &baseFpmConf,
				},
			}
		},
		overriding: php.Stage{
			ConfigFiles: php.PHPConfigFiles{
				IniFile:       &overridingIniFile,
				FPMConfigFile: &overridingFpmConf,
			},
		},
		expected: func() php.Stage {
			s := emptyStage()
			s.ConfigFiles = php.PHPConfigFiles{
				IniFile:       &expectedIniFile,
				FPMConfigFile: &expectedFpmConf,
			}
			return s
		},
	}
}

func initMergeConfigFilesWithoutBaseTC() mergeStageTC {
	overridingIniFile := "php.prod.ini"
	overridingFpmConf := "fpm.prod.conf"
	expectedIniFile := "php.prod.ini"
	expectedFpmConf := "fpm.prod.conf"

	return mergeStageTC{
		base: func() php.Stage {
			return php.Stage{}
		},
		overriding: php.Stage{
			ConfigFiles: php.PHPConfigFiles{
				IniFile:       &overridingIniFile,
				FPMConfigFile: &overridingFpmConf,
			},
		},
		expected: func() php.Stage {
			s := emptyStage()
			s.ConfigFiles = php.PHPConfigFiles{
				IniFile:       &expectedIniFile,
				FPMConfigFile: &expectedFpmConf,
			}
			return s
		},
	}
}

func initMergeComposerDumpFlagsWithBaseTC() mergeStageTC {
	baseFlags := php.ComposerDumpFlags{
		ClassmapAuthoritative: true,
	}
	overridingFlags := php.ComposerDumpFlags{
		APCU: true,
	}
	expectedFlags := php.ComposerDumpFlags{
		APCU: true,
	}

	return mergeStageTC{
		base: func() php.Stage {
			return php.Stage{
				ComposerDumpFlags: &baseFlags,
			}
		},
		overriding: php.Stage{
			ComposerDumpFlags: &overridingFlags,
		},
		expected: func() php.Stage {
			s := emptyStage()
			s.ComposerDumpFlags = &expectedFlags
			return s
		},
	}
}

func initMergeComposerDumpFlagsWithoutBaseTC() mergeStageTC {
	overridingFlags := php.ComposerDumpFlags{
		APCU: true,
	}
	expectedFlags := php.ComposerDumpFlags{
		APCU: true,
	}

	return mergeStageTC{
		base: func() php.Stage {
			return php.Stage{}
		},
		overriding: php.Stage{
			ComposerDumpFlags: &overridingFlags,
		},
		expected: func() php.Stage {
			s := emptyStage()
			s.ComposerDumpFlags = &expectedFlags
			return s
		},
	}
}

func initMergeSourcesWithBaseTC() mergeStageTC {
	return mergeStageTC{
		base: func() php.Stage {
			return php.Stage{
				Sources: []string{"src/"},
			}
		},
		overriding: php.Stage{
			Sources: []string{"bin/worker"},
		},
		expected: func() php.Stage {
			s := emptyStage()
			s.Sources = []string{"src/", "bin/worker"}
			return s
		},
	}
}

func initMergeSourcesWithoutBaseTC() mergeStageTC {
	return mergeStageTC{
		base: func() php.Stage {
			return php.Stage{}
		},
		overriding: php.Stage{
			Sources: []string{"bin/worker"},
		},
		expected: func() php.Stage {
			s := emptyStage()
			s.Sources = []string{"bin/worker"}
			return s
		},
	}
}

func initMergeIntegrationsWithBaseTC() mergeStageTC {
	return mergeStageTC{
		base: func() php.Stage {
			return php.Stage{
				Integrations: []string{"blackfire"},
			}
		},
		overriding: php.Stage{
			Integrations: []string{"some-other"},
		},
		expected: func() php.Stage {
			s := emptyStage()
			s.Integrations = []string{"blackfire", "some-other"}
			return s
		},
	}
}

func initMergeIntegrationsWithoutBaseTC() mergeStageTC {
	return mergeStageTC{
		base: func() php.Stage {
			return php.Stage{}
		},
		overriding: php.Stage{
			Integrations: []string{"some-other"},
		},
		expected: func() php.Stage {
			s := emptyStage()
			s.Integrations = []string{"some-other"}
			return s
		},
	}
}

func initMergeStatefulDirsWithBaseTC() mergeStageTC {
	return mergeStageTC{
		base: func() php.Stage {
			return php.Stage{
				StatefulDirs: []string{"var/sessions/"},
			}
		},
		overriding: php.Stage{
			StatefulDirs: []string{"public/uploads/"},
		},
		expected: func() php.Stage {
			s := emptyStage()
			s.StatefulDirs = []string{"var/sessions/", "public/uploads/"}
			return s
		},
	}
}

func initMergeStatefulDirsWithoutBaseTC() mergeStageTC {
	return mergeStageTC{
		base: func() php.Stage {
			return php.Stage{}
		},
		overriding: php.Stage{
			StatefulDirs: []string{"public/uploads/"},
		},
		expected: func() php.Stage {
			s := emptyStage()
			s.StatefulDirs = []string{"public/uploads/"}
			return s
		},
	}
}

func initMergeHealthcheckWithBaseTC() mergeStageTC {
	baseHealthcheck := true
	overridingHealthcheck := false
	expectedHealthcheck := false

	return mergeStageTC{
		base: func() php.Stage {
			return php.Stage{
				Healthcheck: &baseHealthcheck,
			}
		},
		overriding: php.Stage{
			Healthcheck: &overridingHealthcheck,
		},
		expected: func() php.Stage {
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
		base: func() php.Stage {
			return php.Stage{}
		},
		overriding: php.Stage{
			Healthcheck: &overridingHealthcheck,
		},
		expected: func() php.Stage {
			s := emptyStage()
			s.Healthcheck = &expectedHealthcheck
			return s
		},
	}
}

func initMergePostInstallWithBaseTC() mergeStageTC {
	return mergeStageTC{
		base: func() php.Stage {
			return php.Stage{
				PostInstall: []string{"some-step"},
			}
		},
		overriding: php.Stage{
			PostInstall: []string{"some-other-step"},
		},
		expected: func() php.Stage {
			s := emptyStage()
			s.PostInstall = []string{"some-step", "some-other-step"}
			return s
		},
	}
}

func initMergePostInstallWithoutBaseTC() mergeStageTC {
	return mergeStageTC{
		base: func() php.Stage {
			return php.Stage{}
		},
		overriding: php.Stage{
			PostInstall: []string{"some-other"},
		},
		expected: func() php.Stage {
			s := emptyStage()
			s.PostInstall = []string{"some-other"}
			return s
		},
	}
}

func initMergeGlobalDepsWithBaseTC() mergeStageTC {
	return mergeStageTC{
		base: func() php.Stage {
			return php.Stage{
				GlobalDeps: map[string]string{
					"symfony/flex": "*",
				},
			}
		},
		overriding: php.Stage{
			GlobalDeps: map[string]string{
				"symfony/flex":      "1.6.0",
				"hirak/prestissimo": "0.3.9",
			},
		},
		expected: func() php.Stage {
			stage := emptyStage()
			stage.GlobalDeps = map[string]string{
				"symfony/flex":      "1.6.0",
				"hirak/prestissimo": "0.3.9",
			}

			return stage
		},
	}
}

func initMergeGlobalDepsWithoutBaseTC() mergeStageTC {
	return mergeStageTC{
		base: func() php.Stage {
			return php.Stage{
				GlobalDeps: map[string]string{},
			}
		},
		overriding: php.Stage{
			GlobalDeps: map[string]string{
				"symfony/flex":      "1.6.0",
				"hirak/prestissimo": "0.3.9",
			},
		},
		expected: func() php.Stage {
			stage := emptyStage()
			stage.GlobalDeps = map[string]string{
				"symfony/flex":      "1.6.0",
				"hirak/prestissimo": "0.3.9",
			}

			return stage
		},
	}
}

func TestStageMerge(t *testing.T) {
	testcases := map[string]func() mergeStageTC{
		"merge external files with base":         initMergeExternalFilesWithBaseTC,
		"merge external files without base":      initMergeExternalFilesWithoutBaseTC,
		"merge system packages with base":        initMergeSystemPackagesWithBaseTC,
		"merge system packages without base":     initMergeSystemPackagesWithoutBaseTC,
		"merge fpm with base":                    initMergeFPMWithBaseTC,
		"merge fpm without base":                 initMergeFPMWithoutBaseTC,
		"merge command with base":                initMergeCommandWithBaseTC,
		"merge command without base":             initMergeCommandWithoutBaseTC,
		"merge extensions with base":             initMergeExtensionsWithBaseTC,
		"merge extensions without base":          initMergeExtensionsWithoutBaseTC,
		"merge global deps with base":            initMergeGlobalDepsWithBaseTC,
		"merge global deps without base":         initMergeGlobalDepsWithoutBaseTC,
		"merge config files with base":           initMergeConfigFilesWithBaseTC,
		"merge config files without base":        initMergeConfigFilesWithoutBaseTC,
		"merge composer dump flags with base":    initMergeComposerDumpFlagsWithBaseTC,
		"merge composer dump flags without base": initMergeComposerDumpFlagsWithoutBaseTC,
		"merge sources with base":                initMergeSourcesWithBaseTC,
		"merge sources without base":             initMergeSourcesWithoutBaseTC,
		"merge integrations with base":           initMergeIntegrationsWithBaseTC,
		"merge integrations without base":        initMergeIntegrationsWithoutBaseTC,
		"merge stateful dirs with base":          initMergeStatefulDirsWithBaseTC,
		"merge stateful dirs without base":       initMergeStatefulDirsWithoutBaseTC,
		"merge healthcheck with base":            initMergeHealthcheckWithBaseTC,
		"merge healthcheck without base":         initMergeHealthcheckWithoutBaseTC,
		"merge post install with base":           initMergePostInstallWithBaseTC,
		"merge post install without base":        initMergePostInstallWithoutBaseTC,
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
