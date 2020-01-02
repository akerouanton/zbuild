package llbutils

import (
	"encoding/base64"
	"encoding/json"
	"sort"

	"github.com/gogo/protobuf/proto"
	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/solver/pb"
	"github.com/opencontainers/go-digest"
	"golang.org/x/xerrors"
)

type llbOp struct {
	RawOp      string
	Op         *pb.Op
	Digest     digest.Digest
	OpMetadata pb.OpMetadata
}

func (op *llbOp) UnmarshalJSON(raw []byte) error {
	partial := struct {
		RawOp      string
		Digest     digest.Digest
		OpMetadata pb.OpMetadata
	}{}
	if err := json.Unmarshal(raw, &partial); err != nil {
		return err
	}

	rawOp, err := base64.StdEncoding.DecodeString(partial.RawOp)
	if err != nil {
		return err
	}

	var opOp pb.Op
	if err := proto.Unmarshal(rawOp, &opOp); err != nil {
		return err
	}

	*op = llbOp{
		Op:         &opOp,
		Digest:     partial.Digest,
		OpMetadata: partial.OpMetadata,
	}
	return nil
}

func StateToJSON(state llb.State) ([]byte, error) {
	var out []byte

	def, err := state.Marshal(llb.LinuxAmd64)
	if err != nil {
		return out, err
	}

	ops := make([]llbOp, 0, len(def.Def))
	for _, dt := range def.Def {
		var op pb.Op
		if err := (&op).Unmarshal(dt); err != nil {
			return out, xerrors.Errorf("failed to parse op: %w", err)
		}

		dgst := digest.FromBytes(dt)
		ent := llbOp{
			Op:         &op,
			Digest:     dgst,
			OpMetadata: def.Metadata[dgst],
		}

		rawOp, err := op.Marshal()
		if err != nil {
			return out, xerrors.Errorf("could not marshal Op into binary format: %w", err)
		}

		ent.RawOp = base64.StdEncoding.EncodeToString(rawOp)

		ops = append(ops, ent)
	}

	ops = sortOps(ops)
	out, err = json.MarshalIndent(ops, "", "  ")
	if err != nil {
		return out, xerrors.Errorf("could not encode Op into JSON: %w", err)
	}

	return out, nil
}

// sortOps takes care of sorting a list of llbOps by their digest, in asc
// order. This is used to ensure that identical lists won't change when
// testdata are regenerated (eg. llb graph dumps).
func sortOps(ops []llbOp) []llbOp {
	keys := make([]string, 0, len(ops))
	digests := make(map[string]llbOp)
	for _, op := range ops {
		digest := string(op.Digest)
		keys = append(keys, digest)
		digests[digest] = op
	}

	sort.Strings(keys)

	sorted := make([]llbOp, 0, len(ops))
	for _, key := range keys {
		sorted = append(sorted, digests[key])
	}

	return sorted
}

func JSONToDefinition(raw []byte) (*llb.Definition, error) {
	var ops []llbOp
	if err := json.Unmarshal(raw, &ops); err != nil {
		return nil, xerrors.Errorf("could not unmarshal LLB ops: %w", err)
	}

	def := &llb.Definition{
		Def:      make([][]byte, 0, len(ops)),
		Metadata: map[digest.Digest]pb.OpMetadata{},
	}
	for _, op := range ops {
		rawOp, err := op.Op.Marshal()
		if err != nil {
			return nil, xerrors.Errorf("could not marshal LLB Op: %w", err)
		}

		dgst := digest.FromBytes(rawOp)
		def.Def = append(def.Def, rawOp)
		def.Metadata[dgst] = op.OpMetadata
	}

	return def, nil
}
