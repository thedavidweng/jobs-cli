package e2e

import (
	"strings"
	"testing"

	"github.com/thedavidweng/jobs-cli/internal/testutil"
)

func TestApplyHelpListsLifecycle(t *testing.T) {
	stdout, stderr, code := testutil.RunBinary(t, nil, "apply", "--help")
	if code != 0 {
		t.Fatalf("apply --help exit = %d (stderr: %s)", code, stderr)
	}
	for _, subcommand := range []string{"inspect", "prepare", "submit"} {
		if !strings.Contains(stdout, subcommand) {
			t.Errorf("apply subcommand %q missing from help:\n%s", subcommand, stdout)
		}
	}
}

func TestApplyPrepareHelpListsInputFlags(t *testing.T) {
	stdout, stderr, code := testutil.RunBinary(t, nil, "apply", "prepare", "--help")
	if code != 0 {
		t.Fatalf("apply prepare --help exit = %d (stderr: %s)", code, stderr)
	}
	for _, want := range []string{"--manifest", "--answers", "--resume", "--cover-letter", "--out"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("flag %q missing from apply prepare --help:\n%s", want, stdout)
		}
	}
}

func TestApplySubmitHelpListsArtifactFlag(t *testing.T) {
	stdout, stderr, code := testutil.RunBinary(t, nil, "apply", "submit", "--help")
	if code != 0 {
		t.Fatalf("apply submit --help exit = %d (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stdout, "--artifact") {
		t.Errorf("flag --artifact missing from apply submit --help:\n%s", stdout)
	}
}

func TestApplyRejectsMalformedJobID(t *testing.T) {
	env := []string{"JOBS_CONFIG_DIR=" + t.TempDir()}
	for _, args := range [][]string{
		{"--json", "apply", "inspect", "not-a-job-id"},
		{"--json", "apply", "prepare", "not-a-job-id", "--manifest", "-"},
	} {
		stdout, _, code := testutil.RunBinary(t, env, args...)
		if code != 2 {
			t.Fatalf("%v exit = %d, want 2\n%s", args, code, stdout)
		}
		if doc := decodeOne(t, stdout); doc.Error == nil || doc.Error.Code != "INVALID_ARGUMENTS" {
			t.Fatalf("%v envelope = %+v", args, doc)
		}
	}
}

func TestApplySubmitRequiresArtifact(t *testing.T) {
	env := []string{"JOBS_CONFIG_DIR=" + t.TempDir()}
	stdout, _, code := testutil.RunBinary(t, env, "--json", "apply", "submit")
	if code != 2 {
		t.Fatalf("exit = %d, want 2", code)
	}
	if doc := decodeOne(t, stdout); doc.Error == nil || doc.Error.Code != "INVALID_ARGUMENTS" {
		t.Fatalf("envelope = %+v", doc)
	}
}

func TestApplySubmitRejectsMissingArtifactFile(t *testing.T) {
	env := []string{"JOBS_CONFIG_DIR=" + t.TempDir()}
	stdout, _, code := testutil.RunBinary(t, env, "--json", "apply", "submit", "--artifact", t.TempDir()+"/missing.json")
	if code != 2 {
		t.Fatalf("exit = %d, want 2", code)
	}
	if doc := decodeOne(t, stdout); doc.Error == nil || doc.Error.Code != "INVALID_ARGUMENTS" {
		t.Fatalf("envelope = %+v", doc)
	}
}

func TestApplySubmitRejectsMalformedArtifact(t *testing.T) {
	env := []string{"JOBS_CONFIG_DIR=" + t.TempDir()}
	stdout, _, code := testutil.RunBinaryInput(t, "not-json", env, "--json", "apply", "submit", "--artifact", "-", "--confirm")
	if code != 2 {
		t.Fatalf("exit = %d, want 2", code)
	}
	if doc := decodeOne(t, stdout); doc.Error == nil || doc.Error.Code != "INVALID_ARGUMENTS" {
		t.Fatalf("envelope = %+v", doc)
	}
}

func TestApplySubmitConsumesArtifactAndRoutesToProvider(t *testing.T) {
	env := []string{"JOBS_CONFIG_DIR=" + t.TempDir()}
	artifact := `{"schema_version":"2026-09-26","artifact_version":1,"job_id":"indeed:1","provider":"workday","fingerprint":"sha256:test","application":{"url":"https://example.com/jobs/1","provider":"workday"}}`
	stdout, _, code := testutil.RunBinaryInput(t, artifact, env, "--json", "apply", "submit", "--artifact", "-", "--confirm")
	if code != 6 {
		t.Fatalf("exit = %d, want 6\n%s", code, stdout)
	}
	if doc := decodeOne(t, stdout); doc.Error == nil || doc.Error.Code != "NATIVE_APPLY_UNSUPPORTED" {
		t.Fatalf("envelope = %+v", doc)
	}
}
