package dbcmd

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/spf13/cobra"
	"golang.org/x/crypto/bcrypt"

	"github.com/faciam-dev/gcfm/internal/customfield/migrator"
	"github.com/faciam-dev/gcfm/sdk"
)

// NewMigrateCmd creates the db migrate subcommand.
func NewMigrateCmd() *cobra.Command {
	var flags DBFlags
	var to string
	var seed bool
	var verbose bool
	var tablePrefix string

	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Run DB migrations",
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags.DSN == "" {
				return fmt.Errorf("--db is required")
			}
			if flags.Driver == "" {
				d, err := DetectDriver(flags.DSN)
				if err != nil {
					return err
				}
				flags.Driver = d
			}
			target := 0
			if to != "" && to != "latest" {
				v, err := strconv.Atoi(to)
				if err != nil {
					return fmt.Errorf("invalid --to: %w", err)
				}
				target = v
			}
			if tablePrefix == "" {
				tablePrefix = os.Getenv("CF_TABLE_PREFIX")
			}
			svc := sdk.New(sdk.ServiceConfig{})
			ctx := context.Background()
			migrator.Verbose = verbose
			migrator.LogWriter = cmd.OutOrStdout()
			if err := svc.MigrateRegistry(ctx, sdk.DBConfig{Driver: flags.Driver, DSN: flags.DSN, Schema: flags.Schema, TablePrefix: tablePrefix}, target); err != nil {
				return err
			}
			if seed {
				if err := seedAdmin(ctx, flags, cmd.OutOrStdout()); err != nil {
					return err
				}
			}
			return nil
		},
	}
	flags.AddFlags(cmd)
	cmd.Flags().StringVar(&to, "to", "latest", "target version (number or latest)")
	cmd.Flags().BoolVar(&seed, "seed", false, "seed admin user")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "print SQL statements")
	cmd.Flags().StringVar(&tablePrefix, "table-prefix", "", "custom field table prefix")
	cmd.MarkFlagRequired("db")
	return cmd
}

func seedAdmin(ctx context.Context, f DBFlags, out io.Writer) error {
	db, err := sql.Open(f.Driver, f.DSN)
	if err != nil {
		return err
	}
	defer db.Close()
	var count int
	row := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM gcfm_users WHERE username='admin'`)
	if err := row.Scan(&count); err != nil {
		return err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte("admin123"), 12)
	if err != nil {
		return err
	}
	var q string
	switch f.Driver {
	case "postgres":
		if count > 0 {
			q = `UPDATE gcfm_users SET password_hash=$1 WHERE username='admin'`
		} else {
			q = `INSERT INTO gcfm_users (username,password_hash,role) VALUES ('admin',$1,'admin')`
		}
	default:
		if count > 0 {
			q = `UPDATE gcfm_users SET password_hash=? WHERE username='admin'`
		} else {
			q = `INSERT INTO gcfm_users (username,password_hash,role) VALUES ('admin',?,'admin')`
		}
	}
	if _, err := db.ExecContext(ctx, q, string(hash)); err != nil {
		return err
	}
	if count > 0 {
		fmt.Fprintln(out, "updated admin password: admin / admin123")
	} else {
		fmt.Fprintln(out, "created admin user: admin / admin123")
	}
	return nil
}
