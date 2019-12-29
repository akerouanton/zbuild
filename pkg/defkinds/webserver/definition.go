package webserver

import (
	"fmt"

	"github.com/NiR-/zbuild/pkg/builddef"
	"github.com/mitchellh/mapstructure"
	"golang.org/x/xerrors"
	"gopkg.in/yaml.v2"
)

func DefaultDefinition() Definition {
	healthcheck := true
	return Definition{
		Type:           "nginx",
		SystemPackages: map[string]string{},
		Healthcheck:    &healthcheck,
	}
}

func NewKind(genericDef *builddef.BuildDef) (Definition, error) {
	def := DefaultDefinition()
	decoderConf := mapstructure.DecoderConfig{
		ErrorUnused:      true,
		WeaklyTypedInput: true,
		Result:           &def,
	}
	decoder, err := mapstructure.NewDecoder(&decoderConf)
	if err != nil {
		return def, err
	}

	if err := decoder.Decode(genericDef.RawConfig); err != nil {
		err := xerrors.Errorf("could not decode build manifest: %w", err)
		return def, err
	}

	if err := yaml.Unmarshal(genericDef.RawLocks, &def.Locks); err != nil {
		return def, xerrors.Errorf("could not decode lock manifest: %w", err)
	}

	if def.SystemPackages == nil {
		def.SystemPackages = map[string]string{}
	}

	if def.Healthcheck != nil && *def.Healthcheck {
		def.SystemPackages["curl"] = "*"
	}

	return def, def.Validate()
}

type Definition struct {
	Type           WebserverType     `mapstructure:"type"`
	SystemPackages map[string]string `mapstructure:"system_packages"`
	ConfigFile     *string           `mapstructure:"config_file"`
	Healthcheck    *bool             `mapstructure:"healthcheck"`
	Assets         []AssetToCopy     `mapstructure:"assets"`
	Locks          DefinitionLocks   `mapstructure:"-"`
}

func (def Definition) Validate() error {
	if def.Type.IsEmpty() {
		return xerrors.New("webserver build manifest has no type nor base_image parameters.")
	}

	return nil
}

func (base Definition) Merge(overriding Definition) Definition {
	new := base
	new.Assets = append(new.Assets, overriding.Assets...)
	new.Locks = overriding.Locks

	if !overriding.Type.IsEmpty() {
		new.Type = overriding.Type
	}
	for k, v := range overriding.SystemPackages {
		new.SystemPackages[k] = v
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

func (def Definition) RawConfig() map[string]interface{} {
	raw := map[string]interface{}{
		"type":            def.Type,
		"system_packages": def.SystemPackages,
		"assets":          def.Assets,
	}

	if def.ConfigFile != nil {
		raw["config_file"] = def.ConfigFile
	}
	if def.Healthcheck != nil {
		raw["healthcheck"] = def.Healthcheck
	}

	return raw
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
