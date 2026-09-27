package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMarketFromProfileFileAndEnv(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := `
default_profile: ca
profiles:
  default:
    country: US
  ca:
    country: CA
    locale: en-CA
  fr:
    locale: fr-CA
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	load := func(t *testing.T, profile string) MarketSettings {
		t.Helper()
		t.Setenv("JOBS_PROFILE", profile)
		cfg, err := Load(path)
		if err != nil {
			t.Fatal(err)
		}
		return cfg.Market()
	}

	t.Run("profile country and locale", func(t *testing.T) {
		t.Setenv("JOBS_COUNTRY", "")
		t.Setenv("JOBS_LOCALE", "")
		want := MarketSettings{ProfileCountry: "CA", ProfileLocale: "en-CA"}
		if got := load(t, "ca"); got != want {
			t.Fatalf("market = %+v, want %+v", got, want)
		}
	})

	t.Run("locale-only profile keeps the country unset", func(t *testing.T) {
		t.Setenv("JOBS_COUNTRY", "")
		t.Setenv("JOBS_LOCALE", "")
		want := MarketSettings{ProfileLocale: "fr-CA"}
		if got := load(t, "fr"); got != want {
			t.Fatalf("market = %+v, want %+v (a locale never implies a country)", got, want)
		}
	})

	t.Run("environment stays a separate layer", func(t *testing.T) {
		t.Setenv("JOBS_COUNTRY", " GB ")
		t.Setenv("JOBS_LOCALE", "en-GB")
		want := MarketSettings{EnvCountry: "GB", EnvLocale: "en-GB", ProfileCountry: "US"}
		if got := load(t, "default"); got != want {
			t.Fatalf("market = %+v, want %+v", got, want)
		}
	})

	t.Run("no market configured", func(t *testing.T) {
		cfg := &Config{}
		if got := cfg.Market(); got != (MarketSettings{}) {
			t.Fatalf("market = %+v, want no settings", got)
		}
	})
}
