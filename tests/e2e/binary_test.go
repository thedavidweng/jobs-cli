package e2e

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/thedavidweng/jobs-cli/internal/testutil"
)

type envelope struct {
	OK    bool           `json:"ok"`
	Data  map[string]any `json:"data"`
	Error *struct {
		Code     string `json:"code"`
		Message  string `json:"message"`
		Category string `json:"category"`
	} `json:"error"`
	Meta struct {
		Command       string `json:"command"`
		SchemaVersion string `json:"schema_version"`
		RequestID     string `json:"request_id"`
	} `json:"meta"`
}

func decodeOne(t *testing.T, stdout string) envelope {
	t.Helper()
	if strings.TrimSpace(stdout) == "" {
		t.Fatal("stdout is empty; expected one JSON document")
	}
	if lines := strings.Split(strings.TrimSpace(stdout), "\n"); len(lines) != 1 {
		t.Fatalf("stdout must be exactly one JSON document, got %d lines:\n%s", len(lines), stdout)
	}
	var env envelope
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\n%s", err, stdout)
	}
	return env
}

func TestHelpListsRequiredCommands(t *testing.T) {
	stdout, stderr, code := testutil.RunBinary(t, nil, "--help")
	if code != 0 {
		t.Fatalf("--help exit = %d (stderr: %s)", code, stderr)
	}
	for _, command := range []string{"search", "show", "resolve", "apply", "auth", "sources", "doctor", "version", "completion"} {
		if !strings.Contains(stdout, command) {
			t.Errorf("command %q missing from --help:\n%s", command, stdout)
		}
	}
}

func TestNoJobsSubhierarchy(t *testing.T) {
	_, _, code := testutil.RunBinary(t, nil, "jobs", "list")
	if code == 0 {
		t.Fatal("a `jobs` subhierarchy must not exist")
	}
}

func TestBinaryJSONContractsAndExitCodes(t *testing.T) {
	env := []string{"JOBS_CONFIG_DIR=" + t.TempDir()}

	t.Run("version", func(t *testing.T) {
		stdout, stderr, code := testutil.RunBinary(t, env, "--json", "version")
		if code != 0 {
			t.Fatalf("exit = %d (stderr: %s)", code, stderr)
		}
		doc := decodeOne(t, stdout)
		if !doc.OK || doc.Meta.Command != "version" {
			t.Fatalf("envelope = %+v", doc)
		}
		if doc.Meta.SchemaVersion == "" || doc.Meta.RequestID == "" {
			t.Fatalf("meta = %+v", doc.Meta)
		}
		if doc.Data["version"] == "" {
			t.Fatalf("data = %+v", doc.Data)
		}
	})

	t.Run("doctor", func(t *testing.T) {
		stdout, stderr, code := testutil.RunBinary(t, env, "--json", "doctor")
		if code != 0 {
			t.Fatalf("exit = %d (stderr: %s)", code, stderr)
		}
		doc := decodeOne(t, stdout)
		checks, ok := doc.Data["checks"].([]any)
		if !ok || len(checks) == 0 {
			t.Fatalf("data.checks = %#v", doc.Data)
		}
		if _, ok := doc.Data["providers"]; ok {
			t.Fatal("doctor must not report the provider matrix")
		}
	})

	t.Run("sources_status", func(t *testing.T) {
		stdout, stderr, code := testutil.RunBinary(t, env, "--json", "sources", "status")
		if code != 0 {
			t.Fatalf("exit = %d (stderr: %s)", code, stderr)
		}
		doc := decodeOne(t, stdout)
		if _, ok := doc.Data["sources"].([]any); !ok {
			t.Fatalf("data.sources = %#v", doc.Data)
		}
		if _, ok := doc.Data["providers"].([]any); !ok {
			t.Fatalf("data.providers = %#v", doc.Data)
		}
	})

	t.Run("auth_status", func(t *testing.T) {
		stdout, stderr, code := testutil.RunBinary(t, env, "--json", "auth", "status")
		if code != 0 {
			t.Fatalf("exit = %d (stderr: %s)", code, stderr)
		}
		doc := decodeOne(t, stdout)
		linkedin, ok := doc.Data["linkedin"].(map[string]any)
		if !ok {
			t.Fatalf("data.linkedin = %#v", doc.Data)
		}
		if linkedin["authenticated"] != false {
			t.Fatalf("expected no session, got %#v", linkedin)
		}
	})

	t.Run("search_all_sources_fail_offline", func(t *testing.T) {
		offline := append(append([]string{}, env...), "HTTP_PROXY=http://127.0.0.1:1", "HTTPS_PROXY=http://127.0.0.1:1")
		stdout, stderr, code := testutil.RunBinary(t, offline, "--json", "search", "-q", "go", "--country", "US")
		if code != 5 {
			t.Fatalf("exit = %d, want 5 (stderr: %s)", code, stderr)
		}
		doc := decodeOne(t, stdout)
		if doc.OK || doc.Error == nil || doc.Error.Category != "network" {
			t.Fatalf("envelope = %+v", doc)
		}
	})

	t.Run("unsupported_source", func(t *testing.T) {
		stdout, _, code := testutil.RunBinary(t, env, "--json", "search", "-q", "go", "--source", "lever")
		if code != 2 {
			t.Fatalf("exit = %d, want 2", code)
		}
		if doc := decodeOne(t, stdout); doc.Error == nil || doc.Error.Code != "INVALID_ARGUMENTS" {
			t.Fatalf("envelope = %+v", doc)
		}
	})

	t.Run("show_source_unreachable_offline", func(t *testing.T) {
		offline := append(append([]string{}, env...), "HTTP_PROXY=http://127.0.0.1:1", "HTTPS_PROXY=http://127.0.0.1:1")
		stdout, _, code := testutil.RunBinary(t, offline, "--json", "show", "indeed:1442")
		if code != 5 {
			t.Fatalf("exit = %d, want 5", code)
		}
		if doc := decodeOne(t, stdout); doc.Error == nil || doc.Error.Category != "network" {
			t.Fatalf("envelope = %+v", doc)
		}
	})

	t.Run("resolve_rejected_target", func(t *testing.T) {
		stdout, _, code := testutil.RunBinary(t, env, "--json", "resolve", "https://127.0.0.1/jobs/1")
		if code != 6 {
			t.Fatalf("exit = %d, want 6", code)
		}
		if doc := decodeOne(t, stdout); doc.Error == nil || doc.Error.Code != "ATS_RESOLUTION_FAILED" {
			t.Fatalf("envelope = %+v", doc)
		}
	})

	t.Run("completion", func(t *testing.T) {
		stdout, stderr, code := testutil.RunBinary(t, env, "completion", "bash")
		if code != 0 {
			t.Fatalf("exit = %d (stderr: %s)", code, stderr)
		}
		if stdout == "" {
			t.Fatal("completion output is empty")
		}
	})
}
