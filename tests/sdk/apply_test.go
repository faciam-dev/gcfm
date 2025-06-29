package sdk_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/faciam-dev/gcfm/internal/customfield/registry"
	"github.com/faciam-dev/gcfm/sdk"
)

func TestCalculateDiff(t *testing.T) {
	changes := []registry.Change{
		{Type: registry.ChangeAdded},
		{Type: registry.ChangeDeleted},
		{Type: registry.ChangeUpdated},
		{Type: registry.ChangeUpdated},
	}
	got := sdk.CalculateDiff(changes)
	want := sdk.DiffReport{Added: 1, Deleted: 1, Updated: 2}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("diff mismatch (-want +got):\n%s", diff)
	}
}
