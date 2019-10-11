package service

import (
	"context"

	"github.com/NiR-/webdf/pkg/image"
	"github.com/NiR-/webdf/pkg/llbutils"
	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/frontend/gateway/client"
	dpkg "github.com/snyh/go-dpkg-parser"
	"golang.org/x/xerrors"
)

// ErrUnknownServiceType is returned when the decoded service has an unknown
// type.
var ErrUnknownServiceType = xerrors.New("unknown service type")

// Service represents a service as declared in webdf config file.
type Service struct {
	Name      string                 `yaml:"name"`
	Type      string                 `yaml:"type"`
	RawConfig map[string]interface{} `yaml:",inline"`
	RawLocks  map[string]interface{} `yaml:"-"`
}

// BaseConfig exposes fields shared by all/most specific config structs.
type BaseConfig struct {
	ExternalFiles  []llbutils.ExternalFile
	SystemPackages map[string]string `mapstructure:"system_packages"`
}

// Locks define a common interface implemented by all service-specific Locks.
// It's unique method returns raw locks config stored in the lock file.
type Locks interface {
	Raw() map[string]interface{}
}

// BaseLocks exposes fields shared by all/most service locks.
type BaseLocks struct {
	SystemPackages map[string]string `mapstructure:"system_packages"`
}

// NewService creates a new service with the given name, type and raw config.
// This is mostly useful when services have to be created programatically, like
// during tests.
func NewService(name, serviceType string, rawConfig map[string]interface{}) *Service {
	return &Service{
		Name:      name,
		Type:      serviceType,
		RawConfig: rawConfig,
	}
}

// TypeHandler represents a series of methods used to build and update locks for a given type of service.
type TypeHandler interface {
	Build(ctx context.Context, c client.Client, opts BuildOpts) (llb.State, *image.Image, error)
	UpdateLocks(svc *Service, repo *dpkg.Repository) error
}

// BuildOpts represents the parameters passed to service builders.
type BuildOpts struct {
	Service   *Service
	SessionID string
	Stage     string
}

// TypeRegistry associates service types with their respective service type handler.
type TypeRegistry struct {
	types map[string]TypeHandler
}

// NewTypeRegistry creates an empty TypeRegistry.
func NewTypeRegistry() *TypeRegistry {
	return &TypeRegistry{
		types: map[string]TypeHandler{},
	}
}

// Register adds a type handler to the registry.
func (reg *TypeRegistry) Register(name string, handler TypeHandler) {
	reg.types[name] = handler
}

// FindTypeHandler checks if there's a known service type handler for the given
// service type. It returns the builder if one is found and
// ErrUnknwnServiceType otherwise.
func (reg *TypeRegistry) FindTypeHandler(svcType string) (TypeHandler, error) {
	builder, ok := reg.types[svcType]
	if !ok {
		return nil, ErrUnknownServiceType
	}

	return builder, nil
}

func ResolvePackageVersions(
	pkgs map[string]string,
	repo *dpkg.Repository,
	suites [][]string,
	arch string,
) (map[string]string, error) {
	versions := make(map[string]string, len(pkgs))
	for _, suite := range suites {
		err := repo.AddSuite(suite[0], suite[1], "")
		if err != nil {
			return versions, xerrors.Errorf("could not add suite %s/%s: %v", suite[0], suite[1], err)
		}
	}

	ar, err := repo.Archive(arch)
	if err != nil {
		return versions, err
	}

	for pkg := range pkgs {
		bp, err := ar.FindBinary(pkg)
		if err != nil {
			return versions, err
		}

		versions[pkg] = bp.Version
	}

	return versions, nil
}
