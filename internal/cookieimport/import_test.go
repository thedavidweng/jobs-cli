package cookieimport

import (
	"context"
	"strings"
	"testing"

	"github.com/thedavidweng/jobs-cli/v2/internal/config"
	joberrors "github.com/thedavidweng/jobs-cli/v2/internal/errors"
)

func TestImportAutoDetectsBrowserOrder(t *testing.T) {
	var attempts []string
	importer := &Importer{readBrowser: func(_ context.Context, browser string) (map[string]string, bool) {
		attempts = append(attempts, browser)
		if browser != "firefox" {
			return nil, false
		}
		return map[string]string{
			config.CookieLiAt:       "li-at-secret",
			config.CookieJSessionID: `"ajax:1234"`,
		}, true
	}}

	session, err := importer.Import(context.Background(), "")
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if got, want := strings.Join(attempts, ","), "chrome,safari,firefox"; got != want {
		t.Fatalf("attempts = %q, want %q", got, want)
	}
	if session.Browser != "firefox" || session.CSRFToken != "ajax:1234" {
		t.Fatalf("session = %#v", session)
	}
}

func TestImportBrowserOverride(t *testing.T) {
	var attempts []string
	importer := &Importer{readBrowser: func(_ context.Context, browser string) (map[string]string, bool) {
		attempts = append(attempts, browser)
		return map[string]string{
			config.CookieLiAt:       "li-at-secret",
			config.CookieJSessionID: `"ajax:1234"`,
		}, true
	}}

	session, err := importer.Import(context.Background(), "safari")
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if got, want := strings.Join(attempts, ","), "safari"; got != want {
		t.Fatalf("attempts = %q, want %q", got, want)
	}
	if session.Browser != "safari" {
		t.Fatalf("browser = %q", session.Browser)
	}
}

func TestImportMissingCookiesIsActionable(t *testing.T) {
	importer := &Importer{readBrowser: func(context.Context, string) (map[string]string, bool) {
		return map[string]string{config.CookieLiAt: "li-at-secret"}, true
	}}

	_, err := importer.Import(context.Background(), "chrome")
	e := joberrors.From(err)
	if e.Code != joberrors.LinkedInSessionRequired || !strings.Contains(e.Message, "log in to linkedin.com") {
		t.Fatalf("error = %#v", e)
	}
}
