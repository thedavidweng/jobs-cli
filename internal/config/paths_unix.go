//go:build !windows

package config

import (
	"os"
	"runtime"
)

var defaultDir = func() string {
	if v := os.Getenv("JOBS_CONFIG_DIR"); v != "" {
		return v
	}
	home, _ := os.UserHomeDir()
	return defaultDirFor(runtime.GOOS, home, os.Getenv("XDG_CONFIG_HOME"))
}()

func appDataDir() string {
	return os.Getenv("APPDATA")
}
