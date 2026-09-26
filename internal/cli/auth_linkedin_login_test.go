package cli

import (
	"bytes"
	"context"
	"net/http"
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

func TestAuthLinkedInLoginPromptsAndDoesNotPrintSecrets(t *testing.T) {
	original := openLoginPage
	openLoginPage = func(context.Context) error { return nil }
	t.Cleanup(func() { openLoginPage = original })

	var stdout, stderr bytes.Buffer
	dir := t.TempDir()
	app := New(&Options{
		Stdout:    &stdout,
		Stderr:    &stderr,
		Stdin:     strings.NewReader("\n"),
		ConfigDir: dir,
		RegistryFactory: func(*config.Config, *http.Client) *registry.Registry {
			return &registry.Registry{LinkedInAuth: session.New(config.NewSessionStore(dir), loginReader{})}
		},
	})
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
