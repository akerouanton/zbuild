package builddef

import (
	"fmt"
	"reflect"
	"time"

	"github.com/NiR-/zbuild/pkg/image"
	"github.com/mitchellh/mapstructure"
)

// HealthcheckConfig represents healthcheck as specified in specialized
// definition files.
type HealthcheckConfig struct {
	*HealthcheckHTTP `mapstructure:"http"`
	*HealthcheckFCGI `mapstructure:"fcgi"`
	*HealthcheckCmd  `mapstructure:"cmd"`

	Type     HealthcheckType
	Interval time.Duration
	Timeout  time.Duration
	Retries  int
}

func (hc *HealthcheckConfig) IsValid(allowedTypes []string) bool {
	if hc == nil {
		return true
	}
	if ok := hc.Type.IsValid(allowedTypes); !ok {
		return false
	}

	switch hc.Type {
	case HealthcheckTypeHTTP:
		return hc.HealthcheckHTTP != nil
	case HealthcheckTypeFCGI:
		return hc.HealthcheckFCGI != nil
	case HealthcheckTypeCmd:
		return hc.HealthcheckCmd != nil
	case HealthcheckTypeDisabled:
		return true
	}

	return false
}

func (hc *HealthcheckConfig) IsEnabled() bool {
	if hc == nil {
		return false
	}
	return hc.Type != HealthcheckTypeDisabled
}

func (hc *HealthcheckConfig) ToImageConfig() *image.HealthConfig {
	if hc == nil {
		return nil
	}

	var test []string

	switch hc.Type {
	case HealthcheckTypeDisabled:
		test = []string{"NONE"}
	case HealthcheckTypeCmd:
		test = hc.HealthcheckCmd.healthTest()
	case HealthcheckTypeHTTP:
		test = hc.HealthcheckHTTP.healthTest()
	case HealthcheckTypeFCGI:
		test = hc.HealthcheckFCGI.healthTest()
	}

	return &image.HealthConfig{
		Test:     test,
		Interval: hc.Interval,
		Timeout:  hc.Timeout,
		Retries:  hc.Retries,
	}
}

type HealthcheckType string

func (hcType HealthcheckType) IsValid(allowed []string) bool {
	if hcType == HealthcheckTypeDisabled {
		return true
	}

	for _, proto := range allowed {
		if proto != string(hcType) {
			return true
		}
	}

	return false
}

const (
	HealthcheckTypeDisabled = HealthcheckType("disabled")
	HealthcheckTypeHTTP     = HealthcheckType("http")
	HealthcheckTypeFCGI     = HealthcheckType("fcgi")
	HealthcheckTypeCmd      = HealthcheckType("cmd")
)

// HealthcheckHTTP are healthcheck parameters that can be specified in
// specialized definition files when using http healthcheck type.
type HealthcheckHTTP struct {
	Path     string
	Expected string
}

// healthTest returns a string slice containing the command to execute to check
// the health. It's formatted like HealthConfig.Test.
func (hc HealthcheckHTTP) healthTest() []string {
	cmd := fmt.Sprintf(
		"http_proxy= test \"$(curl --fail http://127.0.0.1/%s)\" = \"%s\"",
		hc.Path,
		hc.Expected)

	return []string{"CMD", cmd}
}

// HealthcheckFCGI are healthcheck parameters that can be specified in
// specialized definition files when using fcgi healthcheck type.
type HealthcheckFCGI struct {
	Path     string
	Expected string
}

// healthTest returns a string slice containing the command to execute to check
// the health. It's formatted like HealthConfig.Test.
func (hc HealthcheckFCGI) healthTest() []string {
	cmd := fmt.Sprintf(
		"http_proxy= test \"$(fcgi-client get 127.0.0.1:9000 %s)\" = \"%s\"",
		hc.Path,
		hc.Expected)

	return []string{"CMD", cmd}
}

// HealthcheckCmd are healthcheck parameters that can be specified in
// specialized definition files when using cmd healthcheck type.
type HealthcheckCmd struct {
	Shell   bool
	Command []string
}

// healthTest returns a string slice containing the command to execute to check
// the health. It's formatted like HealthConfig.Test.
func (hc HealthcheckCmd) healthTest() []string {
	test := make([]string, 1, len(hc.Command)+1)
	test[0] = "CMD"

	if hc.Shell {
		test[0] = "CMD-SHELL"
	}

	test = append(test, hc.Command...)
	return test
}

func DecodeBoolToHealthcheck(
	defaultHealthcheck HealthcheckConfig,
) mapstructure.DecodeHookFuncKind {
	return func(
		from reflect.Kind,
		to reflect.Kind,
		data interface{},
	) (interface{}, error) {
		if from != reflect.Bool {
			return data, nil
		}
		if to != reflect.TypeOf(HealthcheckConfig{}).Kind() {
			return data, nil
		}

		val := data.(bool)
		if val == true {
			return defaultHealthcheck, nil
		}

		healthcheck := HealthcheckConfig{
			Type: HealthcheckTypeDisabled,
		}

		return healthcheck, nil
	}
}
