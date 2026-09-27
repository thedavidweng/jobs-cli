package cookieimport

import (
	"context"
	"strings"

	"github.com/browserutils/kooky"
	_ "github.com/browserutils/kooky/browser/all"

	"github.com/thedavidweng/jobs-cli/internal/config"
	joberrors "github.com/thedavidweng/jobs-cli/internal/errors"
)

var browserOrder = []string{"chrome", "safari", "firefox"}

type CookieReader interface {
	Import(context.Context, string) (*config.LinkedInSession, error)
}

type Importer struct {
	readBrowser func(context.Context, string) (map[string]string, bool)
}

func New() *Importer {
	return &Importer{}
}

func UnsupportedBrowser(browser string) bool {
	return browser != "" && canonicalBrowser(browser) == ""
}

func (i *Importer) Import(ctx context.Context, browser string) (*config.LinkedInSession, error) {
	browser = strings.ToLower(strings.TrimSpace(browser))
	if UnsupportedBrowser(browser) {
		return nil, sessionRequired("choose chrome, safari, or firefox with --browser")
	}
	if browser != "" {
		return i.importBrowser(ctx, canonicalBrowser(browser))
	}
	for _, candidate := range browserOrder {
		if session, err := i.importBrowser(ctx, candidate); err == nil {
			return session, nil
		}
	}
	return nil, sessionRequired("log in to linkedin.com in Chrome, Safari, or Firefox, then retry; if the browser is open, close it to unlock its cookie store")
}

func (i *Importer) importBrowser(ctx context.Context, browser string) (*config.LinkedInSession, error) {
	cookies, found := i.read(ctx, browser)
	if !found || cookies[config.CookieLiAt] == "" || cookies[config.CookieJSessionID] == "" {
		return nil, sessionRequired("log in to linkedin.com in " + browser + " and ensure its cookie store is available, then retry")
	}
	session := config.NewLinkedInSession("default", browser, cookies)
	session.CSRFToken = config.TrimCookieQuotes(cookies[config.CookieJSessionID])
	return session, nil
}

func (i *Importer) read(ctx context.Context, browser string) (map[string]string, bool) {
	if i.readBrowser != nil {
		return i.readBrowser(ctx, browser)
	}
	cookies := make(map[string]string)
	found := false
	for store, err := range kooky.TraverseCookieStores(ctx) {
		if err != nil || store == nil {
			continue
		}
		if canonicalBrowser(store.Browser()) != browser {
			_ = store.Close()
			continue
		}
		found = true
		for cookie, readErr := range store.TraverseCookies(linkedInCookie()) {
			if readErr != nil || cookie == nil {
				continue
			}
			cookies[cookie.Name] = cookie.Value
		}
		_ = store.Close()
	}
	return cookies, found
}

func linkedInCookie() kooky.Filter {
	return kooky.FilterFunc(func(cookie *kooky.Cookie) bool {
		if cookie == nil {
			return false
		}
		domain := strings.ToLower(strings.TrimPrefix(cookie.Domain, "."))
		if domain != "linkedin.com" && !strings.HasSuffix(domain, ".linkedin.com") {
			return false
		}
		switch cookie.Name {
		case config.CookieLiAt, config.CookieJSessionID, config.CookieLiAtCSRF:
			return true
		default:
			return false
		}
	})
}

func canonicalBrowser(browser string) string {
	switch strings.ToLower(strings.TrimSpace(browser)) {
	case "chrome", "chromium", "brave", "edge", "msedge":
		return "chrome"
	case "safari":
		return "safari"
	case "firefox":
		return "firefox"
	default:
		return ""
	}
}

func sessionRequired(action string) *joberrors.Error {
	return joberrors.New(joberrors.LinkedInSessionRequired, "LinkedIn session import failed: "+action, joberrors.CatAuth, false, nil)
}
