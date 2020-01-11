package webserver

import (
	"fmt"
	"time"

	"github.com/NiR-/zbuild/pkg/builddef"
	"github.com/mitchellh/mapstructure"
	"golang.org/x/xerrors"
)

func DefaultDefinition() Definition {
	healthcheck := defaultHealthcheck
	return Definition{
		Type:           WebserverType("nginx"),
		SystemPackages: &builddef.VersionMap{},
		Healthcheck:    &healthcheck,
	}
}

var defaultHealthcheck = builddef.HealthcheckConfig{
	HealthcheckHTTP: &builddef.HealthcheckHTTP{
		Path: "/_status",
	},
	Type:     builddef.HealthcheckTypeHTTP,
	Interval: 10 * time.Second,
	Timeout:  1 * time.Second,
	Retries:  3,
}

func decodeDefinition(raw map[string]interface{}) (Definition, error) {
	decodeHook := mapstructure.ComposeDecodeHookFunc(
		builddef.DecodeBoolToHealthcheck(defaultHealthcheck),
		mapstructure.StringToTimeDurationHookFunc())

	var def Definition
	decoderConf := mapstructure.DecoderConfig{
		ErrorUnused:      true,
		WeaklyTypedInput: true,
		Result:           &def,
		DecodeHook:       decodeHook,
	}
	decoder, err := mapstructure.NewDecoder(&decoderConf)
	if err != nil {
		return def, err
	}

	if err := decoder.Decode(raw); err != nil {
		return def, xerrors.Errorf("could not decode build manifest: %w", err)
	}

	def = DefaultDefinition().Merge(def)
	return def, nil
}

func decodeDefinitionLocks(raw map[string]interface{}) (DefinitionLocks, error) {
	var locks DefinitionLocks
	decoderConf := mapstructure.DecoderConfig{
		ErrorUnused:      true,
		WeaklyTypedInput: true,
		Result:           &locks,
	}
	decoder, err := mapstructure.NewDecoder(&decoderConf)
	if err != nil {
		return locks, err
	}

	if err := decoder.Decode(raw); err != nil {
		return locks, xerrors.Errorf("could not decode lock manifest: %w", err)
	}

	return locks, nil
}

func NewKind(genericDef *builddef.BuildDef) (Definition, error) {
	def, err := decodeDefinition(genericDef.RawConfig)
	if err != nil {
		return def, err
	}

	def.Locks, err = decodeDefinitionLocks(genericDef.RawLocks)
	if err != nil {
		return def, err
	}

	if def.Healthcheck.IsEnabled() {
		def.SystemPackages.Add("curl", "*")
	}

	return def, def.Validate()
}

type Definition struct {
	Type           WebserverType               `mapstructure:"type"`
	SystemPackages *builddef.VersionMap        `mapstructure:"system_packages"`
	ConfigFile     *string                     `mapstructure:"config_file"`
	Healthcheck    *builddef.HealthcheckConfig `mapstructure:"healthcheck"`
	Assets         []AssetToCopy               `mapstructure:"assets"`
	Locks          DefinitionLocks             `mapstructure:"-"`
}

func (def Definition) Validate() error {
	if def.Type.IsEmpty() {
		return xerrors.New("webserver build manifest has no type nor base_image parameters.")
	}

	if !def.Healthcheck.Type.IsValid([]string{"http", "cmd"}) {
		return xerrors.Errorf("healthcheck type %q is not supported",
			def.Healthcheck.Type)
	}

	return nil
}

func (def Definition) Copy() Definition {
	new := Definition{
		Type:           def.Type,
		SystemPackages: def.SystemPackages.Copy(),
		Assets:         def.Assets,
	}

	if def.ConfigFile != nil {
		configFile := *def.ConfigFile
		new.ConfigFile = &configFile
	}
	if def.Healthcheck != nil {
		healthcheck := *def.Healthcheck
		new.Healthcheck = &healthcheck
	}

	return new
}

func (base Definition) Merge(overriding Definition) Definition {
	new := base.Copy()
	new.Assets = append(new.Assets, overriding.Assets...)
	new.SystemPackages.Merge(overriding.SystemPackages)

	if !overriding.Type.IsEmpty() {
		new.Type = overriding.Type
	}
	if overriding.ConfigFile != nil {
		configFile := *overriding.ConfigFile
		new.ConfigFile = &configFile
	}
	if overriding.Healthcheck != nil {
		healthcheck := *overriding.Healthcheck
		new.Healthcheck = &healthcheck
	}

	return new
}

type WebserverType string

func (t WebserverType) IsValid() bool {
	return string(t) == "nginx"
}

func (t WebserverType) IsEmpty() bool {
	return string(t) == ""
}

func (t WebserverType) ConfigPath() string {
	switch string(t) {
	case "nginx":
		return "/etc/nginx/nginx.conf"
	}

	panic(fmt.Sprintf("Webserver type %q is not supported.", string(t)))
}

func (t WebserverType) BaseImage() string {
	switch string(t) {
	case "nginx":
		return "docker.io/library/nginx:latest"
	}

	panic(fmt.Sprintf("Webserver type %q is not supported.", string(t)))
}

type AssetToCopy struct {
	From string `mapstructure:"from"`
	To   string `mapstructure:"to"`
}

func ExtractRawDefFromParent(parentDef map[string]interface{}) map[string]interface{} {
	rawDef := map[string]interface{}{}
	webserver, ok := parentDef["webserver"]
	if !ok {
		return rawDef
	}

	for k, v := range webserver.(map[interface{}]interface{}) {
		rawDef[k.(string)] = v.(interface{})
	}

	return rawDef
}
