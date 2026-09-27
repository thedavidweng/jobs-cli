package session

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/thedavidweng/jobs-cli/v2/internal/config"
	joberrors "github.com/thedavidweng/jobs-cli/v2/internal/errors"
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
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("session mode = %04o, want 0600", info.Mode().Perm())
	}
}

func TestLoginRejectsInvalidImportedSessionWithoutWriting(t *testing.T) {
	store := config.NewSessionStore(t.TempDir())
	reader := &fakeReader{session: config.NewLinkedInSession("ignored", "chrome", map[string]string{
		config.CookieLiAt:       "li-at-secret",
		config.CookieJSessionID: `"ajax:1234"`,
	})}
	reader.session.CSRFToken = "not-an-ajax-token"

	_, err := New(store, reader).Login(context.Background(), "chrome")
	if err == nil {
		t.Fatal("Login accepted an imported session with a malformed csrf token")
	}
	if got := joberrors.From(err).Code; got != joberrors.LinkedInSessionRequired {
		t.Fatalf("error code = %s, want LINKEDIN_SESSION_REQUIRED", got)
	}
	if strings.Contains(err.Error(), "li-at-secret") || strings.Contains(err.Error(), "ajax:1234") {
		t.Fatalf("error leaked session material: %v", err)
	}
	if _, statErr := os.Stat(store.Path()); !os.IsNotExist(statErr) {
		t.Fatalf("invalid session was written to %s", store.Path())
	}
}
