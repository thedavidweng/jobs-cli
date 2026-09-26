package session

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/thedavidweng/jobs-cli/internal/config"
)

type fakeReader struct {
	session *config.LinkedInSession
	browser string
}

func (f *fakeReader) Import(_ context.Context, browser string) (*config.LinkedInSession, error) {
	f.browser = browser
	return f.session, nil
}

func TestLoginSavesCompleteSession(t *testing.T) {
	store := config.NewSessionStore(t.TempDir())
	reader := &fakeReader{session: config.NewLinkedInSession("ignored", "chrome", map[string]string{
		config.CookieLiAt:       "li-at-secret",
		config.CookieJSessionID: `"ajax:1234"`,
	})}
	reader.session.CSRFToken = "ajax:1234"

	got, err := New(store, reader).Login(context.Background(), "chrome")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if got.Profile != "default" || reader.browser != "chrome" {
		t.Fatalf("session = %#v, browser = %q", got, reader.browser)
	}
	info, err := os.Stat(filepath.Join(store.Dir(), "default.json"))
	if err != nil {
		t.Fatalf("stat session: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("session mode = %04o, want 0600", info.Mode().Perm())
	}
}
