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

	t.Run("profile country and locale", func(t *testing.T) {
		t.Setenv("JOBS_PROFILE", "ca")
		cfg, err := Load(path)
		if err != nil {
			t.Fatal(err)
		}
		country, locale := cfg.Market()
		if country != "CA" || locale != "en-CA" {
			t.Fatalf("market = %q/%q, want CA/en-CA", country, locale)
		}
	})

	t.Run("locale-only profile passes the raw locale through", func(t *testing.T) {
		t.Setenv("JOBS_PROFILE", "fr")
		cfg, err := Load(path)
		if err != nil {
			t.Fatal(err)
		}
		country, locale := cfg.Market()
		if country != "" || locale != "fr-CA" {
			t.Fatalf("market = %q/%q, want empty country with fr-CA locale (sources derive the market)", country, locale)
		}
	})

	t.Run("env overrides profile", func(t *testing.T) {
		t.Setenv("JOBS_PROFILE", "default")
		t.Setenv("JOBS_COUNTRY", "GB")
		t.Setenv("JOBS_LOCALE", "en-GB")
		cfg, err := Load(path)
		if err != nil {
			t.Fatal(err)
		}
		country, locale := cfg.Market()
		if country != "GB" || locale != "en-GB" {
			t.Fatalf("market = %q/%q, want GB/en-GB", country, locale)
		}
	})

	t.Run("no market configured", func(t *testing.T) {
		cfg := &Config{}
		if country, locale := cfg.Market(); country != "" || locale != "" {
			t.Fatalf("market = %q/%q, want empty defaults", country, locale)
		}
	})
}
