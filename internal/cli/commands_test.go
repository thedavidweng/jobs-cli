package cli_test

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/thedavidweng/jobs-cli/v2/internal/config"
)

func TestSourcesStatusAndDoctorReportDisjointContent(t *testing.T) {
	h := newHarness(t).useRealRegistry()

	sourcesOut, _, code := h.run("--json", "sources", "status")
	if code != 0 {
		t.Fatalf("sources status exit = %d", code)
	}
	sources := decodeEnvelope(t, sourcesOut)
	entries, ok := sources.Data["sources"].([]any)
	if !ok || len(entries) == 0 {
		t.Fatalf("sources status data.sources = %#v", sources.Data["sources"])
	}
	providers, ok := sources.Data["providers"].([]any)
	if !ok || len(providers) == 0 {
		t.Fatalf("sources status data.providers = %#v", sources.Data["providers"])
	}
	if _, ok := sources.Data["checks"]; ok {
		t.Fatal("sources status must not report local install checks")
	}
	if strings.Contains(sourcesOut, "config_file") {
		t.Fatal("sources status leaked doctor content")
	}

	greenhouseEntry := findEntry(t, providers, "greenhouse")
	if greenhouseEntry["native_submit"] != false || greenhouseEntry["verification"] != "PARTIALLY VERIFIED" {
		t.Fatalf("greenhouse capabilities = %#v", greenhouseEntry)
	}
	leverEntry := findEntry(t, providers, "lever")
	if leverEntry["browser_required"] != true || leverEntry["native_submit"] != false {
		t.Fatalf("lever capabilities = %#v", leverEntry)
	}

	doctorOut, _, code := h.run("--json", "doctor")
	if code != 0 {
		t.Fatalf("doctor exit = %d", code)
	}
	doctor := decodeEnvelope(t, doctorOut)
	checks, ok := doctor.Data["checks"].([]any)
	if !ok || len(checks) == 0 {
		t.Fatalf("doctor data.checks = %#v", doctor.Data["checks"])
	}
	if _, ok := doctor.Data["providers"]; ok {
		t.Fatal("doctor must not duplicate the provider capability matrix")
	}
}

type deadTransport struct{}

func (deadTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errors.New("offline")
}

func TestDoctorConnectAddsOptionalCheck(t *testing.T) {
	h := newHarness(t).useRealRegistry().withTransport(deadTransport{})
	out, _, code := h.run("--json", "doctor", "--connect")
	if code != 0 {
		t.Fatalf("exit = %d", code)
	}
	doc := decodeEnvelope(t, out)
	checks, ok := doc.Data["checks"].([]any)
	if !ok {
		t.Fatalf("data.checks = %#v", doc.Data["checks"])
	}
	seen := map[string]bool{}
	for _, raw := range checks {
		check, cok := raw.(map[string]any)
		if !cok {
			t.Fatalf("check = %#v", raw)
		}
		if name, isConnect := check["check"].(string); isConnect && strings.HasPrefix(name, "connect:") {
			seen[name] = true
			if check["ok"] != false || check["status"] != "warn" {
				t.Fatalf("offline connect check %q should be a warn with ok=false: %#v", name, check)
			}
		}
	}
	for _, want := range []string{"connect:indeed", "connect:linkedin", "connect:greenhouse"} {
		if !seen[want] {
			t.Fatalf("--connect missing check %q; seen = %#v", want, seen)
		}
	}
}

func TestMultiProfileConfigSelectsProfile(t *testing.T) {
	const configBody = "default_profile: work\nprofiles:\n  default:\n    sources:\n      - indeed\n      - linkedin\n  work:\n    sources:\n      - indeed\n"

	t.Run("profile_from_config", func(t *testing.T) {
		h := newHarness(t).useRealRegistry()
		h.writeConfig(configBody)
		out, _, code := h.run("--json", "version")
		if code != 0 {
			t.Fatalf("exit = %d", code)
		}
		if got := decodeEnvelope(t, out).Meta.Profile; got != "work" {
			t.Fatalf("meta.profile = %q, want work", got)
		}
	})

	t.Run("profile_flag_overrides_config", func(t *testing.T) {
		h := newHarness(t).useRealRegistry()
		h.writeConfig(configBody)
		out, _, code := h.run("--json", "--profile", "default", "version")
		if code != 0 {
			t.Fatalf("exit = %d", code)
		}
		if got := decodeEnvelope(t, out).Meta.Profile; got != "default" {
			t.Fatalf("meta.profile = %q, want default", got)
		}
	})
}

func TestAuthStatusReportsSessionWithoutSecrets(t *testing.T) {
	h := newHarness(t).useRealRegistry()
	store := config.NewSessionStore(h.dir)
	session := config.NewLinkedInSession("default", "chrome", map[string]string{
		config.CookieLiAt:       "top-secret-li-at",
		config.CookieJSessionID: `"ajax:csrf"`,
	})
	if err := store.Save(session); err != nil {
		t.Fatalf("save session: %v", err)
	}

	out, errOut, code := h.run("--json", "auth", "status")
	if code != 0 {
		t.Fatalf("exit = %d", code)
	}
	if strings.Contains(out+errOut, "top-secret-li-at") {
		t.Fatalf("auth status leaked a session secret:\n%s\n%s", out, errOut)
	}
	doc := decodeEnvelope(t, out)
	linkedin, ok := doc.Data["linkedin"].(map[string]any)
	if !ok {
		t.Fatalf("data.linkedin = %#v", doc.Data["linkedin"])
	}
	if linkedin["authenticated"] != true {
		t.Fatalf("linkedin.authenticated = %v, want true", linkedin["authenticated"])
	}
	sessionData, ok := linkedin["session"].(map[string]any)
	if !ok || sessionData["complete"] != true {
		t.Fatalf("linkedin.session = %#v", linkedin["session"])
	}
	names, ok := sessionData["cookie_names"].([]any)
	if !ok || len(names) != 2 {
		t.Fatalf("cookie names = %#v", sessionData["cookie_names"])
	}

	logoutOut, _, code := h.run("--json", "auth", "logout")
	if code != 0 {
		t.Fatalf("logout exit = %d", code)
	}
	if decodeEnvelope(t, logoutOut).Data["removed"] != true {
		t.Fatalf("logout did not remove the session: %s", logoutOut)
	}

	secondOut, _, code := h.run("--json", "auth", "logout")
	if code != 0 {
		t.Fatalf("second logout exit = %d", code)
	}
	if decodeEnvelope(t, secondOut).Data["removed"] != false {
		t.Fatal("second logout should report removed=false")
	}

	statusOut, _, code := h.run("--json", "auth", "status")
	if code != 0 {
		t.Fatalf("status exit = %d", code)
	}
	linkedin, lok := decodeEnvelope(t, statusOut).Data["linkedin"].(map[string]any)
	if !lok {
		t.Fatal("data.linkedin is not an object")
	}
	if linkedin["authenticated"] != false {
		t.Fatal("status still reports authenticated after logout")
	}
}

func findEntry(t *testing.T, entries []any, name string) map[string]any {
	t.Helper()
	for _, raw := range entries {
		entry, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if entry["name"] == name {
			return entry
		}
	}
	t.Fatalf("entry %q not found in %#v", name, entries)
	return nil
}
