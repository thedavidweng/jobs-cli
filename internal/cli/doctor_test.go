package cli_test

import (
	"path/filepath"
	"strings"
	"testing"
)

func doctorChecks(t *testing.T, out string) map[string]map[string]any {
	t.Helper()
	doc := decodeEnvelope(t, out)
	raw, ok := doc.Data["checks"].([]any)
	if !ok {
		t.Fatalf("data.checks = %#v", doc.Data["checks"])
	}
	checks := make(map[string]map[string]any, len(raw))
	for _, item := range raw {
		check, ok := item.(map[string]any)
		if !ok {
			t.Fatalf("check = %#v", item)
		}
		name, _ := check["check"].(string)
		checks[name] = check
	}
	return checks
}

func TestDoctorFreshInstallReportsNoProblems(t *testing.T) {
	h := newHarness(t).useRealRegistry()
	h.dir = filepath.Join(h.dir, "not-created")

	out, _, code := h.run("--json", "doctor")
	if code != 0 {
		t.Fatalf("exit = %d", code)
	}
	checks := doctorChecks(t, out)
	for name, check := range checks {
		if check["status"] == "warn" || check["ok"] != true {
			t.Errorf("check %q = %#v, want no problem on a fresh install", name, check)
		}
	}
	configDir := checks["config_dir"]
	if detail, _ := configDir["detail"].(string); configDir["status"] != "ok" || !strings.Contains(detail, "not created yet") {
		t.Errorf("config_dir = %#v, want ok and not created yet", configDir)
	}
	session := checks["session"]
	if detail, _ := session["detail"].(string); session["status"] != "info" || !strings.Contains(detail, "jobs-cli auth linkedin login") {
		t.Errorf("session = %#v, want info with the login command", session)
	}

	text, _, code := h.run("doctor")
	if code != 0 {
		t.Fatalf("text exit = %d", code)
	}
	if strings.Contains(text, "[WARN]") {
		t.Errorf("fresh install has a WARN line:\n%s", text)
	}
	for _, want := range []string{"[info] session", "No problems found."} {
		if !strings.Contains(text, want) {
			t.Errorf("text output missing %q:\n%s", want, text)
		}
	}
}

func TestDoctorCountsProblems(t *testing.T) {
	h := newHarness(t).useRealRegistry().withTransport(deadTransport{})
	text, _, code := h.run("doctor", "--connect")
	if code != 0 {
		t.Fatalf("exit = %d", code)
	}
	if got := strings.Count(text, "[WARN] connect:"); got != 3 {
		t.Errorf("WARN connect lines = %d, want 3:\n%s", got, text)
	}
	if !strings.Contains(text, "3 problems found. Fix the WARN lines above.") {
		t.Errorf("text output missing the problem count:\n%s", text)
	}
}
