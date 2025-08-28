package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

type Resolved struct {
	APIURL  string
	Token   string
	Profile string
}

func Resolve(cmd *cobra.Command) (Resolved, error) {
	flagURL, _ := cmd.Root().PersistentFlags().GetString("api-url")
	flagToken, _ := cmd.Root().PersistentFlags().GetString("token")

	envURL := os.Getenv("FIELDTOOL_API_URL")
	envToken := os.Getenv("FIELDTOOL_TOKEN")

	cfg, _ := Load()
	prof := cfg.Active
	if p, _ := cmd.Root().PersistentFlags().GetString("profile"); p != "" {
		prof = p
	}
	cp := cfg.Profiles[prof]

	url := firstNonEmpty(flagURL, envURL, cp.APIURL)
	tok := firstNonEmpty(flagToken, envToken, cp.Token)
	if url == "" {
		return Resolved{}, fmt.Errorf("API URL not set (flag/env/config)")
	}
	if tok == "" {
		return Resolved{}, fmt.Errorf("token not set (flag/env/config)")
	}

	return Resolved{
		APIURL:  url,
		Token:   tok,
		Profile: prof,
	}, nil
}

func firstNonEmpty(ss ...string) string {
	for _, s := range ss {
		if strings.TrimSpace(s) != "" {
			return s
		}
	}
	return ""
}
