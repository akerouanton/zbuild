package nodejs

import (
	"strings"

	"github.com/NiR-/zbuild/pkg/builddef"
	"github.com/NiR-/zbuild/pkg/defkinds/webserver"
	"github.com/NiR-/zbuild/pkg/llbutils"
	"github.com/mitchellh/mapstructure"
	"golang.org/x/xerrors"
	"gopkg.in/yaml.v2"
)

var defaultBaseImages = map[string]string{
	"10": "docker.io/library/node:10-buster-slim",
	"12": "docker.io/library/node:12-buster-slim",
	"13": "docker.io/library/node:13-buster-slim",
}
var supportedVersions = "10, 12, 13"

func (h *NodeJSHandler) loadDefs(
	buildOpts builddef.BuildOpts,
) (Definition, StageDefinition, error) {
	var def Definition
	var stageDef StageDefinition

	def, err := NewKind(buildOpts.Def)
	if err != nil {
		return def, stageDef, err
	}

	stageName := buildOpts.Stage
	if strings.HasPrefix(stageName, "webserver-") {
		stageName = strings.TrimPrefix(stageName, "webserver-")
	}

	stageDef, err = def.ResolveStageDefinition(stageName, true)
	if err != nil {
		return def, stageDef, xerrors.Errorf("could not resolve stage %q: %w", stageName, err)
	}

	return def, stageDef, nil
}

func defaultDefinition() Definition {
	devStageDevMode := true
	prodStageDevMode := false
	healthcheckEnabled := true

	return Definition{
		BaseStage: Stage{
			Healthcheck: &healthcheckEnabled,
		},
		Stages: DerivedStageSet{
			"dev": {
				DeriveFrom: "base",
				Dev:        &devStageDevMode,
			},
			"prod": {
				DeriveFrom: "base",
				Dev:        &prodStageDevMode,
			},
		},
	}
}

// @TODO: rename into NewDefinition
func NewKind(genericDef *builddef.BuildDef) (Definition, error) {
	var def Definition
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

	def = defaultDefinition().Merge(def)

	if err := yaml.Unmarshal(genericDef.RawLocks, &def.Locks); err != nil {
		err := xerrors.Errorf("could not decode lock manifest: %w", err)
		return def, err
	}

	if def.Webserver != nil {
		*def.Webserver = webserver.DefaultDefinition().Merge(*def.Webserver)
	}

	if def.Version != "" && def.BaseImage != "" {
		return def, xerrors.Errorf("you can't provide both version and base image parameters at the same time")
	}

	if def.BaseImage == "" {
		baseImage, ok := defaultBaseImages[def.Version]
		if !ok {
			return def, xerrors.Errorf("no default base image defined for NodeJS v%s, you have to define it by yourself in your zbuildfile or use one of the supported versions: %s", def.Version, supportedVersions)
		}
		def.BaseImage = baseImage
	}

	return def, nil
}

type Definition struct {
	BaseStage Stage `mapstructure:",squash"`

	// @TODO: check what happens when base isn't prefixed with docker.io/library/
	BaseImage string          `mapstructure:"base"`
	Version   string          `mapstructure:"version"`
	Stages    DerivedStageSet `mapstructure:"stages"`
	// @TODO: move to Stage?
	IsFrontend bool                  `mapstructure:"frontend"`
	Webserver  *webserver.Definition `mapstructure:"webserver"`

	Locks DefinitionLocks `mapstructure:"-"`
}

func (d Definition) Copy() Definition {
	new := Definition{
		BaseStage:  d.BaseStage.Copy(),
		BaseImage:  d.BaseImage,
		Version:    d.Version,
		Stages:     d.Stages.Copy(),
		IsFrontend: d.IsFrontend,
	}

	if d.Webserver != nil {
		webserver := *d.Webserver
		new.Webserver = &webserver
	}

	return new
}

func (base Definition) Merge(overriding Definition) Definition {
	new := base.Copy()

	new.BaseStage = new.BaseStage.Merge(overriding.BaseStage)
	new.Stages = new.Stages.Merge(overriding.Stages)
	new.BaseImage = overriding.BaseImage
	new.Version = overriding.Version
	new.IsFrontend = overriding.IsFrontend

	if overriding.Webserver != nil {
		webserver := overriding.Webserver.Copy()
		if new.Webserver != nil {
			webserver = new.Webserver.Merge(*overriding.Webserver)
		}
		new.Webserver = &webserver
	}

	return new
}

type Stage struct {
	ExternalFiles  []llbutils.ExternalFile `mapstructure:"external_files"`
	SystemPackages map[string]string       `mapstructure:"system_packages"`
	GlobalPackages map[string]string       `mapstructure:"global_packages"`
	BuildCommand   *string                 `mapstructure:"build_command"`
	Command        *[]string               `mapstructure:"command"`
	ConfigFiles    map[string]string       `mapstructure:"config_files"`
	// @TODO: rename into sourcecode and accept both dirs and files
	SourceDirs   []string `mapstructure:"source_dirs"`
	StatefulDirs []string `mapstructure:"stateful_dirs"`
	Healthcheck  *bool    `mapstructur:"healthcheck"`
}

func (s Stage) Copy() Stage {
	new := Stage{
		ExternalFiles:  make([]llbutils.ExternalFile, len(s.ExternalFiles)),
		SystemPackages: map[string]string{},
		GlobalPackages: map[string]string{},
		BuildCommand:   s.BuildCommand,
		Command:        s.Command,
		ConfigFiles:    map[string]string{},
		SourceDirs:     make([]string, len(s.SourceDirs)),
		StatefulDirs:   make([]string, len(s.StatefulDirs)),
		Healthcheck:    s.Healthcheck,
	}

	copy(new.ExternalFiles, s.ExternalFiles)
	copy(new.SourceDirs, s.SourceDirs)
	copy(new.StatefulDirs, s.StatefulDirs)

	for name, constraint := range s.SystemPackages {
		new.SystemPackages[name] = constraint
	}
	for name, constraint := range s.GlobalPackages {
		new.GlobalPackages[name] = constraint
	}
	for src, dst := range s.ConfigFiles {
		new.ConfigFiles[src] = dst
	}

	return new
}

func (s Stage) Merge(overriding Stage) Stage {
	new := s.Copy()
	new.ExternalFiles = append(new.ExternalFiles, overriding.ExternalFiles...)
	new.SourceDirs = append(new.SourceDirs, overriding.SourceDirs...)
	new.StatefulDirs = append(new.StatefulDirs, overriding.StatefulDirs...)

	for name, constraint := range overriding.SystemPackages {
		new.SystemPackages[name] = constraint
	}
	for name, constraint := range overriding.GlobalPackages {
		new.GlobalPackages[name] = constraint
	}
	for from, to := range overriding.ConfigFiles {
		new.ConfigFiles[from] = to
	}

	if overriding.BuildCommand != nil {
		buildCmd := *overriding.BuildCommand
		new.BuildCommand = &buildCmd
	}
	if overriding.Command != nil {
		cmd := *overriding.Command
		new.Command = &cmd
	}
	if overriding.Healthcheck != nil {
		healthcheck := *overriding.Healthcheck
		new.Healthcheck = &healthcheck
	}

	return new
}

type DerivedStage struct {
	Stage `mapstructure:",squash"`

	DeriveFrom string `mapstructure:"from"`
	Dev        *bool  `mapstructure:"dev"`
}

func (s DerivedStage) Copy() DerivedStage {
	new := DerivedStage{
		Stage:      s.Stage.Copy(),
		DeriveFrom: s.DeriveFrom,
	}

	if s.Dev != nil {
		devMode := *s.Dev
		new.Dev = &devMode
	}

	return new
}

func (s DerivedStage) Merge(overriding DerivedStage) DerivedStage {
	new := s.Copy()

	new.Stage = s.Stage.Merge(overriding.Stage)
	new.DeriveFrom = overriding.DeriveFrom

	if overriding.Dev != nil {
		devMode := *overriding.Dev
		new.Dev = &devMode
	}

	return new
}

type DerivedStageSet map[string]DerivedStage

func (set DerivedStageSet) Copy() DerivedStageSet {
	new := DerivedStageSet{}

	for name, stage := range set {
		new[name] = stage.Copy()
	}

	return new
}

func (base DerivedStageSet) Merge(overriding DerivedStageSet) DerivedStageSet {
	new := base.Copy()

	for name, stage := range overriding {
		if _, ok := new[name]; !ok {
			new[name] = stage
		} else {
			new[name] = new[name].Merge(stage)
		}
	}

	return new
}

type StageDefinition struct {
	Stage
	Name       string
	BaseImage  string
	Version    string
	Dev        *bool
	IsFrontend bool
	Webserver  *webserver.Definition
	Locks      StageLocks
}

func (def *Definition) ResolveStageDefinition(
	name string,
	withLocks bool,
) (StageDefinition, error) {
	var stageDef StageDefinition
	stages, err := def.resolveStageChain(name)
	if err != nil {
		return stageDef, err
	}

	stageDef = mergeStages(def, stages...)
	stageDef.Name = name

	if !withLocks {
		return stageDef, nil
	}

	locks, ok := def.Locks.Stages[name]
	if !ok {
		return stageDef, xerrors.Errorf(
			"no locks available for stage %q. Please update your lockfile", name)
	}

	stageDef.Locks = locks

	return stageDef, nil
}

func (def *Definition) resolveStageChain(name string) ([]DerivedStage, error) {
	stages := make([]DerivedStage, 0, len(def.Stages))
	resolvedStages := map[string]struct{}{}
	current := name

	for current != "" && current != "base" {
		if _, ok := resolvedStages[current]; ok {
			return stages, xerrors.Errorf(
				"there's a cyclic dependency between %q and itself", current)
		}

		stage, ok := def.Stages[current]
		if !ok {
			return stages, xerrors.Errorf("stage %q not found", current)
		}

		stages = append(stages, stage)
		resolvedStages[current] = struct{}{}
		current = stage.DeriveFrom
	}

	return stages, nil
}

func mergeStages(base *Definition, stages ...DerivedStage) StageDefinition {
	devMode := false
	stageDef := StageDefinition{
		BaseImage:  base.BaseImage,
		Version:    base.Version,
		Stage:      base.BaseStage.Copy(),
		IsFrontend: base.IsFrontend,
		Dev:        &devMode,
	}

	for i := len(stages) - 1; i >= 0; i-- {
		derived := stages[i]
		stageDef.Stage = stageDef.Stage.Merge(derived.Stage)

		if derived.Dev != nil {
			stageDef.Dev = derived.Dev
		}
	}

	if *stageDef.Dev || stageDef.IsFrontend {
		*stageDef.Healthcheck = false
	}

	return stageDef
}
