package main

import "github.com/faciam-dev/gcfm/internal/customfield/pluginloader"

func resolvePluginDir(dir string) string {
	if dir == "" {
		return pluginloader.DefaultDir()
	}
	return dir
}
