package main

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/spf13/cobra"

	"github.com/faciam-dev/gcfm/internal/customfield/snapshot"
)

func newSnapshotCmd() *cobra.Command {
	var (
		dest   string
		dbDSN  string
		schema string
	)
	cmd := &cobra.Command{
		Use:   "snapshot",
		Short: "Export registry snapshot",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dest == "" {
				return errors.New("--dest is required")
			}
			driver := detectDriver(dbDSN)
			db, err := sql.Open(driver, dbDSN)
			if err != nil {
				return err
			}
			defer db.Close()
			var d snapshot.Dest
			if strings.HasPrefix(dest, "s3://") {
				p := strings.TrimPrefix(dest, "s3://")
				parts := strings.SplitN(p, "/", 2)
				bucket := parts[0]
				prefix := ""
				if len(parts) > 1 {
					prefix = parts[1]
				}
				s3d, err := snapshot.NewS3(context.Background(), bucket, prefix)
				if err != nil {
					return err
				}
				d = s3d
			} else {
				d = snapshot.LocalDir{Path: dest}
			}
			return snapshot.Export(context.Background(), db, schema, d)
		},
	}
	cmd.Flags().StringVar(&dbDSN, "db", "", "database DSN")
	cmd.Flags().StringVar(&schema, "schema", "", "database schema")
	cmd.Flags().StringVar(&dest, "dest", "", "destination path or s3://bucket/prefix")
	cmd.MarkFlagRequired("db")
	cmd.MarkFlagRequired("schema")
	cmd.MarkFlagRequired("dest")
	return cmd
}
