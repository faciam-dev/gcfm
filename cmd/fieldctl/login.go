package main

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/faciam-dev/gcfm/pkg/config"
)

var (
	loginNonInteractive bool
)

func newLoginCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Save API endpoint and token into ~/.fieldctl/config.json",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			prof, _ := cmd.Root().Flags().GetString("profile")
			if prof == "" {
				prof = "default"
			}

			url, _ := cmd.Root().Flags().GetString("api-url")
			tok, _ := cmd.Root().Flags().GetString("token")
			if !loginNonInteractive {
				if url == "" {
					url = prompt("API URL", cfg.Profiles[prof].APIURL)
				}
				if tok == "" {
					tok = promptSecret("Token (Bearer)")
				}
			}
			if url == "" || tok == "" {
				return fmt.Errorf("api-url and token are required (provide flags or use interactive mode)")
			}

			ok, err := probe(url, tok)
			if err != nil || !ok {
				return fmt.Errorf("login failed: %w", err)
			}

			cp := cfg.Profiles[prof]
			cp.Name = prof
			cp.APIURL = url
			cp.Token = tok
			cfg.Profiles[prof] = cp
			cfg.Active = prof

			if err := config.Save(cfg); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Logged in. Active profile: %s\n", prof)
			return nil
		},
	}
	cmd.Flags().BoolVar(&loginNonInteractive, "non-interactive", false, "Fail instead of prompting")
	return cmd
}

func prompt(label, def string) string {
	fmt.Printf("%s [%s]: ", label, def)
	var s string
	if _, err := fmt.Scanln(&s); err != nil {
		return def
	}
	if strings.TrimSpace(s) == "" {
		return def
	}
	return s
}

func promptSecret(label string) string {
	fmt.Printf("%s: ", label)
	b, _ := term.ReadPassword(int(syscall.Stdin))
	fmt.Println()
	return strings.TrimSpace(string(b))
}

func probe(baseURL, token string) (bool, error) {
	url := strings.TrimSuffix(baseURL, "/") + "/admin/targets/version"
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	cli := &http.Client{Transport: tr, Timeout: 5 * time.Second}
	resp, err := cli.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK, nil
}
