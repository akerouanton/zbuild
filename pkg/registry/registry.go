package registry

import (
	"context"

	"github.com/NiR-/zbuild/pkg/builddef"
	"github.com/NiR-/zbuild/pkg/image"
	"github.com/NiR-/zbuild/pkg/pkgsolver"
	"github.com/NiR-/zbuild/pkg/statesolver"
	"github.com/moby/buildkit/client/llb"
	"golang.org/x/xerrors"
)

// KindHandler represents a series of methods used to build and update locks for a given kind of builddef.
type KindHandler interface {
	// WithSolver sets the state solver that should be used when building or
	// update the locks. It serves as a generic way to read files or resolve
	// image references, whether the KindHandler is used inside a Buildkit
	// client or as a CLI tool.
	WithSolver(statesolver.StateSolver)
	// Build is the method called by the builder package when buildkit daemon
	// whenever a new build with zbuild syntax provider starts. It returns a LLB
	// DAG representing the build steps and the metadata of the final image, or
	// an error if something goes wrong during the build.
	Build(context.Context, builddef.BuildOpts) (llb.State, *image.Image, error)
	UpdateLocks(context.Context, pkgsolver.PackageSolversMap, builddef.UpdateLocksOpts) (builddef.Locks, error)
	// DebugConfig loads and parses its kind definition based on parameters in
	// the BuildOpts, like Build() method does. It returns the end struct used
	// to build a given stage, after all merge and inference operations
	// happened. This is used by zbuild CLI tool to show to its users the
	// complete config struct used to build a given stage.
	DebugConfig(builddef.BuildOpts) (interface{}, error)
}

// KindRegistry associates kinds with their respective handler.
type KindRegistry struct {
	kinds            map[string]KindHandler
	withWebserverDef map[string]bool
}

// NewKindRegistry creates an empty KindRegistry.
func NewKindRegistry() *KindRegistry {
	return &KindRegistry{
		kinds:            map[string]KindHandler{},
		withWebserverDef: map[string]bool{},
	}
}

// Register adds a kind handler to the registry. The last parameter indicates
// whether this kind of definition embeds webserver definitions.
func (reg *KindRegistry) Register(
	name string,
	handler KindHandler,
	withWebserverDef bool,
) {
	reg.kinds[name] = handler
	reg.withWebserverDef[name] = withWebserverDef
}

// FindHandler checks if there's a known handler for the given kind. It returns
// the builder if one is found and ErrUnknownDefKind otherwise.
func (reg *KindRegistry) FindHandler(defkind string) (KindHandler, error) {
	builder, ok := reg.kinds[defkind]
	if !ok {
		return nil, xerrors.Errorf("kind %q is not supported: %w", defkind, ErrUnknownDefKind)
	}

	return builder, nil
}

func (reg *KindRegistry) EmbedWebserverDef(defkind string) bool {
	embedding, ok := reg.withWebserverDef[defkind]
	if !ok {
		return false
	}
	return embedding
}

// ErrUnknownDefKind is returned when the decoded service has an unknown
// kind.
var ErrUnknownDefKind = xerrors.New("unknown kind")

var Registry = NewKindRegistry()

func Register(name string, handler KindHandler, embedWebserverDef bool) {
	Registry.Register(name, handler, embedWebserverDef)
}

func FindHandler(defkind string) (KindHandler, error) {
	return Registry.FindHandler(defkind)
}
