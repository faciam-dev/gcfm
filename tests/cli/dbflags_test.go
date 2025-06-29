package cli_test

import (
	"reflect"
	"testing"

	dbcmd "github.com/faciam-dev/gcfm/cmd/fieldctl/db"
	"github.com/spf13/cobra"
)

func TestDBFlagsParse(t *testing.T) {
	var f dbcmd.DBFlags
	cmd := &cobra.Command{Run: func(cmd *cobra.Command, args []string) {}}
	f.AddFlags(cmd)
	args := []string{"--db", "dsn", "--schema", "s", "--driver", "postgres"}
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	want := dbcmd.DBFlags{DSN: "dsn", Schema: "s", Driver: "postgres"}
	if !reflect.DeepEqual(f, want) {
		t.Fatalf("mismatch: %#v != %#v", f, want)
	}
}
