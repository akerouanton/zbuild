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
	return newDefinitionTC{
		file:     "testdata/def/definition.yml",
		lockFile: "testdata/def/definition.lock",
		expected: webserver.Definition{
			Type:    "nginx",
			Version: "latest",
			Alpine:  true,
			ConfigFiles: builddef.PathsMap{
				"./docker/nginx.conf": "nginx.conf",
			},
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
				BaseImage: "docker.io/library/nginx:alpine@sha256",
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
			ConfigFiles: builddef.PathsMap{},
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

func loadDefLocks(t *testing.T, filepath string) builddef.RawLocks {
	raw := loadRawTestdata(t, filepath)

	var locks builddef.RawLocks
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
	if *flagTestdata {
		return
	}

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
					ConfigFiles:    builddef.PathsMap{},
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
					ConfigFiles:    builddef.PathsMap{},
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
					ConfigFiles: builddef.PathsMap{},
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
					ConfigFiles: builddef.PathsMap{},
					SystemPackages: &builddef.VersionMap{
						"curl":            "7.64.0-4",
						"ca-certificates": "*",
					},
				}
			},
		},
		"merge config files with base": {
			base: func() webserver.Definition {
				return webserver.Definition{
					ConfigFiles: builddef.PathsMap{
						"./docker/nginx.dev.conf": "nginx.conf",
					},
				}
			},
			overriding: func() webserver.Definition {
				return webserver.Definition{
					ConfigFiles: builddef.PathsMap{
						"./docker/nginx.prod.conf": "nginx.conf",
					},
				}
			},
			expected: func() webserver.Definition {
				return webserver.Definition{
					SystemPackages: &builddef.VersionMap{},
					ConfigFiles: builddef.PathsMap{
						"./docker/nginx.prod.conf": "nginx.conf",
					},
				}
			},
		},
		"merge config file without base": {
			base: func() webserver.Definition {
				return webserver.Definition{}
			},
			overriding: func() webserver.Definition {
				return webserver.Definition{
					ConfigFiles: builddef.PathsMap{
						"./docker/nginx.prod.conf": "nginx.conf",
					},
				}
			},
			expected: func() webserver.Definition {
				return webserver.Definition{
					ConfigFiles: builddef.PathsMap{
						"./docker/nginx.prod.conf": "nginx.conf",
					},
					SystemPackages: &builddef.VersionMap{},
				}
			},
		},
		"ignore nil config file": {
			base: func() webserver.Definition {
				return webserver.Definition{
					ConfigFiles: builddef.PathsMap{
						"./docker/nginx.dev.conf": "nginx.conf",
					},
				}
			},
			overriding: func() webserver.Definition {
				return webserver.Definition{}
			},
			expected: func() webserver.Definition {
				return webserver.Definition{
					SystemPackages: &builddef.VersionMap{},
					ConfigFiles: builddef.PathsMap{
						"./docker/nginx.dev.conf": "nginx.conf",
					},
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
					ConfigFiles:    builddef.PathsMap{},
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
					ConfigFiles:    builddef.PathsMap{},
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
					ConfigFiles:    builddef.PathsMap{},
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
					ConfigFiles:    builddef.PathsMap{},
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
					ConfigFiles:    builddef.PathsMap{},
				}
			},
		},
		"merge version with base": {
			base: func() webserver.Definition {
				return webserver.Definition{
					Version: "latest",
				}
			},
			overriding: func() webserver.Definition {
				return webserver.Definition{
					Version: "1.17.8",
				}
			},
			expected: func() webserver.Definition {
				return webserver.Definition{
					Version:        "1.17.8",
					SystemPackages: &builddef.VersionMap{},
					ConfigFiles:    builddef.PathsMap{},
				}
			},
		},
		"merge version without base": {
			base: func() webserver.Definition {
				return webserver.Definition{}
			},
			overriding: func() webserver.Definition {
				return webserver.Definition{
					Version: "1.17.8",
				}
			},
			expected: func() webserver.Definition {
				return webserver.Definition{
					Version:        "1.17.8",
					SystemPackages: &builddef.VersionMap{},
					ConfigFiles:    builddef.PathsMap{},
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
