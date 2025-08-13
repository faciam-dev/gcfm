package usercmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/faciam-dev/goquent/orm"

	dbcmd "github.com/faciam-dev/gcfm/cmd/fieldctl/db"
	"github.com/faciam-dev/gcfm/internal/config"
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

			prefix := flags.TablePrefix
			if prefix == "" {
				prefix = "gcfm_"
			}
			cfg := config.Config{TablePrefix: prefix}
			_, err = db.Table(cfg.T("users")).
				Where("id", id).
				Update(map[string]any{"is_deleted": true})
			return err
		},
	}
	flags.AddFlags(cmd)
	cmd.Flags().Uint64Var(&id, "id", 0, "user id")
	cmd.MarkFlagRequired("db")
	cmd.MarkFlagRequired("id")
	return cmd
}
