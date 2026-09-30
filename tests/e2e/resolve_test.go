package e2e

import (
	"testing"

	"github.com/thedavidweng/jobs-cli/v2/internal/testutil"
)

func TestResolveJSONReturnsApplicationTargetOnly(t *testing.T) {
	env := []string{"JOBS_CONFIG_DIR=" + t.TempDir()}
	const url = "https://boards.greenhouse.io/acme/jobs/4123456"

	stdout, stderr, code := testutil.RunBinary(t, env, "--json", "resolve", url)
	if code != 0 {
		t.Fatalf("exit = %d (stderr: %s, stdout: %s)", code, stderr, stdout)
	}
	doc := decodeOne(t, stdout)
	if !doc.OK {
		t.Fatalf("ok = false: %+v", doc)
	}
	if doc.Meta.Command != "resolve" {
		t.Fatalf("meta.command = %q, want resolve", doc.Meta.Command)
	}
	if doc.Data["provider"] != "greenhouse" {
		t.Fatalf("data.provider = %v, want greenhouse", doc.Data["provider"])
	}
	if doc.Data["url"] != url {
		t.Fatalf("data.url = %v, want %s", doc.Data["url"], url)
	}
	if doc.Data["board_token"] != "acme" || doc.Data["provider_job_id"] != "4123456" {
		t.Fatalf("data = %#v", doc.Data)
	}
	capabilities, ok := doc.Data["capabilities"].(map[string]any)
	if !ok {
		t.Fatalf("data.capabilities = %#v", doc.Data["capabilities"])
	}
	for _, flag := range []string{"inspect", "prepare", "native_submit"} {
		if capabilities[flag] != true {
			t.Errorf("capabilities.%s = %v, want false", flag, capabilities[flag])
		}
	}
	if capabilities["browser_required"] != false {
		t.Errorf("capabilities.browser_required = %v, want false", capabilities["browser_required"])
	}
	for _, forbidden := range []string{"id", "title", "employer", "description", "source_job_id", "source_url", "application_url", "application", "diagnostics"} {
		if _, exists := doc.Data[forbidden]; exists {
			t.Errorf("resolve leaked Job field %q: %#v", forbidden, doc.Data)
		}
	}
}

func TestResolveJSONReportsBrowserRequiredProvider(t *testing.T) {
	env := []string{"JOBS_CONFIG_DIR=" + t.TempDir()}

	stdout, stderr, code := testutil.RunBinary(t, env, "--json", "resolve", "https://jobs.lever.co/palantir/6ed76ce8-4156-4b60-b120-403538bd66cd")
	if code != 0 {
		t.Fatalf("exit = %d (stderr: %s, stdout: %s)", code, stderr, stdout)
	}
	doc := decodeOne(t, stdout)
	if !doc.OK || doc.Data["provider"] != "lever" {
		t.Fatalf("envelope = %+v", doc)
	}
	capabilities, ok := doc.Data["capabilities"].(map[string]any)
	if !ok {
		t.Fatalf("data.capabilities = %#v", doc.Data["capabilities"])
	}
	if capabilities["browser_required"] != true || capabilities["native_submit"] != false {
		t.Fatalf("capabilities = %#v, want browser_required without native submit", capabilities)
	}
}
