package usercmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/faciam-dev/goquent/orm"

	dbcmd "github.com/faciam-dev/gcfm/cmd/fieldctl/db"
	"github.com/faciam-dev/gcfm/internal/config"
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
			db, err := orm.OpenWithDriver(flags.Driver, flags.DSN)
			if err != nil {
				return err
			}
			defer db.Close()

			var us []listUser
			prefix := flags.TablePrefix
			if prefix == "" {
				prefix = "gcfm_"
			}
			cfg := config.Config{TablePrefix: prefix}
			err = db.Table(cfg.T("users")).
				Select("id", "username", "role").
				WhereRaw("COALESCE(is_deleted,false) = :f", map[string]any{
					"f": false,
				}).
				Get(&us)
			if err != nil {
				return err
			}
			b, _ := json.MarshalIndent(us, "", "  ")
			if _, err := cmd.OutOrStdout().Write(b); err != nil {
				return err
			}
			if len(us) > 0 {
				if _, err := cmd.OutOrStdout().Write([]byte("\n")); err != nil {
					return err
				}
			}
			return nil
		},
	}
	flags.AddFlags(cmd)
	cobra.CheckErr(cmd.MarkFlagRequired("db"))
	return cmd
}
