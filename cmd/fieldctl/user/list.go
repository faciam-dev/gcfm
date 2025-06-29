package usercmd

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	dbcmd "github.com/faciam-dev/gcfm/cmd/fieldctl/db"
)

type listUser struct {
	ID       uint64 `json:"id"`
	Username string `json:"username"`
	Role     string `json:"role"`
}

// NewListCmd creates the user list subcommand.
func NewListCmd() *cobra.Command {
	var flags dbcmd.DBFlags
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List users",
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags.DSN == "" {
				return fmt.Errorf("--db is required")
			}
			if flags.Driver == "" {
				d, err := dbcmd.DetectDriver(flags.DSN)
				if err != nil {
					return err
				}
				flags.Driver = d
			}
			db, err := sql.Open(flags.Driver, flags.DSN)
			if err != nil {
				return err
			}
			defer db.Close()
			var q string
			switch flags.Driver {
			case "postgres":
				q = `SELECT id, username, role FROM users WHERE COALESCE(is_deleted,false)=false`
			default:
				q = `SELECT id, username, role FROM users WHERE COALESCE(is_deleted,0)=0`
			}
			rows, err := db.QueryContext(context.Background(), q)
			if err != nil {
				return err
			}
			defer rows.Close()
			var us []listUser
			for rows.Next() {
				var u listUser
				if err := rows.Scan(&u.ID, &u.Username, &u.Role); err != nil {
					return err
				}
				us = append(us, u)
			}
			b, _ := json.MarshalIndent(us, "", "  ")
			cmd.OutOrStdout().Write(b)
			if len(us) > 0 {
				cmd.OutOrStdout().Write([]byte("\n"))
			}
			return nil
		},
	}
	flags.AddFlags(cmd)
	cmd.MarkFlagRequired("db")
	return cmd
}
