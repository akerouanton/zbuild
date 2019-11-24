package llbtest

import (
	"encoding/json"
	"sort"
	"testing"

	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/solver/pb"
	"github.com/opencontainers/go-digest"
)

type llbOp struct {
	Op         pb.Op
	Digest     digest.Digest
	OpMetadata pb.OpMetadata
}

func StateToJSON(t *testing.T, state llb.State) string {
	def, err := state.Marshal(llb.LinuxAmd64)
	if err != nil {
		t.Fatal(err)
	}

	ops := make([]llbOp, 0, len(def.Def))

	for _, dt := range def.Def {
		var op pb.Op
		if err := (&op).Unmarshal(dt); err != nil {
			t.Fatalf("Failed to parse op: %v", err)
		}

		dgst := digest.FromBytes(dt)
		ent := llbOp{Op: op, Digest: dgst, OpMetadata: def.Metadata[dgst]}
		ops = append(ops, ent)
	}

	ops = sortOps(ops)
	out, err := json.MarshalIndent(ops, "", "  ")
	if err != nil {
		t.Fatalf("Could not encode Op into JSON: %v", err)
	}

	return string(out)
}

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
