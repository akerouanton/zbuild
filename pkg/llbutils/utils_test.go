package llbutils_test

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/NiR-/webdf/pkg/llbtest"
	"github.com/NiR-/webdf/pkg/llbutils"
	"github.com/go-test/deep"
	"github.com/golang/mock/gomock"
	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/frontend/gateway/client"
)

func TestSolveState_succeeds_to_solve_a_source_and_get_a_single_ref_from_it(t *testing.T) {
	t.Parallel()

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	ref := llbtest.NewMockReference(mockCtrl)
	res := &client.Result{
		Refs: map[string]client.Reference{"linux/amd64": ref},
		Ref:  ref,
	}

	ctx := context.TODO()
	c := llbtest.NewMockClient(mockCtrl)
	c.EXPECT().Solve(ctx, gomock.Any()).Return(res, nil)

	outRes, outRef, err := llbutils.SolveState(ctx, c, llb.State{})
	if err != nil {
		t.Errorf("Expected no error but got one: %v\n", err)
		t.FailNow()
	}
	if diff := deep.Equal(ref, outRef); diff != nil {
		t.Fatal(diff)
	}
	if diff := deep.Equal(res, outRes); diff != nil {
		t.Fatal(diff)
	}
}

func TestSolveState_returns_an_error_when_solve_request_fail(t *testing.T) {
	t.Parallel()

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	ctx := context.TODO()
	c := llbtest.NewMockClient(mockCtrl)
	c.EXPECT().Solve(ctx, gomock.Any()).Return(nil, errors.New("some error"))

	_, _, err := llbutils.SolveState(ctx, c, llb.State{})
	if err == nil {
		t.Error("Error expected but none returned")
	}
}

func TestSolveState_returns_an_error_when_result_as_no_singe_ref(t *testing.T) {
	t.Parallel()

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	ref := llbtest.NewMockReference(mockCtrl)
	res := &client.Result{
		Refs: map[string]client.Reference{"linux/amd64": ref},
	}

	ctx := context.TODO()
	c := llbtest.NewMockClient(mockCtrl)
	c.EXPECT().Solve(ctx, gomock.Any()).Return(res, nil)

	_, _, err := llbutils.SolveState(ctx, c, llb.State{})
	if err == nil {
		t.Error("Error expected but none returned")
	}
}

func TestReadFile_successfully_returns_file_content(t *testing.T) {
	t.Parallel()

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	ctx := context.TODO()
	filepath := "some/file.yml"
	expected := []byte("some file content")

	ref := llbtest.NewMockReference(mockCtrl)
	ref.EXPECT().ReadFile(ctx, client.ReadRequest{
		Filename: filepath,
	}).Return(expected, nil)

	out, ok, err := llbutils.ReadFile(ctx, ref, filepath)
	if err != nil {
		t.Errorf("No error expected but got one: %v\n", err)
		t.FailNow()
	}
	if !ok {
		t.Errorf("File %q not found but should be.", filepath)
	}
	if diff := deep.Equal([]byte("some file content"), out); diff != nil {
		t.Error(diff)
	}
}

func TestReadFile_returns_no_error_when_file_is_not_found(t *testing.T) {
	t.Parallel()

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	ctx := context.TODO()
	filepath := "some/file.yml"

	ref := llbtest.NewMockReference(mockCtrl)
	ref.EXPECT().ReadFile(ctx, gomock.Any()).Return([]byte{}, os.ErrNotExist)

	_, ok, err := llbutils.ReadFile(ctx, ref, filepath)
	if err != nil {
		t.Errorf("No error expected but got one: %v\n", err)
		t.FailNow()
	}
	if ok {
		t.Errorf("File %q should not exist.", filepath)
	}
}
