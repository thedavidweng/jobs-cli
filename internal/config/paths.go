package config

import (
	"os"
	"path/filepath"
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

func DefaultDir() string {
	return defaultDir
}

func DefaultConfigPath() string {
	return filepath.Join(DefaultDir(), "config.yaml")
}

func defaultDirFor(goos, home, xdgConfigHome string) string {
	if goos != "linux" {
		appData := ""
		if goos == "windows" {
			appData = appDataDir()
		}
		if appData != "" {
			return filepath.Join(appData, "jobs-cli")
		}
		return filepath.Join(home, ".jobs-cli")
	}
	if xdgConfigHome != "" && filepath.IsAbs(xdgConfigHome) {
		return filepath.Join(xdgConfigHome, "jobs-cli")
	}
	return filepath.Join(home, ".config", "jobs-cli")
}
