package webserver_test

import (
	"io/ioutil"
	"testing"
	"time"

	"github.com/NiR-/zbuild/pkg/builddef"
	"github.com/NiR-/zbuild/pkg/defkinds/webserver"
	"github.com/go-test/deep"
	"gopkg.in/yaml.v2"
)

type newDefinitionTC struct {
	file        string
	lockFile    string
	expected    webserver.Definition
	expectedErr error
}

func initSuccessfullyParseRawDefinitionTC() newDefinitionTC {
	configFile := "./docker/nginx.conf"
	return newDefinitionTC{
		file:     "testdata/locks/definition.yml",
		lockFile: "testdata/locks/definition.lock",
		expected: webserver.Definition{
			Type:       "nginx",
			ConfigFile: &configFile,
			Healthcheck: &builddef.HealthcheckConfig{
				HealthcheckHTTP: &builddef.HealthcheckHTTP{
					Path:     "/_ping",
					Expected: "pong",
				},
				Type:     builddef.HealthcheckTypeHTTP,
				Interval: 10 * time.Second,
				Timeout:  1 * time.Second,
				Retries:  3,
			},
			SystemPackages: &builddef.VersionMap{
				"curl": "*",
			},
			Assets: []webserver.AssetToCopy{
				{
					From: "/app/public",
					To:   "/app/public",
				},
			},
			Locks: webserver.DefinitionLocks{
				BaseImage: "docker.io/library/nginx:latest@sha256",
				SystemPackages: map[string]string{
					"curl": "7.64.0-4",
				},
			},
		},
	}
}

func initParseDefinitionWithCustomHealthcheckTC() newDefinitionTC {
	return newDefinitionTC{
		file: "testdata/def/with-custom-healthcheck.yml",
		expected: webserver.Definition{
			Type: "nginx",
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
			SystemPackages: &builddef.VersionMap{
				"curl": "*",
			},
		},
	}
}

func TestNewKind(t *testing.T) {
	if *flagTestdata {
		return
	}

	testcases := map[string]func() newDefinitionTC{
		"parse definition":                         initSuccessfullyParseRawDefinitionTC,
		"parse definition with custom healthcheck": initParseDefinitionWithCustomHealthcheckTC,
	}

	for tcname := range testcases {
		tcinit := testcases[tcname]

		t.Run(tcname, func(t *testing.T) {
			t.Parallel()
			tc := tcinit()

			generic := loadBuildDef(t, tc.file)
			if tc.lockFile != "" {
				generic.RawLocks = loadDefLocks(t, tc.lockFile)
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

func loadDefLocks(t *testing.T, filepath string) map[string]interface{} {
	raw := loadRawTestdata(t, filepath)

	var locks map[string]interface{}
	if err := yaml.Unmarshal(raw, &locks); err != nil {
		t.Fatal(err)
	}

	return locks
}

type mergeDefinitionTC struct {
	base       func() webserver.Definition
	overriding func() webserver.Definition
	expected   func() webserver.Definition
}

func TestDefinitionMerge(t *testing.T) {
	testcases := map[string]mergeDefinitionTC{
		"merge type with base": {
			base: func() webserver.Definition {
				return webserver.Definition{
					Type: webserver.WebserverType("nginx"),
				}
			},
			overriding: func() webserver.Definition {
				return webserver.Definition{
					Type: webserver.WebserverType("caddy"),
				}
			},
			expected: func() webserver.Definition {
				return webserver.Definition{
					Type:           webserver.WebserverType("caddy"),
					SystemPackages: &builddef.VersionMap{},
				}
			},
		},
		"merge type without base": {
			base: func() webserver.Definition {
				return webserver.Definition{}
			},
			overriding: func() webserver.Definition {
				return webserver.Definition{
					Type: webserver.WebserverType("caddy"),
				}
			},
			expected: func() webserver.Definition {
				return webserver.Definition{
					Type:           webserver.WebserverType("caddy"),
					SystemPackages: &builddef.VersionMap{},
				}
			},
		},
		"merge system packages with base": {
			base: func() webserver.Definition {
				return webserver.Definition{
					SystemPackages: &builddef.VersionMap{
						"curl": "*",
					},
				}
			},
			overriding: func() webserver.Definition {
				return webserver.Definition{
					SystemPackages: &builddef.VersionMap{
						"curl":            "7.64.0-4",
						"ca-certificates": "*",
					},
				}
			},
			expected: func() webserver.Definition {
				return webserver.Definition{
					SystemPackages: &builddef.VersionMap{
						"curl":            "7.64.0-4",
						"ca-certificates": "*",
					},
				}
			},
		},
		"merge system packages without base": {
			base: func() webserver.Definition {
				return webserver.Definition{}
			},
			overriding: func() webserver.Definition {
				return webserver.Definition{
					SystemPackages: &builddef.VersionMap{
						"curl":            "7.64.0-4",
						"ca-certificates": "*",
					},
				}
			},
			expected: func() webserver.Definition {
				return webserver.Definition{
					SystemPackages: &builddef.VersionMap{
						"curl":            "7.64.0-4",
						"ca-certificates": "*",
					},
				}
			},
		},
		"merge config file with base": {
			base: func() webserver.Definition {
				configFile := "nginx.conf"
				return webserver.Definition{
					ConfigFile: &configFile,
				}
			},
			overriding: func() webserver.Definition {
				configFile := "docker/nginx.conf"
				return webserver.Definition{
					ConfigFile: &configFile,
				}
			},
			expected: func() webserver.Definition {
				configFile := "docker/nginx.conf"
				return webserver.Definition{
					ConfigFile:     &configFile,
					SystemPackages: &builddef.VersionMap{},
				}
			},
		},
		"merge config file without base": {
			base: func() webserver.Definition {
				return webserver.Definition{}
			},
			overriding: func() webserver.Definition {
				configFile := "docker/nginx.conf"
				return webserver.Definition{
					ConfigFile: &configFile,
				}
			},
			expected: func() webserver.Definition {
				configFile := "docker/nginx.conf"
				return webserver.Definition{
					ConfigFile:     &configFile,
					SystemPackages: &builddef.VersionMap{},
				}
			},
		},
		"ignore nil config file": {
			base: func() webserver.Definition {
				configFile := "nginx.conf"
				return webserver.Definition{
					ConfigFile: &configFile,
				}
			},
			overriding: func() webserver.Definition {
				return webserver.Definition{}
			},
			expected: func() webserver.Definition {
				configFile := "nginx.conf"
				return webserver.Definition{
					ConfigFile:     &configFile,
					SystemPackages: &builddef.VersionMap{},
				}
			},
		},
		"merge healthcheck with base": {
			base: func() webserver.Definition {
				return webserver.Definition{
					Healthcheck: &builddef.HealthcheckConfig{
						HealthcheckHTTP: &builddef.HealthcheckHTTP{
							Path: "/_ping",
						},
						Type:     builddef.HealthcheckTypeHTTP,
						Interval: 10 * time.Second,
						Timeout:  1 * time.Second,
						Retries:  3,
					},
				}
			},
			overriding: func() webserver.Definition {
				return webserver.Definition{
					Healthcheck: &builddef.HealthcheckConfig{
						Type: builddef.HealthcheckTypeDisabled,
					},
				}
			},
			expected: func() webserver.Definition {
				return webserver.Definition{
					Healthcheck: &builddef.HealthcheckConfig{
						Type: builddef.HealthcheckTypeDisabled,
					},
					SystemPackages: &builddef.VersionMap{},
				}
			},
		},
		"merge healthcheck without base": {
			base: func() webserver.Definition {
				return webserver.Definition{}
			},
			overriding: func() webserver.Definition {
				return webserver.Definition{
					Healthcheck: &builddef.HealthcheckConfig{
						Type: builddef.HealthcheckTypeDisabled,
					},
				}
			},
			expected: func() webserver.Definition {
				return webserver.Definition{
					Healthcheck: &builddef.HealthcheckConfig{
						Type: builddef.HealthcheckTypeDisabled,
					},
					SystemPackages: &builddef.VersionMap{},
				}
			},
		},
		"ignore nil healthcheck": {
			base: func() webserver.Definition {
				return webserver.Definition{
					Healthcheck: &builddef.HealthcheckConfig{
						Type: builddef.HealthcheckTypeDisabled,
					},
				}
			},
			overriding: func() webserver.Definition {
				return webserver.Definition{}
			},
			expected: func() webserver.Definition {
				return webserver.Definition{
					Healthcheck: &builddef.HealthcheckConfig{
						Type: builddef.HealthcheckTypeDisabled,
					},
					SystemPackages: &builddef.VersionMap{},
				}
			},
		},
		"merge assets with base": {
			base: func() webserver.Definition {
				return webserver.Definition{
					Assets: []webserver.AssetToCopy{
						{From: "public/", To: "public/"},
					},
				}
			},
			overriding: func() webserver.Definition {
				return webserver.Definition{
					Assets: []webserver.AssetToCopy{
						{From: "web/", To: "web/"},
					},
				}
			},
			expected: func() webserver.Definition {
				return webserver.Definition{
					Assets: []webserver.AssetToCopy{
						{From: "public/", To: "public/"},
						{From: "web/", To: "web/"},
					},
					SystemPackages: &builddef.VersionMap{},
				}
			},
		},
		"merge assets without base": {
			base: func() webserver.Definition {
				return webserver.Definition{}
			},
			overriding: func() webserver.Definition {
				return webserver.Definition{
					Assets: []webserver.AssetToCopy{
						{From: "web/", To: "web/"},
					},
				}
			},
			expected: func() webserver.Definition {
				return webserver.Definition{
					Assets: []webserver.AssetToCopy{
						{From: "web/", To: "web/"},
					},
					SystemPackages: &builddef.VersionMap{},
				}
			},
		},
	}

	for tcname := range testcases {
		tc := testcases[tcname]

		t.Run(tcname, func(t *testing.T) {
			base := tc.base()
			new := base.Merge(tc.overriding())

			if diff := deep.Equal(new, tc.expected()); diff != nil {
				t.Fatal(diff)
			}

			if diff := deep.Equal(base, tc.base()); diff != nil {
				t.Fatalf("Base definition don't match: %v", diff)
			}
		})
	}
}
