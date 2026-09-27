package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	ProviderLinkedIn = "linkedin"
)

type LinkedInProfile struct {
	SessionFile string `yaml:"session_file"`
}

type Profile struct {
	Timeout  time.Duration   `yaml:"timeout"`
	ReadOnly bool            `yaml:"read_only"`
	Sources  []string        `yaml:"sources"`
	Country  string          `yaml:"country"`
	Locale   string          `yaml:"locale"`
	LinkedIn LinkedInProfile `yaml:"linkedin"`
}

type Config struct {
	DefaultProfile string              `yaml:"default_profile"`
	Profiles       map[string]*Profile `yaml:"profiles"`

	ProfileName string   `yaml:"-"`
	Active      *Profile `yaml:"-"`
	Path        string   `yaml:"-"`

	envCountry string
	envLocale  string
}

// MarketSettings are the configured Indeed market values per layer. They stay
// separate because search ranks the environment above the profile and reports
// which layer chose the market.
type MarketSettings struct {
	EnvCountry     string
	EnvLocale      string
	ProfileCountry string
	ProfileLocale  string
}

func DefaultSources() []string {
	return []string{"indeed", "linkedin"}
}

func defaults() *Config {
	return &Config{
		DefaultProfile: "default",
		Profiles: map[string]*Profile{
			"default": {Timeout: 30 * time.Second, Sources: DefaultSources()},
		},
	}
}

func Load(path string) (*Config, error) {
	cfg := defaults()
	if path == "" {
		path = DefaultConfigPath()
	}
	cfg.Path = path
	if err := applyFileFromPath(cfg, path); err != nil {
		return cfg, err
	}
	cfg.resolve()
	return cfg, nil
}

func applyFileFromPath(cfg *Config, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read config %s: %w", path, err)
	}
	raw := map[string]any{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("parse config %s: %w", path, err)
	}
	applyFile(cfg, raw)
	return nil
}

func applyFile(cfg *Config, raw map[string]any) {
	if v, ok := raw["default_profile"].(string); ok && v != "" {
		cfg.DefaultProfile = v
	}
	profiles, ok := raw["profiles"].(map[string]any)
	if !ok {
		return
	}
	for name, p := range profiles {
		fields, ok := p.(map[string]any)
		if !ok {
			continue
		}
		prof := cfg.ensureProfile(name)
		if v, ok := fields["timeout"]; ok {
			if d, dok := coerceDuration(v); dok {
				prof.Timeout = d
			}
		}
		if v, ok := fields["read_only"].(bool); ok {
			prof.ReadOnly = v
		}
		if v, ok := fields["sources"].([]any); ok {
			prof.Sources = toStrings(v)
		}
		if v, ok := fields["country"].(string); ok {
			prof.Country = strings.TrimSpace(v)
		}
		if v, ok := fields["locale"].(string); ok {
			prof.Locale = strings.TrimSpace(v)
		}
		if v, ok := fields["linkedin"].(map[string]any); ok {
			if v, ok := v["session_file"].(string); ok {
				prof.LinkedIn.SessionFile = v
			}
		}
	}
}

func (c *Config) ensureProfile(name string) *Profile {
	if c.Profiles == nil {
		c.Profiles = map[string]*Profile{}
	}
	p, ok := c.Profiles[name]
	if !ok {
		p = &Profile{Timeout: 30 * time.Second, Sources: DefaultSources()}
		c.Profiles[name] = p
	}
	if len(p.Sources) == 0 {
		p.Sources = DefaultSources()
	}
	if p.Timeout == 0 {
		p.Timeout = 30 * time.Second
	}
	return p
}

func (c *Config) resolve() {
	name := c.DefaultProfile
	if v := os.Getenv("JOBS_PROFILE"); v != "" {
		name = v
	}
	if name == "" {
		name = "default"
	}
	c.ProfileName = name
	prof := c.ensureProfile(name)
	if v := os.Getenv("JOBS_TIMEOUT"); v != "" {
		if d, ok := coerceDuration(v); ok {
			prof.Timeout = d
		}
	}
	if v := os.Getenv("JOBS_READ_ONLY"); v != "" {
		prof.ReadOnly = ParseBool(v)
	}
	if v := os.Getenv("JOBS_SOURCES"); v != "" {
		if names := splitList(v); len(names) > 0 {
			prof.Sources = names
		}
	}
	c.envCountry = strings.TrimSpace(os.Getenv("JOBS_COUNTRY"))
	c.envLocale = strings.TrimSpace(os.Getenv("JOBS_LOCALE"))
	c.Active = prof
}

func (c *Config) Sources() []string {
	if c.Active == nil || len(c.Active.Sources) == 0 {
		return DefaultSources()
	}
	return c.Active.Sources
}

func (c *Config) Market() MarketSettings {
	settings := MarketSettings{EnvCountry: c.envCountry, EnvLocale: c.envLocale}
	if c.Active != nil {
		settings.ProfileCountry = c.Active.Country
		settings.ProfileLocale = c.Active.Locale
	}
	return settings
}

func (c *Config) Dir() string {
	if c.Path == "" {
		return DefaultDir()
	}
	return filepath.Dir(c.Path)
}

func (c *Config) SessionStore() *SessionStore {
	store := NewSessionStore(c.Dir()).WithProfile(c.ProfileName)
	if c.Active != nil && c.Active.LinkedIn.SessionFile != "" {
		store.SetOverride(c.ProfileName, c.Active.LinkedIn.SessionFile)
	}
	return store
}

func coerceDuration(v any) (time.Duration, bool) {
	switch t := v.(type) {
	case string:
		if d, err := time.ParseDuration(t); err == nil {
			return d, true
		}
	case int:
		return time.Duration(t) * time.Second, true
	case int64:
		return time.Duration(t) * time.Second, true
	case float64:
		return time.Duration(int64(t)) * time.Second, true
	}
	return 0, false
}

func toStrings(values []any) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		if s, ok := v.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func splitList(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func ParseBool(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
