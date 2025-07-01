package usercmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"golang.org/x/crypto/bcrypt"

	"github.com/faciam-dev/goquent/orm"

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
			db, err := orm.OpenWithDriver(flags.Driver, flags.DSN)
			if err != nil {
				return err
			}
			defer db.Close()

			hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
			if err != nil {
				return err
			}
			_, err = db.Table("users").
				Insert(map[string]any{"username": username, "password_hash": string(hash), "role": role})
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
