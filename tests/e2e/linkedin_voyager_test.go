package e2e

import (
	"testing"

	"github.com/thedavidweng/jobs-cli/internal/testutil"
)

func TestLinkedInAuthenticatedSearchRequiresSession(t *testing.T) {
	stdout, stderr, code := testutil.RunBinary(t, []string{"JOBS_CONFIG_DIR=" + t.TempDir()},
		"--json", "search", "--source", "linkedin", "--authenticated", "--query", "go")
	if code != 3 {
		t.Fatalf("exit = %d, want 3 (stderr: %s)", code, stderr)
	}
	doc := decodeOne(t, stdout)
	if doc.Error == nil || doc.Error.Code != "LINKEDIN_SESSION_REQUIRED" {
		t.Fatalf("envelope = %#v", doc)
	}
}
