package e2e

import (
	"bytes"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thedavidweng/jobs-cli/v2/internal/cli"
	"github.com/thedavidweng/jobs-cli/v2/internal/testutil"
)

const linkedInGuestCards = `<ul class="jobs-search__results-list">
  <li>
    <div class="base-card base-search-card base-search-card--link job-search-card" data-entity-urn="urn:li:jobPosting:3891234567" data-reference-id="9aZq1mKeQ3yWtB7l">
      <a class="base-card__full-link absolute top-0 right-0 bottom-0 left-0 p-0" href="https://www.linkedin.com/jobs/view/software-engineer-at-acme-3891234567?position=1&amp;pageNum=0&amp;refId=9aZq1mKeQ3yWtB7l">
        <span class="sr-only">Software Engineer</span>
      </a>
      <div class="base-search-card__info">
        <h3 class="base-search-card__title">Software Engineer</h3>
        <h4 class="base-search-card__subtitle">
          <a class="hidden-nested-link" href="https://www.linkedin.com/company/acme">Acme</a>
        </h4>
        <div class="base-search-card__metadata">
          <span class="job-search-card__location">Vancouver, British Columbia, Canada</span>
          <time class="job-search-card__listdate" datetime="2026-05-01">3 days ago</time>
        </div>
      </div>
    </div>
  </li>
  <li>
    <div class="base-card base-search-card base-search-card--link job-search-card" data-entity-urn="urn:li:jobPosting:3892233445" data-reference-id="Pq7Lm2RtV8xNc4dZ">
      <a class="base-card__full-link absolute top-0 right-0 bottom-0 left-0 p-0" href="https://www.linkedin.com/jobs/view/senior-platform-engineer-remote-at-globex-3892233445?position=2&amp;pageNum=0">
        <span class="sr-only">Senior Platform Engineer (Remote)</span>
      </a>
      <div class="base-search-card__info">
        <h3 class="base-search-card__title">Senior Platform Engineer (Remote)</h3>
        <h4 class="base-search-card__subtitle">
          <a class="hidden-nested-link" href="https://www.linkedin.com/company/globex">Globex</a>
        </h4>
        <div class="base-search-card__metadata">
          <span class="job-search-card__location">Toronto, Ontario, Canada</span>
          <time class="job-search-card__listdate--new" datetime="2026-05-04">1 day ago</time>
        </div>
      </div>
    </div>
  </li>
  <li>
    <div class="base-card base-search-card base-search-card--link job-search-card" data-entity-urn="urn:li:jobPosting:3900000001">
      <a class="base-card__full-link absolute top-0 right-0 bottom-0 left-0 p-0" href="https://www.linkedin.com/jobs/view/data-analyst-at-initech-3900000001?position=3&amp;pageNum=0">
        <span class="sr-only">Data Analyst</span>
      </a>
      <div class="base-search-card__info">
        <h3 class="base-search-card__title">Data Analyst</h3>
        <h4 class="base-search-card__subtitle">
          <a class="hidden-nested-link" href="https://www.linkedin.com/company/initech">Initech</a>
        </h4>
        <div class="base-search-card__metadata">
          <span class="job-search-card__location">Remote</span>
          <time class="job-search-card__listdate" datetime="2026-04-28">1 week ago</time>
        </div>
      </div>
    </div>
  </li>
</ul>`

const linkedInLoginPage = `<!DOCTYPE html>
<html lang="en">
  <head><title>Sign In | LinkedIn</title></head>
  <body>
    <div class="sign-in-card">
      <h1>Sign in</h1>
      <form class="login__form" action="/uas/login-submit" method="post"></form>
    </div>
  </body>
</html>`

type guestRun struct {
	doc    envelope
	stdout string
	stderr string
}

func runGuestSearch(t *testing.T, dir string, transport http.RoundTripper, args ...string) guestRun {
	t.Helper()
	var stdout, stderr bytes.Buffer
	app := cli.New(&cli.Options{
		Stdout:        &stdout,
		Stderr:        &stderr,
		Stdin:         strings.NewReader(""),
		ConfigDir:     dir,
		BaseTransport: transport,
	})
	code := app.Run(args)
	if code != 0 {
		t.Fatalf("exit = %d, want 0 (stderr: %s stdout: %s)", code, stderr.String(), stdout.String())
	}
	return guestRun{doc: decodeOne(t, stdout.String()), stdout: stdout.String(), stderr: stderr.String()}
}

func TestSearchSourceLinkedInDefaultsToGuest(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("profiles:\n  default:\n    sources:\n      - linkedin\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	sessionFile := filepath.Join(dir, "sessions", "default.json")
	if err := os.MkdirAll(sessionFile, 0o700); err != nil {
		t.Fatalf("create session tripwire: %v", err)
	}

	guestRequests := 0
	transport := testutil.RoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Host != "www.linkedin.com" || req.URL.Path != "/jobs-guest/jobs/api/seeMoreJobPostings/search" {
			t.Errorf("unexpected request: %s %s", req.Method, req.URL)
			return testutil.HTMLResponse(http.StatusNotFound, ""), nil
		}
		guestRequests++
		if cookie := req.Header.Get("Cookie"); cookie != "" {
			t.Errorf("guest request carried a cookie: %q", cookie)
		}
		if csrf := req.Header.Get("csrf-token"); csrf != "" {
			t.Errorf("guest request carried a csrf token: %q", csrf)
		}
		if start := req.URL.Query().Get("start"); start != "0" {
			t.Errorf("guest start = %q, want 0", start)
		}
		if keywords := req.URL.Query().Get("keywords"); keywords != "golang" {
			t.Errorf("guest keywords = %q, want golang", keywords)
		}
		return testutil.HTMLResponse(http.StatusOK, linkedInGuestCards), nil
	})

	run := runGuestSearch(t, dir, transport, "--json", "search", "-q", "golang", "-l", "Vancouver, BC", "--source", "linkedin")

	if guestRequests != 1 {
		t.Fatalf("guest requests = %d, want 1", guestRequests)
	}
	if !run.doc.OK {
		t.Fatalf("envelope = %+v", run.doc)
	}
	if _, err := os.Stat(sessionFile); err != nil {
		t.Errorf("session tripwire was removed: %v", err)
	}
	if strings.Contains(strings.ToLower(run.stderr), "session") {
		t.Errorf("guest search touched the session surface: %s", run.stderr)
	}
	if strings.Contains(run.stdout, "voyager") {
		t.Errorf("guest search leaked a voyager request into output: %s", run.stdout)
	}

	partitions, ok := run.doc.Data["partitions"].([]any)
	if !ok || len(partitions) != 1 {
		t.Fatalf("data.partitions = %#v", run.doc.Data["partitions"])
	}
	partition, ok := partitions[0].(map[string]any)
	if !ok {
		t.Fatalf("partitions[0] = %#v", partitions[0])
	}
	if partition["source"] != "linkedin" {
		t.Fatalf("partition source = %v", partition["source"])
	}
	jobs, ok := partition["jobs"].([]any)
	if !ok || len(jobs) != 3 {
		t.Fatalf("partition jobs = %#v", partition["jobs"])
	}
	first, ok := jobs[0].(map[string]any)
	if !ok {
		t.Fatalf("jobs[0] = %#v", jobs[0])
	}
	for field, want := range map[string]any{
		"id":            "linkedin:3891234567",
		"source":        "linkedin",
		"source_job_id": "3891234567",
		"title":         "Software Engineer",
		"employer":      "Acme",
		"location":      "Vancouver, British Columbia, Canada",
		"posted_date":   "2026-05-01",
		"source_url":    "https://www.linkedin.com/jobs/view/3891234567",
		"remote":        false,
	} {
		if got := first[field]; got != want {
			t.Errorf("jobs[0].%s = %#v, want %#v", field, got, want)
		}
	}

	pagination, ok := partition["pagination"].(map[string]any)
	if !ok {
		t.Fatalf("partition pagination = %#v", partition["pagination"])
	}
	if pagination["has_more"] != true {
		t.Errorf("pagination.has_more = %#v, want true", pagination["has_more"])
	}
	if pagination["next_cursor"] != "3" {
		t.Errorf("pagination.next_cursor = %#v, want 3", pagination["next_cursor"])
	}
	native, ok := pagination["native"].(map[string]any)
	if !ok {
		t.Fatalf("pagination.native = %#v", pagination["native"])
	}
	if native["kind"] != "start_count" {
		t.Errorf("native.kind = %#v, want start_count", native["kind"])
	}
	if native["count"] != float64(3) {
		t.Errorf("native.count = %#v, want 3", native["count"])
	}
}

func TestSearchSourceLinkedInReportsSchemaChange(t *testing.T) {
	dir := t.TempDir()
	transport := testutil.RoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/jobs-guest/jobs/api/seeMoreJobPostings/search" {
			t.Errorf("unexpected request: %s", req.URL)
		}
		return testutil.HTMLResponse(http.StatusOK, linkedInLoginPage), nil
	})
	var stdout, stderr bytes.Buffer
	app := cli.New(&cli.Options{
		Stdout:        &stdout,
		Stderr:        &stderr,
		Stdin:         strings.NewReader(""),
		ConfigDir:     dir,
		BaseTransport: transport,
	})
	if code := app.Run([]string{"--json", "search", "-q", "golang", "--source", "linkedin"}); code != 6 {
		t.Fatalf("exit = %d, want 6 (stdout: %s stderr: %s)", code, stdout.String(), stderr.String())
	}
	doc := decodeOne(t, stdout.String())
	if doc.OK || doc.Error == nil || doc.Error.Code != "API_SCHEMA_CHANGED" {
		t.Fatalf("envelope = %+v", doc)
	}
}
