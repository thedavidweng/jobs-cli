package e2e

import (
	"strings"
	"testing"

	"github.com/thedavidweng/jobs-cli/v2/internal/testutil"
)

func TestAuthLinkedInLoginRejectsUnsupportedBrowser(t *testing.T) {
	stdout, stderr, code := testutil.RunBinary(t, []string{"JOBS_CONFIG_DIR=" + t.TempDir()}, "--json", "auth", "linkedin", "login", "--browser", "opera")
	if code != 2 {
		t.Fatalf("exit = %d, want 2 (stderr: %s)", code, stderr)
	}
	doc := decodeOne(t, stdout)
	if doc.OK || doc.Error == nil || doc.Error.Code != "INVALID_ARGUMENTS" {
		t.Fatalf("envelope = %#v", doc)
	}
	if strings.Contains(stdout, "li_at") || strings.Contains(stderr, "li_at") {
		t.Fatal("login failure exposed cookie material")
	}
}
