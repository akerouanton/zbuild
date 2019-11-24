package registry

import (
	"context"

	"github.com/NiR-/webdf/pkg/builddef"
	"github.com/NiR-/webdf/pkg/image"
	"github.com/NiR-/webdf/pkg/pkgsolver"
	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/frontend/gateway/client"
	"golang.org/x/xerrors"
)

// TypeHandler represents a series of methods used to build and update locks for a given type of builddef.
type TypeHandler interface {
	// Build is the method called by the builder package when buildkit daemon
	// whenever a new build with webdf syntax provider starts. It returns a LLB
	// DAG representing the build steps and the metadata of the final image, or
	// an error if something goes wrong during the build.
	Build(ctx context.Context, c client.Client, buildOpts builddef.BuildOpts) (llb.State, *image.Image, error)
	// DebugLLB returns a LLB DAG like a call to BUild method does, but unlike
	// this other method, DebugLLB is never called during a buildkit session,
	// so there's no buildkit client available.
	DebugLLB(buildOpts builddef.BuildOpts) (llb.State, error)
	UpdateLocks(genericDef *builddef.BuildDef, pkgSolver pkgsolver.PackageSolver) (builddef.Locks, error)
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
// ErrUnknownDefType otherwise.
func (reg *TypeRegistry) FindTypeHandler(defType string) (TypeHandler, error) {
	builder, ok := reg.types[defType]
	if !ok {
		// @TODO: put the type in the error message for better UX
		return nil, ErrUnknownDefType
	}

	return builder, nil
}

// ErrUnknownDefType is returned when the decoded service has an unknown
// type.
var ErrUnknownDefType = xerrors.New("unknown service type")
