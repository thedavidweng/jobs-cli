package cli

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thedavidweng/jobs-cli/internal/config"
	"github.com/thedavidweng/jobs-cli/internal/linkedin/session"
	"github.com/thedavidweng/jobs-cli/internal/registry"
)

type loginReader struct{}

func (loginReader) Import(context.Context, string) (*config.LinkedInSession, error) {
	s := config.NewLinkedInSession("default", "chrome", map[string]string{
		config.CookieLiAt:       "li-at-secret",
		config.CookieJSessionID: `"ajax:1234"`,
	})
	s.CSRFToken = "ajax:1234"
	return s, nil
}

func newLoginApp(dir string, stdin *strings.Reader, stdout, stderr *bytes.Buffer) *App {
	return New(&Options{
		Stdout:    stdout,
		Stderr:    stderr,
		Stdin:     stdin,
		ConfigDir: dir,
		RegistryFactory: func(*config.Config, *http.Client) *registry.Registry {
			return &registry.Registry{LinkedInAuth: session.New(config.NewSessionStore(dir), loginReader{})}
		},
	})
}

func TestAuthLinkedInLoginPromptsAndDoesNotPrintSecrets(t *testing.T) {
	original := openLoginPage
	openLoginPage = func(context.Context) error { return nil }
	t.Cleanup(func() { openLoginPage = original })

	var stdout, stderr bytes.Buffer
	dir := t.TempDir()
	app := newLoginApp(dir, strings.NewReader("\n"), &stdout, &stderr)
	if code := app.Run([]string{"auth", "linkedin", "login"}); code != 0 {
		t.Fatalf("exit = %d, stderr = %s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "press Enter") {
		t.Fatalf("prompt missing from stderr: %q", stderr.String())
	}
	for _, secret := range []string{"li-at-secret", "ajax:1234"} {
		if strings.Contains(stdout.String(), secret) || strings.Contains(stderr.String(), secret) {
			t.Fatalf("secret %q appeared in output", secret)
		}
	}
	if !strings.Contains(stdout.String(), "stored LinkedIn session") {
		t.Fatalf("success output = %q", stdout.String())
	}
}

func TestAuthLinkedInLoginFallsBackWhenOpenerFails(t *testing.T) {
	original := openLoginPage
	openLoginPage = func(context.Context) error { return errors.New("xdg-open: not found") }
	t.Cleanup(func() { openLoginPage = original })

	var stdout, stderr bytes.Buffer
	dir := t.TempDir()
	app := newLoginApp(dir, strings.NewReader("\n"), &stdout, &stderr)
	if code := app.Run([]string{"auth", "linkedin", "login"}); code != 0 {
		t.Fatalf("exit = %d (INTERNAL_ERROR abort), stderr = %s", code, stderr.String())
	}
	prompt := stderr.String()
	if !strings.Contains(prompt, "could not open a browser automatically") {
		t.Fatalf("fallback prompt missing the opener-failure note: %q", prompt)
	}
	if !strings.Contains(prompt, "https://www.linkedin.com/login") || !strings.Contains(prompt, "press Enter") {
		t.Fatalf("fallback prompt missing the URL or Enter instruction: %q", prompt)
	}
	if _, err := os.Stat(filepath.Join(dir, "sessions", "default.json")); err != nil {
		t.Fatalf("login did not reach the import step: %v", err)
	}
	if !strings.Contains(stdout.String(), "stored LinkedIn session") {
		t.Fatalf("success output = %q", stdout.String())
	}
}

func TestAuthLinkedInLoginNoOpenSkipsBrowserAttempt(t *testing.T) {
	original := openLoginPage
	opened := false
	openLoginPage = func(context.Context) error {
		opened = true
		return nil
	}
	t.Cleanup(func() { openLoginPage = original })

	var stdout, stderr bytes.Buffer
	dir := t.TempDir()
	app := newLoginApp(dir, strings.NewReader("\n"), &stdout, &stderr)
	if code := app.Run([]string{"auth", "linkedin", "login", "--no-open"}); code != 0 {
		t.Fatalf("exit = %d, stderr = %s", code, stderr.String())
	}
	if opened {
		t.Fatal("--no-open must skip the browser opener entirely")
	}
	prompt := stderr.String()
	if !strings.Contains(prompt, "https://www.linkedin.com/login") || !strings.Contains(prompt, "press Enter") {
		t.Fatalf("manual prompt missing the URL or Enter instruction: %q", prompt)
	}
	if strings.Contains(prompt, "could not open a browser") {
		t.Fatalf("--no-open must not claim the opener failed: %q", prompt)
	}
	if _, err := os.Stat(filepath.Join(dir, "sessions", "default.json")); err != nil {
		t.Fatalf("login did not reach the import step: %v", err)
	}
}
