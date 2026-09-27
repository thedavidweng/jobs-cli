package config_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/thedavidweng/jobs-cli/internal/config"
)

func TestSessionFilePermissionsAndRoundTrip(t *testing.T) {
	dir := t.TempDir()
	store := config.NewSessionStore(dir)
	session := config.NewLinkedInSession("default", "chrome", map[string]string{
		config.CookieLiAt:       "li-at-value",
		config.CookieJSessionID: `"ajax:1234"`,
	})
	if err := store.Save(session); err != nil {
		t.Fatalf("save: %v", err)
	}

	sessionsDir := filepath.Join(dir, "sessions")
	dirInfo, err := os.Stat(sessionsDir)
	if err != nil {
		t.Fatalf("stat sessions dir: %v", err)
	}
	if perm := dirInfo.Mode().Perm(); runtime.GOOS != "windows" && perm != 0o700 {
		t.Fatalf("sessions dir mode = %04o, want 0700", perm)
	}
	fileInfo, err := os.Stat(filepath.Join(sessionsDir, "default.json"))
	if err != nil {
		t.Fatalf("stat session file: %v", err)
	}
	if perm := fileInfo.Mode().Perm(); runtime.GOOS != "windows" && perm != 0o600 {
		t.Fatalf("session file mode = %04o, want 0600", perm)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.Cookie(config.CookieLiAt) != "li-at-value" {
		t.Fatalf("li_at round trip = %q", loaded.Cookie(config.CookieLiAt))
	}
	if loaded.CSRFTokenValue() != "ajax:1234" {
		t.Fatalf("csrf token = %q, want unquoted JSESSIONID", loaded.CSRFTokenValue())
	}
	if !loaded.Complete() {
		t.Fatal("session should be complete")
	}
	if loaded.SchemaVersion != config.SessionSchemaVersion {
		t.Fatalf("schema_version = %q", loaded.SchemaVersion)
	}

	status := store.Status()
	if !status.Present || !status.Complete {
		t.Fatalf("status = %#v", status)
	}
	if len(status.CookieNames) != 2 {
		t.Fatalf("cookie names = %#v", status.CookieNames)
	}

	removed, err := store.Remove()
	if err != nil || !removed {
		t.Fatalf("remove = %v, %v", removed, err)
	}
	if store.Status().Present {
		t.Fatal("session reported present after removal")
	}
}

func TestIncompleteStoredSessionIsRejectedByLoad(t *testing.T) {
	store := config.NewSessionStore(t.TempDir())
	session := config.NewLinkedInSession("default", "chrome", map[string]string{
		config.CookieLiAt: "only-li-at",
	})
	if session.Complete() {
		t.Fatal("session without JSESSIONID must be incomplete")
	}
	if err := store.Save(session); err != nil {
		t.Fatalf("save: %v", err)
	}
	if _, err := store.Load(); err == nil || !strings.Contains(err.Error(), "JSESSIONID") {
		t.Fatalf("load = %v, want a validation error naming JSESSIONID", err)
	}
}

func TestSessionStatusDistinguishesAbsentInvalidUnreadableAndValid(t *testing.T) {
	store := config.NewSessionStore(t.TempDir())

	status := store.Status()
	if status.Present || status.Invalid || status.Complete {
		t.Fatalf("absent status = %#v", status)
	}

	if err := os.MkdirAll(store.Dir(), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(store.Path(), []byte("{not json"), 0o600); err != nil {
		t.Fatalf("write corrupt session: %v", err)
	}
	status = store.Status()
	if !status.Present || !status.Invalid || status.Complete || status.InvalidReason == "" {
		t.Fatalf("corrupt status = %#v, want present but invalid with a reason", status)
	}
	if !strings.Contains(status.InvalidReason, "not valid JSON") {
		t.Fatalf("invalid reason = %q, want a JSON parse failure", status.InvalidReason)
	}

	if runtime.GOOS != "windows" {
		if err := os.Chmod(store.Path(), 0o000); err != nil {
			t.Fatalf("chmod: %v", err)
		}
		t.Cleanup(func() {
			_ = os.Chmod(store.Path(), 0o600)
		})
		status = store.Status()
		if !status.Present || !status.Invalid || status.Complete {
			t.Fatalf("unreadable status = %#v, want present but invalid", status)
		}
		if !strings.Contains(status.InvalidReason, store.Path()) {
			t.Fatalf("invalid reason = %q, want the unreadable path", status.InvalidReason)
		}
		if err := os.Chmod(store.Path(), 0o600); err != nil {
			t.Fatalf("chmod back: %v", err)
		}
	}

	if err := os.Remove(store.Path()); err != nil {
		t.Fatalf("remove: %v", err)
	}
	session := config.NewLinkedInSession("default", "chrome", map[string]string{
		config.CookieLiAt:       "li-at-value",
		config.CookieJSessionID: `"ajax:1234"`,
	})
	if err := store.Save(session); err != nil {
		t.Fatalf("save: %v", err)
	}
	status = store.Status()
	if !status.Present || !status.Complete || status.Invalid {
		t.Fatalf("valid status = %#v", status)
	}

	if _, err := store.Load(); err != nil {
		t.Fatalf("load valid session: %v", err)
	}
}

func TestSessionStatusReportsUnstattableSessionAsInvalid(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix permission bits")
	}
	store := config.NewSessionStore(t.TempDir())
	if err := os.MkdirAll(store.Dir(), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(store.Path(), []byte("{}"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(store.Dir(), 0o700)
	})
	if err := os.Chmod(store.Dir(), 0o000); err != nil {
		t.Fatalf("chmod dir: %v", err)
	}
	status := store.Status()
	if !status.Present || !status.Invalid || status.Complete || status.InvalidReason == "" {
		t.Fatalf("unstattable status = %#v, want present but invalid with a reason", status)
	}
}

func TestSessionPathOverrideFromConfig(t *testing.T) {
	dir := t.TempDir()
	custom := filepath.Join(dir, "custom-session.json")
	body := "default_profile: work\nprofiles:\n  work:\n    linkedin:\n      session_file: " + custom + "\n"
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(body), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg, err := config.Load(filepath.Join(dir, "config.yaml"))
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if got := cfg.SessionStore().Path(); got != custom {
		t.Fatalf("session path = %q, want %q", got, custom)
	}
}

func TestMultiProfileConfigDefaults(t *testing.T) {
	dir := t.TempDir()
	body := "default_profile: work\nprofiles:\n  work:\n    sources:\n      - indeed\n  default:\n    sources:\n      - indeed\n      - linkedin\n"
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(body), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg, err := config.Load(filepath.Join(dir, "config.yaml"))
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.ProfileName != "work" {
		t.Fatalf("profile = %q, want work", cfg.ProfileName)
	}
	if sources := cfg.Sources(); len(sources) != 1 || sources[0] != "indeed" {
		t.Fatalf("sources = %#v", sources)
	}
}

func TestDefaultSourcesWhenNoConfig(t *testing.T) {
	cfg, err := config.Load(filepath.Join(t.TempDir(), "missing.yaml"))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	sources := cfg.Sources()
	if len(sources) != 2 || sources[0] != "indeed" || sources[1] != "linkedin" {
		t.Fatalf("default sources = %#v", sources)
	}
	if cfg.Active == nil || cfg.Active.Timeout <= 0 {
		t.Fatalf("default timeout = %#v", cfg.Active)
	}
}
