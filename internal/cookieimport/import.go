package cookieimport

import (
	"context"
	"sort"
	"strings"

	"github.com/browserutils/kooky"
	_ "github.com/browserutils/kooky/browser/all"

	"github.com/thedavidweng/jobs-cli/internal/config"
	"github.com/thedavidweng/jobs-cli/internal/errors"
)

type Importer struct{}

func New() *Importer {
	return &Importer{}
}

func DetectBrowsers(ctx context.Context) []string {
	seen := map[string]bool{}
	var out []string
	for store, err := range kooky.TraverseCookieStores(ctx) {
		if err != nil || store == nil {
			continue
		}
		name := strings.ToLower(store.Browser())
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func UnsupportedBrowser(browser string) bool {
	if browser == "" {
		return false
	}
	for _, name := range []string{"chrome", "chromium", "brave", "edge", "safari", "firefox"} {
		if strings.EqualFold(browser, name) {
			return false
		}
	}
	return true
}

func (i *Importer) Import(ctx context.Context, browser string) (*config.LinkedInSession, error) {
	detected := DetectBrowsers(ctx)
	message := "guided LinkedIn session import is not implemented yet"
	if browser != "" {
		message += " (requested browser: " + browser + ")"
	}
	if len(detected) > 0 {
		message += " (cookie stores detected: " + strings.Join(detected, ", ") + ")"
	}
	return nil, errors.New(errors.NotImplemented, message, errors.CatInternal, false, nil)
}
