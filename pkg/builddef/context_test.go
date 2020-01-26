package builddef_test

import (
	"testing"

	"github.com/NiR-/zbuild/pkg/builddef"
	"github.com/go-test/deep"
)

func TestNewContext(t *testing.T) {
	testcases := map[string]struct {
		source      string
		expected    *builddef.Context
		expectedErr error
	}{
		"git URI": {
			source: "git://github.com/some/repo",
			expected: &builddef.Context{
				Type:   builddef.ContextTypeGit,
				Source: "git://github.com/some/repo",
				GitContext: builddef.GitContext{
					Reference: "",
					Path:      "",
				},
			},
		},
		"git URI with subdir": {
			source: "git://github.com/some/repo#:sub/dir",
			expected: &builddef.Context{
				Type:   builddef.ContextTypeGit,
				Source: "git://github.com/some/repo",
				GitContext: builddef.GitContext{
					Reference: "",
					Path:      "sub/dir",
				},
			},
		},
		"git URI with ref": {
			source: "git://github.com/some/repo#someref",
			expected: &builddef.Context{
				Type:   builddef.ContextTypeGit,
				Source: "git://github.com/some/repo",
				GitContext: builddef.GitContext{
					Reference: "someref",
					Path:      "",
				},
			},
		},
		"git URI with subdir and ref": {
			source: "git://github.com/some/repo#someref:sub/dir",
			expected: &builddef.Context{
				Type:   builddef.ContextTypeGit,
				Source: "git://github.com/some/repo",
				GitContext: builddef.GitContext{
					Reference: "someref",
					Path:      "sub/dir",
				},
			},
		},
	}

	for tcname := range testcases {
		tc := testcases[tcname]

		t.Run(tcname, func(t *testing.T) {
			context, err := builddef.NewContext(tc.source, "")

			if tc.expectedErr != nil {
				if err == nil || err.Error() != tc.expectedErr.Error() {
					t.Fatalf("Expected error: %+v\nGot: %+v", tc.expectedErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Unexpected error: %+v", err)
			}

			if diff := deep.Equal(context, tc.expected); diff != nil {
				t.Fatal(diff)
			}
		})
	}
}
