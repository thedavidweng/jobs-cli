package cli_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/thedavidweng/jobs-cli/v2/internal/config"
)

const validImportJSON = `{"schema_version":"1","provider":"linkedin","profile":"default","captured_at":"2026-09-26T12:00:00Z","browser":"manual","csrf_token":"ajax:1234567890","cookies":{"li_at":"li-at-secret","JSESSIONID":"\"ajax:1234567890\""}}`

func sessionPath(t *testing.T, h *harness) string {
	t.Helper()
	return config.NewSessionStore(h.dir).Path()
}

func TestAuthLinkedInImportFromStdinStoresSession(t *testing.T) {
	h := newHarness(t).useRealRegistry()
	h.withStdin(validImportJSON)

	out, errOut, code := h.run("--json", "auth", "linkedin", "import", "--from-json", "-")
	if code != 0 {
		t.Fatalf("exit = %d (stderr: %s)", code, errOut)
	}
	doc := decodeEnvelope(t, out)
	if !doc.OK || doc.Data["profile"] != "default" {
		t.Fatalf("envelope = %#v", doc)
	}
	if strings.Contains(out+errOut, "li-at-secret") {
		t.Fatalf("import leaked session material:\n%s%s", out, errOut)
	}

	path := sessionPath(t, h)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("session file missing: %v", err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("session file mode = %04o, want 0600", info.Mode().Perm())
	}
	if dirInfo, derr := os.Stat(filepath.Dir(path)); derr == nil && runtime.GOOS != "windows" && dirInfo.Mode().Perm() != 0o700 {
		t.Fatalf("sessions dir mode = %04o, want 0700", dirInfo.Mode().Perm())
	}

	statusOut, _, code := h.run("--json", "auth", "status")
	if code != 0 {
		t.Fatalf("auth status exit = %d", code)
	}
	statusDoc := decodeEnvelope(t, statusOut)
	linkedin, ok := statusDoc.Data["linkedin"].(map[string]any)
	if !ok {
		t.Fatalf("data.linkedin = %#v", statusDoc.Data["linkedin"])
	}
	if linkedin["authenticated"] != true {
		t.Fatalf("linkedin.authenticated = %v, want true after import", linkedin["authenticated"])
	}
	sessionData, ok := linkedin["session"].(map[string]any)
	if !ok || sessionData["invalid"] != false || sessionData["present"] != true {
		t.Fatalf("linkedin.session = %#v", linkedin["session"])
	}
}

func TestAuthLinkedInImportNormalizesSmartQuotedJSessionID(t *testing.T) {
	h := newHarness(t).useRealRegistry()
	body := `{"schema_version":"1","provider":"linkedin","profile":"default","captured_at":"2026-09-26T12:00:00Z","cookies":{"li_at":"li-at-secret","JSESSIONID":"“ajax:1234567890”"}}`
	h.withStdin(body)

	out, _, code := h.run("--json", "auth", "linkedin", "import", "--from-json", "-")
	if code != 0 {
		t.Fatalf("exit = %d (out: %s)", code, out)
	}
	loaded, err := config.NewSessionStore(h.dir).Load()
	if err != nil {
		t.Fatalf("load imported session: %v", err)
	}
	if got := loaded.Cookie(config.CookieJSessionID); got != "ajax:1234567890" {
		t.Fatalf("stored JSESSIONID = %q, want the smart-quote artifact removed", got)
	}
	if got := loaded.CSRFTokenValue(); got != "ajax:1234567890" {
		t.Fatalf("csrf token = %q", got)
	}
}

func TestAuthLinkedInImportRejectsInvalidInputWithoutWriting(t *testing.T) {
	tests := []struct {
		name   string
		stdin  string
		reason string
	}{
		{"malformed json", `{"schema_version":`, "not valid"},
		{"wrong schema version", `{"schema_version":"2","provider":"linkedin","profile":"default","captured_at":"2026-09-26T12:00:00Z","cookies":{"li_at":"li-at-secret","JSESSIONID":"\"ajax:1234567890\""}}`, "schema_version"},
		{"wrong provider", `{"schema_version":"1","provider":"greenhouse","profile":"default","captured_at":"2026-09-26T12:00:00Z","cookies":{"li_at":"li-at-secret","JSESSIONID":"\"ajax:1234567890\""}}`, "provider"},
		{"profile mismatch", `{"schema_version":"1","provider":"linkedin","profile":"work","captured_at":"2026-09-26T12:00:00Z","cookies":{"li_at":"li-at-secret","JSESSIONID":"\"ajax:1234567890\""}}`, "does not match the active profile"},
		{"missing cookies", `{"schema_version":"1","provider":"linkedin","profile":"default","captured_at":"2026-09-26T12:00:00Z","cookies":{}}`, "li_at"},
		{"non ajax csrf token", `{"schema_version":"1","provider":"linkedin","profile":"default","captured_at":"2026-09-26T12:00:00Z","csrf_token":"garbage-token","cookies":{"li_at":"li-at-secret","JSESSIONID":"\"ajax:1234567890\""}}`, "csrf_token"},
		{"unnormalizable jsessionid", `{"schema_version":"1","provider":"linkedin","profile":"default","captured_at":"2026-09-26T12:00:00Z","cookies":{"li_at":"li-at-secret","JSESSIONID":"“not-an-ajax-token”"}}`, "JSESSIONID"},
		{"empty payload", "", "empty"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(t).useRealRegistry()
			h.withStdin(tt.stdin)
			out, errOut, code := h.run("--json", "auth", "linkedin", "import", "--from-json", "-")
			if code != 2 {
				t.Fatalf("exit = %d, want 2 (out: %s, err: %s)", code, out, errOut)
			}
			doc := decodeEnvelope(t, out)
			requireCode(t, &doc, "INVALID_ARGUMENTS", 2, code)
			if !strings.Contains(doc.Error.Message, tt.reason) {
				t.Fatalf("error message %q does not mention %q", doc.Error.Message, tt.reason)
			}
			if strings.Contains(out+errOut, "li-at-secret") {
				t.Fatalf("validation error leaked session material:\n%s%s", out, errOut)
			}
			if _, err := os.Stat(sessionPath(t, h)); !os.IsNotExist(err) {
				t.Fatalf("invalid input wrote a session file")
			}
		})
	}
}

func TestAuthLinkedInImportRequiresFromJSONFlag(t *testing.T) {
	h := newHarness(t).useRealRegistry()
	out, _, code := h.run("--json", "auth", "linkedin", "import")
	if code != 2 {
		t.Fatalf("exit = %d, want 2 (out: %s)", code, out)
	}
	doc := decodeEnvelope(t, out)
	requireCode(t, &doc, "INVALID_ARGUMENTS", 2, code)
	if !strings.Contains(doc.Error.Message, "--from-json") {
		t.Fatalf("error message = %q", doc.Error.Message)
	}
}

func TestAuthStatusAndDoctorReportPresentButInvalidSession(t *testing.T) {
	h := newHarness(t).useRealRegistry()
	if err := os.MkdirAll(filepath.Join(h.dir, "sessions"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sessionPath(t, h), []byte(`{"schema_version":"1","profile":"default"`), 0o600); err != nil {
		t.Fatal(err)
	}

	out, _, code := h.run("--json", "auth", "status")
	if code != 0 {
		t.Fatalf("auth status exit = %d", code)
	}
	doc := decodeEnvelope(t, out)
	linkedin, ok := doc.Data["linkedin"].(map[string]any)
	if !ok {
		t.Fatalf("data.linkedin = %#v", doc.Data["linkedin"])
	}
	if linkedin["authenticated"] != false {
		t.Fatalf("authenticated = %v, want false for an invalid session", linkedin["authenticated"])
	}
	sessionData, ok := linkedin["session"].(map[string]any)
	if !ok || sessionData["present"] != true || sessionData["invalid"] != true {
		t.Fatalf("linkedin.session = %#v, want present but invalid", linkedin["session"])
	}
	if reason, _ := sessionData["invalid_reason"].(string); reason == "" {
		t.Fatalf("invalid_reason is empty: %#v", sessionData)
	}

	humanOut, _, code := h.run("auth", "status")
	if code != 0 {
		t.Fatalf("human auth status exit = %d", code)
	}
	if !strings.Contains(humanOut, "present but invalid") {
		t.Fatalf("human output missing the present-but-invalid state:\n%s", humanOut)
	}
	if strings.Contains(humanOut, "run `jobs-cli auth linkedin login` to import a session") && strings.Contains(humanOut, "absent") {
		t.Fatalf("human output reports the session as absent:\n%s", humanOut)
	}

	doctorOut, _, code := h.run("--json", "doctor")
	if code != 0 {
		t.Fatalf("doctor exit = %d", code)
	}
	doctorDoc := decodeEnvelope(t, doctorOut)
	checks, ok := doctorDoc.Data["checks"].([]any)
	if !ok {
		t.Fatalf("data.checks = %#v", doctorDoc.Data["checks"])
	}
	found := false
	for _, raw := range checks {
		check, cok := raw.(map[string]any)
		if !cok || check["check"] != "session" {
			continue
		}
		found = true
		if check["ok"] != false || check["status"] != "warn" {
			t.Fatalf("doctor session check = %#v, want warn with ok=false", check)
		}
		if detail, _ := check["detail"].(string); !strings.Contains(detail, "present but invalid") {
			t.Fatalf("doctor session detail = %q, want present but invalid", detail)
		}
	}
	if !found {
		t.Fatal("doctor output has no session check")
	}

	doctorText, _, code := h.run("doctor")
	if code != 0 {
		t.Fatalf("human doctor exit = %d", code)
	}
	for _, want := range []string{"[WARN] session", "1 problem found. Fix the WARN line above."} {
		if !strings.Contains(doctorText, want) {
			t.Fatalf("human doctor output missing %q:\n%s", want, doctorText)
		}
	}
}
