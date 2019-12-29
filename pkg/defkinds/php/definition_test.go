package php_test

import (
	"errors"
	"testing"

	"github.com/NiR-/zbuild/pkg/builddef"
	"github.com/NiR-/zbuild/pkg/defkinds/php"
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
			Infer:         false,
			Stages: map[string]php.DerivedStage{
				"dev": {
					DeriveFrom: "base",
					Dev:        &isDev,
				},
				"prod": {
					DeriveFrom: "base",
					Dev:        &isNotDev,
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
	isDev := true
	isFPM := true
	isNotFPM := false
	workerCmd := []string{"bin/worker"}

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
			Infer:         true,
			Stages: map[string]php.DerivedStage{
				"dev": {
					DeriveFrom: "",
					Dev:        &isDev,
					Stage: php.Stage{
						ConfigFiles: php.PHPConfigFiles{
							IniFile: &iniDevFile,
						},
					},
				},
				"prod": {
					DeriveFrom: "",
					Stage: php.Stage{
						ConfigFiles: php.PHPConfigFiles{
							IniFile: &iniProdFile,
						},
						Healthcheck:  &healthcheckEnabled,
						Integrations: []string{"blackfire"},
					},
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
				ExternalFiles:  []llbutils.ExternalFile{},
				SystemPackages: map[string]string{},
				FPM:            &isFPM,
				Extensions: map[string]string{
					"intl":      "*",
					"pdo_mysql": "*",
					"soap":      "*",
				},
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
				Integrations: []string{},
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

			stageDef, err := def.ResolveStageDefinition(tc.stage, tc.composerLockLoader)
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
