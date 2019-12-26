package llbtest

import (
	"testing"

	"github.com/NiR-/zbuild/pkg/llbutils"
	"github.com/moby/buildkit/client/llb"
)

func StateToJSON(t *testing.T, state llb.State) string {
	out, err := llbutils.StateToJSON(state)
	if err != nil {
		t.Fatal(err)
	}

	return string(out)
}
