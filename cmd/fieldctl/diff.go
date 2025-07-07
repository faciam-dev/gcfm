package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/faciam-dev/gcfm/internal/customfield/registry"
	"github.com/faciam-dev/gcfm/internal/customfield/registry/codec"
	"github.com/faciam-dev/gcfm/sdk"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var exitFunc = os.Exit

func newDiffCmd() *cobra.Command {
	var (
		dbDSN      string
		schema     string
		file       string
		format     string
		fail       bool
		driverFlag string
		ignore     []string
		skipRes    bool
		prefix     string
		fallback   bool
	)
	cmd := &cobra.Command{
		Use:   "diff",
		Short: "Show schema drift between YAML and database",
		RunE: func(cmd *cobra.Command, args []string) error {
			if file == "" {
				return errors.New("--file is required")
			}
			if format != "text" && format != "markdown" {
				return errors.New("--format must be text or markdown")
			}
			ctx := context.Background()
			exported := false
			data, err := os.ReadFile(file)
			if err != nil {
				if os.IsNotExist(err) && fallback {
					svc := sdk.New(sdk.ServiceConfig{})
					data, err = svc.Export(ctx, sdk.DBConfig{Driver: driverFlag, DSN: dbDSN, Schema: schema, TablePrefix: prefix})
					if err != nil {
						return err
					}
					if err := os.WriteFile(file, data, 0644); err != nil {
						return err
					}
					exported = true
				} else {
					return err
				}
			}
			if skipRes {
				patterns, _ := loadReservedPatterns()
				ignore = append(ignore, patterns...)
			}
			yamlMetas, err := codec.DecodeYAML(data)
			if err != nil {
				return err
			}
			svc := sdk.New(sdk.ServiceConfig{})
			dbMetas, err := svc.Scan(ctx, sdk.DBConfig{Driver: driverFlag, DSN: dbDSN, Schema: schema, TablePrefix: prefix})
			if err != nil {
				return err
			}
			changes := registry.Diff(dbMetas, yamlMetas)
			if len(ignore) > 0 {
				regs := make([]*regexp.Regexp, 0, len(ignore))
				for _, p := range ignore {
					r, err := regexp.Compile(p)
					if err != nil {
						return fmt.Errorf("invalid ignore regex %s: %w", p, err)
					}
					regs = append(regs, r)
				}
				filtered := changes[:0]
				for _, c := range changes {
					var tbl string
					if c.New != nil {
						tbl = c.New.TableName
					} else if c.Old != nil {
						tbl = c.Old.TableName
					}
					match := false
					for _, r := range regs {
						if r.MatchString(tbl) {
							match = true
							break
						}
					}
					if match {
						continue
					}
					filtered = append(filtered, c)
				}
				changes = filtered
			}
			var drift bool
			for _, c := range changes {
				if c.Type != registry.ChangeUnchanged {
					drift = true
					break
				}
			}
			if !drift {
				fmt.Fprintln(cmd.OutOrStdout(), "✅ No schema drift detected.")
				if exported {
					exitFunc(3)
				}
				return nil
			}

			var b bytes.Buffer
			if format == "markdown" {
				b.WriteString("```diff\n")
				writeDiff(&b, changes, false)
				b.WriteString("```")
				b.WriteString("\n")
			} else {
				writeDiff(&b, changes, true)
			}
			cmd.Print(b.String())
			if fail {
				exitFunc(2)
			} else if exported {
				exitFunc(3)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbDSN, "db", "", "database DSN")
	cmd.Flags().StringVar(&schema, "schema", "public", "database schema")
	cmd.Flags().StringVar(&file, "file", "registry.yaml", "registry file")
	cmd.Flags().StringVar(&format, "format", "text", "output format (text|markdown)")
	cmd.Flags().BoolVar(&fail, "fail-on-change", false, "exit 2 if drift detected")
	cmd.Flags().StringVar(&driverFlag, "driver", "", "database driver (mysql|postgres|mongo|sqlmock)")
	cmd.Flags().StringSliceVar(&ignore, "ignore-regex", nil, "regex patterns of tables to ignore")
	cmd.Flags().StringVar(&prefix, "table-prefix", os.Getenv("CF_TABLE_PREFIX"), "table name prefix")
	cmd.Flags().BoolVar(&fallback, "fallback-export", false, "export registry if file missing")
	cmd.Flags().BoolVar(&skipRes, "skip-reserved", true, "exclude reserved tables")
	cmd.MarkFlagRequired("db")
	cmd.MarkFlagRequired("driver")
	return cmd
}

func writeDiff(buf *bytes.Buffer, changes []registry.Change, color bool) {
	const (
		green  = "\x1b[32m"
		red    = "\x1b[31m"
		yellow = "\x1b[33m"
		reset  = "\x1b[0m"
	)
	for _, c := range changes {
		switch c.Type {
		case registry.ChangeAdded:
			line := fmt.Sprintf("+ %s.%s (%s)", c.New.TableName, c.New.ColumnName, c.New.DataType)
			if color {
				fmt.Fprintf(buf, "%s%s%s\n", green, line, reset)
			} else {
				buf.WriteString(line + "\n")
			}
		case registry.ChangeDeleted:
			line := fmt.Sprintf("- %s.%s (%s)", c.Old.TableName, c.Old.ColumnName, c.Old.DataType)
			if color {
				fmt.Fprintf(buf, "%s%s%s\n", red, line, reset)
			} else {
				buf.WriteString(line + "\n")
			}
		case registry.ChangeUpdated:
			detail := updatedDetail(c.Old, c.New)
			line := fmt.Sprintf("± %s.%s %s", c.New.TableName, c.New.ColumnName, detail)
			if color {
				fmt.Fprintf(buf, "%s%s%s\n", yellow, line, reset)
			} else {
				buf.WriteString(line + "\n")
			}
		}
	}
}

func updatedDetail(old, new *registry.FieldMeta) string {
	var parts []string
	if old.DataType != new.DataType {
		parts = append(parts, fmt.Sprintf("type: %s → %s", old.DataType, new.DataType))
	}
	if old.Validator != new.Validator {
		o := old.Validator
		if o == "" {
			o = "none"
		}
		n := new.Validator
		if n == "" {
			n = "none"
		}
		parts = append(parts, fmt.Sprintf("validator: %s → %s", o, n))
	}
	if old.Nullable != new.Nullable {
		parts = append(parts, fmt.Sprintf("nullable: %v → %v", old.Nullable, new.Nullable))
	}
	if old.Unique != new.Unique {
		parts = append(parts, fmt.Sprintf("unique: %v → %v", old.Unique, new.Unique))
	}
	if !equalStringPtr(old.Default, new.Default) || old.HasDefault != new.HasDefault {
		oldDef := "none"
		if old.HasDefault {
			if old.Default != nil {
				oldDef = *old.Default
			} else {
				oldDef = ""
			}
		}
		newDef := "none"
		if new.HasDefault {
			if new.Default != nil {
				newDef = *new.Default
			} else {
				newDef = ""
			}
		}
		parts = append(parts, fmt.Sprintf("default: %s → %s", oldDef, newDef))
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, ", ")
}

func equalStringPtr(a, b *string) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func loadReservedPatterns() ([]string, error) {
	_, f, _, _ := runtime.Caller(0)
	base := filepath.Join(filepath.Dir(f), "..", "..")
	data, err := os.ReadFile(filepath.Join(base, "configs", "default.yaml"))
	if err != nil {
		return nil, err
	}
	var cfg struct {
		Reserved []string `yaml:"reserved_tables"`
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	if env := os.Getenv("CF_RESERVED_TABLES"); env != "" {
		var out []string
		for _, t := range strings.Split(env, ",") {
			t = strings.TrimSpace(t)
			if t != "" {
				out = append(out, t)
			}
		}
		return out, nil
	}
	return cfg.Reserved, nil
}
