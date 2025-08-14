package dbcmd

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/faciam-dev/gcfm/internal/monitordb"
	"github.com/faciam-dev/gcfm/pkg/crypto"
)

// NewAddMonCmd returns command to add monitored database.
func NewAddMonCmd() *cobra.Command {
	var f DBFlags
	var name, driver, dsn, tenant string
	cmd := &cobra.Command{Use: "add", Short: "Add monitored database", RunE: func(cmd *cobra.Command, args []string) error {
		if err := crypto.CheckEnv(); err != nil {
			return err
		}
		db, err := sql.Open(f.Driver, f.DSN)
		if err != nil {
			return err
		}
		defer db.Close()
		repo := &monitordb.Repo{DB: db, Driver: f.Driver, TablePrefix: f.TablePrefix}
		enc, err := crypto.Encrypt([]byte(dsn))
		if err != nil {
			return err
		}
		id, err := repo.Create(cmd.Context(), monitordb.Database{TenantID: tenant, Name: name, Driver: driver, DSNEnc: enc})
		if err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "added database %d\n", id)
		return nil
	}}
	f.AddFlags(cmd)
	cmd.Flags().StringVar(&name, "name", "", "database name")
	cmd.Flags().StringVar(&driver, "target-driver", "", "target driver")
	cmd.Flags().StringVar(&dsn, "dsn", "", "target DSN")
	cmd.Flags().StringVar(&tenant, "tenant", "default", "tenant id")
	cmd.MarkFlagRequired("name")
	cmd.MarkFlagRequired("driver")
	cmd.MarkFlagRequired("dsn")
	return cmd
}

// NewListMonCmd lists monitored databases.
func NewListMonCmd() *cobra.Command {
	var f DBFlags
	var tenant string
	cmd := &cobra.Command{Use: "ls", Short: "List monitored databases", RunE: func(cmd *cobra.Command, args []string) error {
		db, err := sql.Open(f.Driver, f.DSN)
		if err != nil {
			return err
		}
		defer db.Close()
		repo := &monitordb.Repo{DB: db, Driver: f.Driver, TablePrefix: f.TablePrefix}
		dbs, err := repo.List(cmd.Context(), tenant)
		if err != nil {
			return err
		}
		for _, d := range dbs {
			fmt.Fprintf(cmd.OutOrStdout(), "%d\t%s\t%s\n", d.ID, d.Name, d.Driver)
		}
		return nil
	}}
	f.AddFlags(cmd)
	cmd.Flags().StringVar(&tenant, "tenant", "default", "tenant id")
	return cmd
}

// NewRemoveMonCmd removes a monitored database.
func NewRemoveMonCmd() *cobra.Command {
	var f DBFlags
	var id int64
	var tenant string
	cmd := &cobra.Command{Use: "rm", Short: "Remove monitored database", RunE: func(cmd *cobra.Command, args []string) error {
		db, err := sql.Open(f.Driver, f.DSN)
		if err != nil {
			return err
		}
		defer db.Close()
		repo := &monitordb.Repo{DB: db, Driver: f.Driver, TablePrefix: f.TablePrefix}
		return repo.Delete(context.Background(), tenant, id)
	}}
	f.AddFlags(cmd)
	cmd.Flags().Int64Var(&id, "id", 0, "database id")
	cmd.Flags().StringVar(&tenant, "tenant", "default", "tenant id")
	cmd.MarkFlagRequired("id")
	return cmd
}
