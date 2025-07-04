package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/faciam-dev/gcfm/internal/customfield/registry"
	"github.com/faciam-dev/gcfm/internal/customfield/registry/codec"
)

var errDiffDetected = errors.New("schema diff detected")

func newDiffCmd() *cobra.Command {
	var (
		dbDSN  string
		schema string
		file   string
		fail   bool
		format string
	)
	cmd := &cobra.Command{
		Use:   "diff",
		Short: "Compare registry YAML with database",
		RunE: func(cmd *cobra.Command, args []string) error {
			if file == "" {
				return errors.New("--file is required")
			}
			if format != "color" && format != "markdown" {
				return fmt.Errorf("invalid --format %s", format)
			}
			data, err := os.ReadFile(file)
			if err != nil {
				return err
			}
			desired, err := codec.DecodeYAML(data)
			if err != nil {
				return err
			}
			drv := detectDriver(dbDSN)
			db, err := sql.Open(drv, dbDSN)
			if err != nil {
				return err
			}
			defer db.Close()
			ctx := context.Background()
			current, err := registry.LoadSQL(ctx, db, registry.DBConfig{Driver: drv, Schema: schema})
			if err != nil {
				return err
			}
			changes := registry.Diff(current, desired)
			if len(changes) > 0 {
				printChanges(cmd.OutOrStdout(), changes, format)
				if fail {
					return errDiffDetected
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbDSN, "db", "", "database DSN")
	cmd.Flags().StringVar(&schema, "schema", "", "database schema")
	cmd.Flags().StringVar(&file, "file", "registry.yaml", "input file")
	cmd.Flags().BoolVar(&fail, "fail-on-change", false, "exit with code 2 when diff found")
	cmd.Flags().StringVar(&format, "format", "color", "output format (color|markdown)")
	cmd.MarkFlagRequired("db")
	cmd.MarkFlagRequired("schema")
	return cmd
}

func printChanges(w io.Writer, cs []registry.Change, format string) {
	for _, c := range cs {
		var line string
		switch c.Type {
		case registry.ChangeAdded:
			line = fmt.Sprintf("+ %s.%s", c.New.TableName, c.New.ColumnName)
			writeLine(w, line, "\x1b[32m", format)
		case registry.ChangeDeleted:
			line = fmt.Sprintf("- %s.%s", c.Old.TableName, c.Old.ColumnName)
			writeLine(w, line, "\x1b[31m", format)
		case registry.ChangeUpdated:
			line = fmt.Sprintf("± %s.%s", c.New.TableName, c.New.ColumnName)
			writeLine(w, line, "\x1b[33m", format)
		}
	}
}

func writeLine(w io.Writer, line, color, format string) {
	switch format {
	case "markdown":
		fmt.Fprintf(w, "- `%s`\n", line)
	default:
		fmt.Fprintf(w, "%s%s\x1b[0m\n", color, line)
	}
}
