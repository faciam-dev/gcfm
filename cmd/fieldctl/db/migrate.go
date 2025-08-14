package dbcmd

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"regexp"
	"strconv"

	"github.com/spf13/cobra"
	"golang.org/x/crypto/bcrypt"

	"github.com/faciam-dev/gcfm/sdk"
)

// NewMigrateCmd creates the db migrate subcommand.
func NewMigrateCmd() *cobra.Command {
	var flags DBFlags
	var to string
	var seed bool

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
			svc := sdk.New(sdk.ServiceConfig{})
			ctx := context.Background()
			if err := svc.MigrateRegistry(ctx, sdk.DBConfig{Driver: flags.Driver, DSN: flags.DSN, Schema: flags.Schema, TablePrefix: flags.TablePrefix}, target); err != nil {
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
	cmd.MarkFlagRequired("db")
	return cmd
}

func seedAdmin(ctx context.Context, f DBFlags, out io.Writer) error {
	db, err := sql.Open(f.Driver, f.DSN)
	if err != nil {
		return err
	}
	defer db.Close()
	prefix := f.TablePrefix
	users := prefix + "users"
	roles := prefix + "roles"
	userRoles := prefix + "user_roles"
	if err := validateIdentifier(users); err != nil {
		return err
	}
	if err := validateIdentifier(roles); err != nil {
		return err
	}
	if err := validateIdentifier(userRoles); err != nil {
		return err
	}
	var casbin string
	if f.Driver == "postgres" {
		casbin = "authz.casbin_rule"
	} else {
		casbin = "casbin_rule"
	}
	if err := validateIdentifier(casbin); err != nil {
		return err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte("admin123"), 12)
	if err != nil {
		return err
	}

	// ensure admin role exists
	switch f.Driver {
	case "postgres":
		if _, err := db.ExecContext(ctx, fmt.Sprintf("INSERT INTO %s (id,name) VALUES (1,'admin') ON CONFLICT (id) DO NOTHING", roles)); err != nil {
			return err
		}
	default:
		if _, err := db.ExecContext(ctx, fmt.Sprintf("INSERT INTO %s (id,name) VALUES (1,'admin') ON DUPLICATE KEY UPDATE name=VALUES(name)", roles)); err != nil {
			return err
		}
	}

	// upsert admin user
	switch f.Driver {
	case "postgres":
		if _, err := db.ExecContext(ctx, fmt.Sprintf("INSERT INTO %s (id,tenant_id,username,password_hash) VALUES (1,'default','admin',$1) ON CONFLICT (id) DO UPDATE SET password_hash=EXCLUDED.password_hash", users), string(hash)); err != nil {
			return err
		}
	default:
		if _, err := db.ExecContext(ctx, fmt.Sprintf("INSERT INTO %s (id,tenant_id,username,password_hash) VALUES (1,'default','admin',?) ON DUPLICATE KEY UPDATE password_hash=VALUES(password_hash)", users), string(hash)); err != nil {
			return err
		}
	}

	// link user to admin role
	switch f.Driver {
	case "postgres":
		if _, err := db.ExecContext(ctx, fmt.Sprintf("INSERT INTO %s (user_id,role_id) VALUES (1,1) ON CONFLICT DO NOTHING", userRoles)); err != nil {
			return err
		}
	default:
		if _, err := db.ExecContext(ctx, fmt.Sprintf("INSERT IGNORE INTO %s (user_id,role_id) VALUES (1,1)", userRoles)); err != nil {
			return err
		}
	}

	// seed casbin rules giving admin full access
	switch f.Driver {
	case "postgres":
		if _, err := db.ExecContext(ctx, "INSERT INTO "+casbin+"(ptype,v0,v1,v2,v3,v4,v5) VALUES ('p','admin','*','*','*','*','*'),('g','admin','admin','','','','') ON CONFLICT DO NOTHING"); err != nil {
			return err
		}
	default:
		if _, err := db.ExecContext(ctx, "INSERT IGNORE INTO "+casbin+"(ptype,v0,v1,v2,v3,v4,v5) VALUES ('p','admin','*','*','*','*','*'),('g','admin','admin','','','','')"); err != nil {
			return err
		}
	}

	fmt.Fprintln(out, "seeded admin user: admin / admin123")
	return nil
}

var identPattern = regexp.MustCompile(`^[A-Za-z0-9_.]+$`)

func validateIdentifier(name string) error {
	if !identPattern.MatchString(name) {
		return fmt.Errorf("invalid identifier: %s", name)
	}
	return nil
}
