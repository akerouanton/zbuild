package webserver

import (
	"fmt"

	"github.com/NiR-/zbuild/pkg/builddef"
	"github.com/mitchellh/mapstructure"
	"golang.org/x/xerrors"
	"gopkg.in/yaml.v2"
)

func defaultDefinition() Definition {
	return Definition{
		Type: "nginx",
	}
}

func NewKind(genericDef *builddef.BuildDef) (Definition, error) {
	def := Definition{
		Type:           WebserverType("nginx"),
		SystemPackages: map[string]string{},
		Healthcheck:    true,
	}

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

	if def.Healthcheck {
		def.SystemPackages["curl"] = "*"
	}

	return def, def.Validate()
}

type Definition struct {
	Type           WebserverType     `mapstructure:"webserver"`
	SystemPackages map[string]string `mapstructure:"system_packages"`
	ConfigFile     string            `mapstructure:"config_file"`
	Healthcheck    bool              `mapstructure:"healthcheck"`
	Assets         []AssetToCopy     `mapstructure:"assets"`
	Locks          DefinitionLocks   `mapstructure:"-"`
}

func (def Definition) Validate() error {
	if def.Type.IsEmpty() {
		return xerrors.New("webserver build manifest has no type nor base_image parameters.")
	}

	return nil
}

func (def Definition) RawConfig() map[string]interface{} {
	return map[string]interface{}{
		"webserver":       def.Type,
		"system_packages": def.SystemPackages,
		"config_file":     def.ConfigFile,
		"healthcheck":     def.Healthcheck,
		"assets":          def.Assets,
	}
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
		return "nginx:latest"
	}

	panic(fmt.Sprintf("Webserver type %q is not supported.", string(t)))
}

type AssetToCopy struct {
	From string `mapstructure:"from"`
	To   string `mapstructure:"to"`
}
