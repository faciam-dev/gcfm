package main

import (
	"os"
	"strings"

	"github.com/faciam-dev/gcfm/internal/customfield/pluginloader"
)

func resolvePluginDir(dir string) string {
	if dir == "" {
		return pluginloader.DefaultDir()
	}
	return dir
}

func isTrustedModule(module string) bool {
	allowed := os.Getenv("FIELDCTL_TRUSTED_MODULE_PREFIXES")
	if allowed == "" {
		allowed = "github.com/faciam-dev/"
	}
	for _, p := range strings.Split(allowed, ",") {
		p = strings.TrimSpace(p)
		if p != "" && strings.HasPrefix(module, p) {
			return true
		}
	}
	return false
}
