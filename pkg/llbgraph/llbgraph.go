package llbgraph

import (
	"encoding/json"
	"fmt"
	"html"
	"sort"
	"strings"

	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/solver/pb"
	"github.com/opencontainers/go-digest"
	"golang.org/x/xerrors"
)

type GraphOpts struct {
	RawAttrs bool
}

func ToDotGraph(def *llb.Definition, opts GraphOpts) (*DotGraph, error) {
	g := newDotGraph(opts)
	for _, dt := range def.Def {
		var op pb.Op
		if err := (&op).Unmarshal(dt); err != nil {
			return nil, err
		}

		dgst := digest.FromBytes(dt)
		meta := def.Metadata[dgst]
		if err := g.addOp(op, meta, dgst); err != nil {
			return nil, err
		}
	}

	return g, nil
}

func newDotGraph(opts GraphOpts) *DotGraph {
	return &DotGraph{
		nodes: map[string]graphNode{},
		edges: []graphEdge{},
		opts:  opts,
	}
}

func (g *DotGraph) sortNodeIDs() []string {
	ids := make([]string, 0, len(g.nodes))
	for id := range g.nodes {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func (g *DotGraph) MarshalText() ([]byte, error) {
	lines := make([]string, 0, len(g.nodes)+len(g.edges))
	lines = append(lines, "digraph llbgraph {")

	nodeIDs := g.sortNodeIDs()
	for _, nodeID := range nodeIDs {
		node := g.nodes[nodeID]
		lines = append(lines, fmt.Sprintf(
			`"%s" [label=%s,shape="%s",style="%s",fillcolor="%s"]`,
			nodeID,
			cleanupLabel(node.label),
			node.nodeType.shape(),
			node.nodeType.style(),
			node.nodeType.fillcolor(),
		))
	}
	for _, edge := range g.edges {
		lines = append(lines, fmt.Sprintf(
			`"%s" -> "%s" [label=%s]`,
			edge.source,
			edge.dest,
			cleanupLabel(edge.label),
		))
	}

	lines = append(lines, "}")
	text := strings.Join(lines, "\n")
	return []byte(text), nil
}

func cleanupLabel(label string) string {
	if len(label) == 0 {
		return `""`
	}
	if label[0] != '<' {
		label = strings.Replace(label, "\"", "\\\"", -1)
		return `"` + label + `"`
	}
	return `<` + label + `>`
}

type DotGraph struct {
	nodes map[string]graphNode
	edges []graphEdge
	opts  GraphOpts
}

type graphNode struct {
	label    string
	nodeType nodeType
}

type graphEdge struct {
	label  string
	source string
	dest   string
}

func (g *DotGraph) addOp(
	op pb.Op,
	meta pb.OpMetadata,
	output digest.Digest,
) error {
	var err error
	switch op.Op.(type) {
	case *pb.Op_Source:
		opSource := op.Op.(*pb.Op_Source)
		err = g.addOpSource(opSource, meta, op.Inputs, output)
	case *pb.Op_Exec:
		opExec := op.Op.(*pb.Op_Exec)
		err = g.addOpExec(opExec, meta, op.Inputs, output)
	case *pb.Op_File:
		opFile := op.Op.(*pb.Op_File)
		err = g.addOpFile(opFile, meta, op.Inputs, output)
	case nil:
		g.addOpFinal(op.Inputs, output)
	default:
		err = xerrors.New("llb op type not supported")
	}

	return err
}

func getMetaDescription(meta pb.OpMetadata) string {
	descAttrs := meta.GetDescription()
	return descAttrs["llb.customname"]
}

func (g *DotGraph) addOpSource(
	op *pb.Op_Source,
	meta pb.OpMetadata,
	inputs []*pb.Input,
	output digest.Digest,
) error {
	sourceID := g.addNodeSource(op.Source.Identifier)
	lastID := sourceID

	attrs := op.Source.GetAttrs()
	if len(attrs) > 0 {
		label := formatOpSourceLabel(meta, attrs)
		opNodeID, err := g.addNodeOp(output.String(), "Op_Source", label, attrs)
		if err != nil {
			return xerrors.Errorf("could not marshal Op_Source Attrs: %w", err)
		}

		g.addEdge(sourceID, opNodeID, graphEdge{})
		lastID = opNodeID
	}

	outputID := g.addNodeLayer(output, false)
	g.addEdge(lastID, outputID, graphEdge{})

	return nil
}

func formatOpSourceLabel(meta pb.OpMetadata, attrs map[string]string) string {
	if label := getMetaDescription(meta); len(label) > 0 {
		return label
	}

	if v, ok := attrs["git.fullurl"]; ok {
		return "Clone " + v
	}

	return fmt.Sprintf("%+v", attrs)
}

func (g *DotGraph) addOpExec(
	op *pb.Op_Exec,
	meta pb.OpMetadata,
	inputs []*pb.Input,
	output digest.Digest,
) error {
	opID := output.String() + "_exec"
	description := getMetaDescription(meta)
	attrs := map[string]interface{}{
		"Exec": op.Exec.GetMeta(),
	}

	if len(op.Exec.GetMounts()) > 0 {
		attrs["Mounts"] = op.Exec.GetMounts()
	}

	opNodeID, err := g.addNodeOp(opID, "Op_Exec", description, attrs)
	if err != nil {
		return xerrors.Errorf("could not marshal Op_Exec meta args: %w", err)
	}

	for _, input := range inputs {
		inputID := g.addNodeLayer(input.Digest, false)
		g.addEdge(inputID, opNodeID, graphEdge{})
	}

	outputID := g.addNodeLayer(output, false)
	g.addEdge(opNodeID, outputID, graphEdge{})

	return nil
}

func (g *DotGraph) addOpFile(
	op *pb.Op_File,
	meta pb.OpMetadata,
	inputs []*pb.Input,
	output digest.Digest,
) error {
	outputID := g.addNodeLayer(output, false)
	lastID := outputID

	description := getMetaDescription(meta)
	actions := op.File.GetActions()

	for i := len(actions) - 1; i >= 0; i-- {
		action := actions[i]
		opID := fmt.Sprintf("%s_op%d", output.String(), i)

		actionDesc := description
		if len(description) == 0 {
			actionDesc = action.String()
		}

		opNodeID, err := g.addNodeOp(
			opID, "Op_File", actionDesc, action.Action)
		if err != nil {
			return xerrors.Errorf("could not marshal Op_File action: %w", err)
		}

		g.addEdge(opNodeID, lastID, graphEdge{})
		lastID = opNodeID
	}

	for _, input := range inputs {
		inputID := g.addNodeLayer(input.Digest, false)
		g.addEdge(inputID, lastID, graphEdge{})
	}

	return nil
}

func (g *DotGraph) addNodeLayer(dgst digest.Digest, final bool) string {
	id := g.nodeID(typeLayer, dgst.String())
	if g.hasNode(id) {
		return id
	}

	nodeType := typeLayer
	if final {
		nodeType = typeFinalLayer
	}

	g.nodes[id] = graphNode{
		label:    dgst.String(),
		nodeType: nodeType,
	}
	return id
}

func (g *DotGraph) addNodeSource(sourceID string) string {
	id := g.nodeID(typeSource, sourceID)
	if g.hasNode(id) {
		return id
	}

	label := fmt.Sprintf("<B>%s</B>", sourceID)
	g.nodes[id] = graphNode{
		label:    label,
		nodeType: typeSource,
	}
	return id
}

func (g *DotGraph) addNodeOp(
	opID,
	opType,
	label string,
	attrs interface{},
) (string, error) {
	nodeID := g.nodeID(typeOp, opID)
	if g.hasNode(nodeID) {
		return nodeID, nil
	}

	fullLabel := fmt.Sprintf(
		"<I>%s:</I><BR/><B>%s</B>",
		html.EscapeString(opType),
		html.EscapeString(label),
	)
	if g.opts.RawAttrs && attrs != nil {
		rawAttrs, err := json.MarshalIndent(attrs, "", "  ")
		if err != nil {
			return "", err
		}
		escaped := html.EscapeString(string(rawAttrs))
		escaped = strings.Replace(escaped, "\n", "<BR/>", -1)
		fullLabel += fmt.Sprintf(
			"<BR/>%s", escaped)
	}

	g.nodes[nodeID] = graphNode{
		label:    fullLabel,
		nodeType: typeOp,
	}
	return nodeID, nil
}

func (g *DotGraph) addOpFinal(inputs []*pb.Input, output digest.Digest) {
	outputID := g.addNodeLayer(output, true)

	for _, input := range inputs {
		inputID := g.addNodeLayer(input.Digest, false)
		g.addEdge(inputID, outputID, graphEdge{})
	}
}

func (g *DotGraph) hasNode(nodeID string) bool {
	_, ok := g.nodes[nodeID]
	return ok
}

func (g *DotGraph) nodeID(nodeType nodeType, name string) string {
	return fmt.Sprintf("%s_%s", string(nodeType), name)
}

func (g *DotGraph) addEdge(src, dst string, opts graphEdge) {
	opts.source = src
	opts.dest = dst
	g.edges = append(g.edges, opts)
}

type nodeType string

var (
	typeSource     = nodeType("source")
	typeLayer      = nodeType("layer")
	typeFinalLayer = nodeType("final_layer")
	typeOp         = nodeType("op")
)

func (t nodeType) shape() dotGraphShape {
	switch t {
	case typeSource:
		return shapeSource
	case typeLayer:
		return shapeLayer
	case typeFinalLayer:
		return shapeLayer
	case typeOp:
		return shapeOp
	}

	panic(fmt.Sprintf("node type %q not supported", t))
}

func (t nodeType) style() string {
	switch t {
	case typeFinalLayer:
		return "filled"
	default:
		return ""
	}
}

func (t nodeType) fillcolor() string {
	switch t {
	case typeFinalLayer:
		return "#bdbdbd"
	default:
		return ""
	}
}

type dotGraphShape string

var (
	shapeSource = dotGraphShape("doublecircle")
	shapeLayer  = dotGraphShape("box")
	shapeOp     = dotGraphShape("diamond")
)
