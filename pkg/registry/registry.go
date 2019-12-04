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

// KindHandler represents a series of methods used to build and update locks for a given kind of builddef.
type KindHandler interface {
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

// KindRegistry associates kinds with their respective handler.
type KindRegistry struct {
	kinds map[string]KindHandler
}

// NewKindRegistry creates an empty KindRegistry.
func NewKindRegistry() *KindRegistry {
	return &KindRegistry{
		kinds: map[string]KindHandler{},
	}
}

// Register adds a kind handler to the registry.
func (reg *KindRegistry) Register(name string, handler KindHandler) {
	reg.kinds[name] = handler
}

// FindHandler checks if there's a known handler for the given
// kind. It returns the builder if one is found and
// ErrUnknownDefKind otherwise.
func (reg *KindRegistry) FindHandler(defKind string) (KindHandler, error) {
	builder, ok := reg.kinds[defKind]
	if !ok {
		// @TODO: put the kind in the error message for better UX
		return nil, ErrUnknownDefKind
	}

	return builder, nil
}

// ErrUnknownDefKind is returned when the decoded service has an unknown
// kind.
var ErrUnknownDefKind = xerrors.New("unknown kind")
