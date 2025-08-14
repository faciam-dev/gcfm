package cli_test

import (
	"reflect"
	"testing"

	dbcmd "github.com/faciam-dev/gcfm/cmd/fieldctl/db"
	"github.com/spf13/cobra"
)

func TestDBFlagsParse(t *testing.T) {
	t.Setenv("CF_TABLE_PREFIX", "")
	var f dbcmd.DBFlags
	cmd := &cobra.Command{Run: func(cmd *cobra.Command, args []string) {}}
	f.AddFlags(cmd)
	args := []string{"--db", "dsn", "--schema", "s", "--driver", "postgres"}
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	want := dbcmd.DBFlags{DSN: "dsn", Schema: "s", Driver: "postgres", TablePrefix: "gcfm_"}
	if !reflect.DeepEqual(f, want) {
		t.Fatalf("mismatch: %#v != %#v", f, want)
	}
}

func TestDetectDriver(t *testing.T) {
	drv, err := dbcmd.DetectDriver("postgres://u:p@localhost/db")
	if err != nil {
		t.Fatalf("detect: %v", err)
	}
	if drv != "postgres" {
		t.Fatalf("want postgres got %s", drv)
	}
	if _, err := dbcmd.DetectDriver("://"); err == nil {
		t.Fatalf("expected error for bad dsn")
	}
}
