package main

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/faciam-dev/gcfm/pkg/config"
)

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "config", Short: "Manage fieldctl configuration"}
	cmd.AddCommand(newConfigUseCmd())
	cmd.AddCommand(newConfigListCmd())
	cmd.AddCommand(newConfigGetCmd())
	return cmd
}

func newConfigUseCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "use <profile>",
		Short: "Set active profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			prof := args[0]
			if _, ok := cfg.Profiles[prof]; !ok {
				return fmt.Errorf("profile %q not found", prof)
			}
			cfg.Active = prof
			if err := config.Save(cfg); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Switched to profile %q\n", prof)
			return nil
		},
	}
}

func newConfigListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List profiles",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _ := config.Load()
			for name, p := range cfg.Profiles {
				mark := ""
				if name == cfg.Active {
					mark = "*"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s %s\t%s\n", mark, name, p.APIURL)
			}
			return nil
		},
	}
}

func newConfigGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get",
		Short: "Show active profile",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _ := config.Load()
			p := cfg.Profiles[cfg.Active]
			b, _ := json.MarshalIndent(struct {
				Active   string `json:"active"`
				APIURL   string `json:"apiUrl"`
				Insecure bool   `json:"insecure"`
				HasToken bool   `json:"hasToken"`
			}{cfg.Active, p.APIURL, p.Insecure, p.Token != ""}, "", "  ")
			fmt.Fprintln(cmd.OutOrStdout(), string(b))
			return nil
		},
	}
}
