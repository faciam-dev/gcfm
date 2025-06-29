package usercmd

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/spf13/cobra"

	dbcmd "github.com/faciam-dev/gcfm/cmd/fieldctl/db"
)

// NewDeleteCmd creates the user delete subcommand.
func NewDeleteCmd() *cobra.Command {
	var flags dbcmd.DBFlags
	var id uint64

	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete user logically",
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags.DSN == "" {
				return fmt.Errorf("--db is required")
			}
			if id == 0 {
				return fmt.Errorf("--id is required")
			}
			if flags.Driver == "" {
				flags.Driver = dbcmd.DetectDriver(flags.DSN)
			}
			db, err := sql.Open(flags.Driver, flags.DSN)
			if err != nil {
				return err
			}
			defer db.Close()
			var q string
			switch flags.Driver {
			case "postgres":
				q = `UPDATE users SET is_deleted=true WHERE id=$1`
			default:
				q = `UPDATE users SET is_deleted=1 WHERE id=?`
			}
			_, err = db.ExecContext(context.Background(), q, id)
			return err
		},
	}
	flags.AddFlags(cmd)
	cmd.Flags().Uint64Var(&id, "id", 0, "user id")
	cmd.MarkFlagRequired("db")
	cmd.MarkFlagRequired("id")
	return cmd
}
