package usercmd

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/spf13/cobra"
	"golang.org/x/crypto/bcrypt"

	dbcmd "github.com/faciam-dev/gcfm/cmd/fieldctl/db"
)

// NewCreateCmd creates the user create subcommand.
func NewCreateCmd() *cobra.Command {
	var flags dbcmd.DBFlags
	var username, password, role string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create user",
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags.DSN == "" {
				return fmt.Errorf("--db is required")
			}
			if username == "" || password == "" || role == "" {
				return fmt.Errorf("--username, --password and --role are required")
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

			hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
			if err != nil {
				return err
			}
			var q string
			switch flags.Driver {
			case "postgres":
				q = `INSERT INTO users (username,password_hash,role) VALUES ($1,$2,$3)`
			default:
				q = `INSERT INTO users (username,password_hash,role) VALUES (?,?,?)`
			}
			_, err = db.ExecContext(context.Background(), q, username, string(hash), role)
			return err
		},
	}
	flags.AddFlags(cmd)
	cmd.Flags().StringVar(&username, "username", "", "username")
	cmd.Flags().StringVar(&password, "password", "", "password")
	cmd.Flags().StringVar(&role, "role", "", "role")
	cmd.MarkFlagRequired("db")
	cmd.MarkFlagRequired("username")
	cmd.MarkFlagRequired("password")
	cmd.MarkFlagRequired("role")
	return cmd
}
