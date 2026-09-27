package e2e

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/thedavidweng/jobs-cli/v2/internal/testutil"
)

const importSessionJSON = `{"schema_version":"1","provider":"linkedin","profile":"default","captured_at":"2026-09-26T12:00:00Z","browser":"manual","csrf_token":"ajax:1234567890","cookies":{"li_at":"li-at-secret","JSESSIONID":"\"ajax:1234567890\""}}`

func linkedinStatus(t *testing.T, env []string) map[string]any {
	t.Helper()
	stdout, stderr, code := testutil.RunBinary(t, env, "--json", "auth", "status")
	if code != 0 {
		t.Fatalf("auth status exit = %d (stderr: %s)", code, stderr)
	}
	doc := decodeOne(t, stdout)
	linkedin, ok := doc.Data["linkedin"].(map[string]any)
	if !ok {
		t.Fatalf("data.linkedin = %#v", doc.Data["linkedin"])
	}
	return linkedin
}

func TestAuthLinkedInImportFromStdinReachesAuthenticatedState(t *testing.T) {
	env := []string{"JOBS_CONFIG_DIR=" + t.TempDir()}

	stdout, stderr, code := testutil.RunBinaryInput(t, importSessionJSON, env, "--json", "auth", "linkedin", "import", "--from-json", "-")
	if code != 0 {
		t.Fatalf("import exit = %d (stderr: %s)", code, stderr)
	}
	if doc := decodeOne(t, stdout); !doc.OK {
		t.Fatalf("import envelope = %+v", doc)
	}
	if strings.Contains(stdout+stderr, "li-at-secret") {
		t.Fatal("import leaked session material")
	}

	linkedin := linkedinStatus(t, env)
	if linkedin["authenticated"] != true {
		t.Fatalf("linkedin = %#v, want authenticated=true after import", linkedin)
	}
	session, ok := linkedin["session"].(map[string]any)
	if !ok || session["present"] != true || session["complete"] != true || session["invalid"] != false {
		t.Fatalf("linkedin.session = %#v", linkedin["session"])
	}

	dir := env[0][len("JOBS_CONFIG_DIR="):]
	info, err := os.Stat(filepath.Join(dir, "sessions", "default.json"))
	if err != nil {
		t.Fatalf("session file missing: %v", err)
	}
	if perm := info.Mode().Perm(); runtime.GOOS != "windows" && perm != 0o600 {
		t.Fatalf("session file mode = %04o, want 0600", perm)
	}
}

func TestAuthLinkedInImportRejectsInvalidInputWithoutWriting(t *testing.T) {
	env := []string{"JOBS_CONFIG_DIR=" + t.TempDir()}
	invalid := strings.Replace(importSessionJSON, `"schema_version":"1"`, `"schema_version":"2"`, 1)

	stdout, stderr, code := testutil.RunBinaryInput(t, invalid, env, "--json", "auth", "linkedin", "import", "--from-json", "-")
	if code != 2 {
		t.Fatalf("import exit = %d, want 2 (stderr: %s)", code, stderr)
	}
	doc := decodeOne(t, stdout)
	if doc.OK || doc.Error == nil || doc.Error.Code != "INVALID_ARGUMENTS" {
		t.Fatalf("envelope = %#v", doc)
	}

	linkedin := linkedinStatus(t, env)
	if linkedin["authenticated"] != false {
		t.Fatalf("linkedin = %#v, want authenticated=false after a rejected import", linkedin)
	}
	if session, ok := linkedin["session"].(map[string]any); !ok || session["present"] != false {
		t.Fatalf("linkedin.session = %#v, want absent (nothing written)", linkedin["session"])
	}
}

func TestAuthStatusReportsPresentButInvalidSession(t *testing.T) {
	dir := t.TempDir()
	env := []string{"JOBS_CONFIG_DIR=" + dir}
	if err := os.MkdirAll(filepath.Join(dir, "sessions"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sessions", "default.json"), []byte("{corrupt"), 0o600); err != nil {
		t.Fatal(err)
	}

	linkedin := linkedinStatus(t, env)
	if linkedin["authenticated"] != false {
		t.Fatalf("linkedin = %#v, want authenticated=false", linkedin)
	}
	session, ok := linkedin["session"].(map[string]any)
	if !ok || session["present"] != true || session["invalid"] != true {
		t.Fatalf("linkedin.session = %#v, want present but invalid", linkedin["session"])
	}
	if reason, _ := session["invalid_reason"].(string); !strings.Contains(reason, "not valid JSON") {
		t.Fatalf("invalid_reason = %q", reason)
	}
}
