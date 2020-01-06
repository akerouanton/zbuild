package builddef_test

import (
	"testing"

	"github.com/NiR-/zbuild/pkg/builddef"
	"github.com/go-test/deep"
)

func TestVersionMap(t *testing.T) {
	testcases := map[string]struct {
		initial  *builddef.VersionMap
		modifier func(*builddef.VersionMap)
		expected *builddef.VersionMap
	}{
		"Add() adds an item to the version map": {
			initial: &builddef.VersionMap{},
			modifier: func(versions *builddef.VersionMap) {
				versions.Add("curl", "7.64.0-4")
			},
			expected: &builddef.VersionMap{
				"curl": "7.64.0-4",
			},
		},
		"Add() doesn't overwrite a previous value": {
			initial: &builddef.VersionMap{
				"curl": "*",
			},
			modifier: func(versions *builddef.VersionMap) {
				versions.Add("curl", "7.64.0-4")
			},
			expected: &builddef.VersionMap{
				"curl": "*",
			},
		},
		"Add() does nothing when the map is nil": {
			initial: nil,
			modifier: func(versions *builddef.VersionMap) {
				versions.Add("curl", "7.64.0-4")
			},
			expected: nil,
		},
		"Overwrite() replaces any previously defined value": {
			initial: &builddef.VersionMap{
				"curl": "*",
			},
			modifier: func(versions *builddef.VersionMap) {
				versions.Overwrite("curl", "7.64.0-4")
			},
			expected: &builddef.VersionMap{
				"curl": "7.64.0-4",
			},
		},
		"Overwrite() adds a value if the key doesn't exist yet": {
			initial: &builddef.VersionMap{},
			modifier: func(versions *builddef.VersionMap) {
				versions.Overwrite("curl", "7.64.0-4")
			},
			expected: &builddef.VersionMap{
				"curl": "7.64.0-4",
			},
		},
		"Overwrite() does nothing when the map is nil": {
			initial: nil,
			modifier: func(versions *builddef.VersionMap) {
				versions.Overwrite("curl", "7.64.0-4")
			},
			expected: nil,
		},
		"Remove() does nothing if the item doesn't exist": {
			initial: &builddef.VersionMap{},
			modifier: func(versions *builddef.VersionMap) {
				versions.Remove("curl")
			},
			expected: &builddef.VersionMap{},
		},
		"Remove() does remove an item when it exists": {
			initial: &builddef.VersionMap{
				"curl": "*",
			},
			modifier: func(versions *builddef.VersionMap) {
				versions.Remove("curl")
			},
			expected: &builddef.VersionMap{},
		},
		"Remove() does nothing when the map is nil": {
			initial: nil,
			modifier: func(versions *builddef.VersionMap) {
				versions.Remove("curl")
			},
			expected: nil,
		},
	}

	for tcname := range testcases {
		tc := testcases[tcname]
		t.Run(tcname, func(t *testing.T) {
			versions := tc.initial
			tc.modifier(versions)

			if diff := deep.Equal(versions, tc.expected); diff != nil {
				t.Fatal(diff)
			}
		})
	}
}
