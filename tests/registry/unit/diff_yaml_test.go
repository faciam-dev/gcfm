package unit_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/faciam-dev/gcfm/pkg/snapshot"
	sdk "github.com/faciam-dev/gcfm/sdk"
)

func TestDiffYaml(t *testing.T) {
	a := []byte("version: 0.3\nfields:\n  - table: posts\n    column: name\n    type: text\n")
	b := []byte("version: 0.3\nfields:\n  - table: posts\n    column: name\n    type: varchar\n  - table: posts\n    column: age\n    type: int\n")

	changes, err := snapshot.DiffYaml(a, b)
	if err != nil {
		t.Fatalf("diffyaml: %v", err)
	}
	got := sdk.CalculateDiff(changes)
	want := sdk.DiffReport{Added: 1, Updated: 1}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("unexpected diff (-want +got):\n%s", diff)
	}
}
