package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"

	"github.com/faciam-dev/gcfm/pkg/config"
)

// Target represents a target definition returned by the Admin API.
type Target struct {
	Key           string   `json:"key"`
	Driver        string   `json:"driver"`
	Dsn           string   `json:"dsn"`
	Schema        string   `json:"schema"`
	Labels        []string `json:"labels"`
	MaxOpenConns  int      `json:"maxOpenConns"`
	MaxIdleConns  int      `json:"maxIdleConns"`
	ConnMaxIdleMs int      `json:"connMaxIdleMs"`
	ConnMaxLifeMs int      `json:"connMaxLifeMs"`
	IsDefault     bool     `json:"isDefault"`
	UpdatedAt     string   `json:"updatedAt"`
}

// TargetInput is used to send target definitions to the Admin API.
type TargetInput struct {
	Key           string   `json:"key"`
	Driver        string   `json:"driver"`
	Dsn           string   `json:"dsn"`
	Schema        string   `json:"schema"`
	Labels        []string `json:"labels"`
	MaxOpenConns  int      `json:"maxOpenConns"`
	MaxIdleConns  int      `json:"maxIdleConns"`
	ConnMaxIdleMs int      `json:"connMaxIdleMs"`
	ConnMaxLifeMs int      `json:"connMaxLifeMs"`
	IsDefault     bool     `json:"isDefault"`
}

var (
	targetKey           string
	targetDriver        string
	targetDSN           string
	targetSchema        string
	targetLabels        []string
	targetMaxOpen       int
	targetMaxIdle       int
	targetConnMaxIdleMs int
	targetConnMaxLifeMs int
	targetIsDefault     bool
	targetIfMatch       string
)

// apiRequest performs an HTTP request against the Admin API using resolved configuration.
func apiRequest(method, path string, body any, ifMatch string) (*http.Response, error) {
	resolved, err := config.Resolve(rootCmd)
	if err != nil {
		return nil, err
	}
	url := strings.TrimSuffix(resolved.APIURL, "/") + path

	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, url, rdr)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+resolved.Token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if ifMatch != "" {
		req.Header.Set("If-Match", ifMatch)
	}
	client := http.DefaultClient
	if resolved.Insecure {
		// SECURITY WARNING:
		// Disabling TLS certificate verification (InsecureSkipVerify: true) is highly insecure.
		// This exposes the client to man-in-the-middle attacks and should only be used for testing or development.
		tr := &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
		client = &http.Client{Transport: tr}
	}
	return client.Do(req)
}

// printOutput prints data in either JSON or table format based on the --output flag.
func printOutput(v any) error {
	format, err := rootCmd.Flags().GetString("output")
	if err != nil {
		return err
	}
	switch format {
	case "json":
		b, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(b))
	default:
		switch x := v.(type) {
		case []Target:
			tw := tablewriter.NewWriter(os.Stdout)
			tw.SetHeader([]string{"Key", "Driver", "DSN", "Labels", "Default", "Updated"})
			for _, t := range x {
				tw.Append([]string{t.Key, t.Driver, t.Dsn, strings.Join(t.Labels, ","), fmt.Sprint(t.IsDefault), t.UpdatedAt})
			}
			tw.Render()
		case Target:
			fmt.Printf("%s (%s) default=%v\n", x.Key, x.Driver, x.IsDefault)
			fmt.Println("Labels:", strings.Join(x.Labels, ","))
			fmt.Println("DSN:", x.Dsn)
		default:
			b, err := json.MarshalIndent(v, "", "  ")
			if err != nil {
				return err
			}
			fmt.Println(string(b))
		}
	}
	return nil
}

var targetsCmd = &cobra.Command{
	Use:   "targets",
	Short: "Manage target DB definitions in MetaDB",
}

var listTargetsCmd = &cobra.Command{
	Use:   "list",
	Short: "List all targets",
	RunE: func(cmd *cobra.Command, args []string) error {
		resp, err := apiRequest("GET", "/admin/targets", nil, "")
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("error: %s", resp.Status)
		}
		var out struct {
			Items []Target `json:"items"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			return err
		}
		return printOutput(out.Items)
	},
}

var getTargetCmd = &cobra.Command{
	Use:   "get [key]",
	Short: "Get one target",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := fmt.Sprintf("/admin/targets/%s", url.PathEscape(args[0]))
		resp, err := apiRequest("GET", path, nil, "")
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("error: %s", resp.Status)
		}
		var t Target
		if err := json.NewDecoder(resp.Body).Decode(&t); err != nil {
			return err
		}
		return printOutput(t)
	},
}

var createTargetCmd = &cobra.Command{
	Use:   "create",
	Short: "Create new target",
	RunE: func(cmd *cobra.Command, args []string) error {
		in := TargetInput{
			Key:           targetKey,
			Driver:        targetDriver,
			Dsn:           targetDSN,
			Schema:        targetSchema,
			Labels:        targetLabels,
			MaxOpenConns:  targetMaxOpen,
			MaxIdleConns:  targetMaxIdle,
			ConnMaxIdleMs: targetConnMaxIdleMs,
			ConnMaxLifeMs: targetConnMaxLifeMs,
			IsDefault:     targetIsDefault,
		}
		resp, err := apiRequest("POST", "/admin/targets", in, "")
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusCreated {
			return fmt.Errorf("error: %s", resp.Status)
		}
		var t Target
		if err := json.NewDecoder(resp.Body).Decode(&t); err != nil {
			return err
		}
		return printOutput(t)
	},
}

var updateTargetCmd = &cobra.Command{
	Use:   "update [key]",
	Short: "Update target",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := fmt.Sprintf("/admin/targets/%s", url.PathEscape(args[0]))
		in := TargetInput{
			Key:           targetKey,
			Driver:        targetDriver,
			Dsn:           targetDSN,
			Schema:        targetSchema,
			Labels:        targetLabels,
			MaxOpenConns:  targetMaxOpen,
			MaxIdleConns:  targetMaxIdle,
			ConnMaxIdleMs: targetConnMaxIdleMs,
			ConnMaxLifeMs: targetConnMaxLifeMs,
			IsDefault:     targetIsDefault,
		}
		resp, err := apiRequest("PUT", path, in, targetIfMatch)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("error: %s", resp.Status)
		}
		var t Target
		if err := json.NewDecoder(resp.Body).Decode(&t); err != nil {
			return err
		}
		return printOutput(t)
	},
}

var patchTargetCmd = &cobra.Command{
	Use:   "patch [key]",
	Short: "Patch target",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := fmt.Sprintf("/admin/targets/%s", url.PathEscape(args[0]))
		in := TargetInput{
			Key:           targetKey,
			Driver:        targetDriver,
			Dsn:           targetDSN,
			Schema:        targetSchema,
			Labels:        targetLabels,
			MaxOpenConns:  targetMaxOpen,
			MaxIdleConns:  targetMaxIdle,
			ConnMaxIdleMs: targetConnMaxIdleMs,
			ConnMaxLifeMs: targetConnMaxLifeMs,
			IsDefault:     targetIsDefault,
		}
		resp, err := apiRequest("PATCH", path, in, targetIfMatch)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("error: %s", resp.Status)
		}
		var t Target
		if err := json.NewDecoder(resp.Body).Decode(&t); err != nil {
			return err
		}
		return printOutput(t)
	},
}

var deleteTargetCmd = &cobra.Command{
	Use:   "delete [key]",
	Short: "Delete a target",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := fmt.Sprintf("/admin/targets/%s", url.PathEscape(args[0]))
		resp, err := apiRequest("DELETE", path, nil, targetIfMatch)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusNoContent {
			return fmt.Errorf("error: %s", resp.Status)
		}
		fmt.Println("Deleted", args[0])
		return nil
	},
}

var setDefaultTargetCmd = &cobra.Command{
	Use:   "set-default [key]",
	Short: "Set a target as default",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := fmt.Sprintf("/admin/targets/%s/default", url.PathEscape(args[0]))
		resp, err := apiRequest("POST", path, nil, targetIfMatch)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusNoContent {
			return fmt.Errorf("error: %s", resp.Status)
		}
		fmt.Println("Default set:", args[0])
		return nil
	},
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show current version & default target",
	RunE: func(cmd *cobra.Command, args []string) error {
		resp, err := apiRequest("GET", "/admin/targets/version", nil, "")
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("error: %s", resp.Status)
		}
		_, err = io.Copy(os.Stdout, resp.Body)
		return err
	},
}

var bumpVersionCmd = &cobra.Command{
	Use:   "bump-version",
	Short: "Force bump version",
	RunE: func(cmd *cobra.Command, args []string) error {
		resp, err := apiRequest("POST", "/admin/targets/version/bump", nil, "")
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("error: %s", resp.Status)
		}
		_, err = io.Copy(os.Stdout, resp.Body)
		return err
	},
}

func init() {
	rootCmd.AddCommand(targetsCmd)
	targetsCmd.AddCommand(
		listTargetsCmd,
		getTargetCmd,
		createTargetCmd,
		updateTargetCmd,
		patchTargetCmd,
		deleteTargetCmd,
		setDefaultTargetCmd,
		versionCmd,
		bumpVersionCmd,
	)

	cmdWithTargetFlags := []*cobra.Command{createTargetCmd, updateTargetCmd, patchTargetCmd}
	for _, c := range cmdWithTargetFlags {
		c.Flags().StringVar(&targetKey, "key", "", "Target key")
		c.Flags().StringVar(&targetDriver, "driver", "", "Database driver")
		c.Flags().StringVar(&targetDSN, "dsn", "", "Database DSN")
		c.Flags().StringVar(&targetSchema, "schema", "", "Schema name")
		c.Flags().StringSliceVar(&targetLabels, "label", nil, "Labels")
		c.Flags().IntVar(&targetMaxOpen, "max-open", 0, "Max open connections")
		c.Flags().IntVar(&targetMaxIdle, "max-idle", 0, "Max idle connections")
		c.Flags().IntVar(&targetConnMaxIdleMs, "conn-max-idle-ms", 0, "Connection max idle time in ms")
		c.Flags().IntVar(&targetConnMaxLifeMs, "conn-max-life-ms", 0, "Connection max lifetime in ms")
		c.Flags().BoolVar(&targetIsDefault, "default", false, "Set as default")
	}

	for _, c := range []*cobra.Command{updateTargetCmd, patchTargetCmd, deleteTargetCmd, setDefaultTargetCmd} {
		c.Flags().StringVar(&targetIfMatch, "if-match", "", "ETag value")
	}
}
