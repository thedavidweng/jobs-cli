package e2e

import (
	"bytes"
	"net/http"
	"strings"
	"testing"

	"github.com/thedavidweng/jobs-cli/internal/cli"
)

const voyagerDetailFixture = `{"data":{"detail":{"elements":[{"jobPostingDetailSection":[{"topCardV2":{"jobPostingCard":{"jobPostingTitle":"Engineer","primaryDescription":{"text":"Acme"},"tertiaryDescription":{"text":"Vancouver, BC · 2 days ago"}}}},{"jobDescription":{"jobPosting":{"description":{"text":"Build things"}}}}]}]}}}`

const voyagerEasyApplyAvailableFixture = `{"data":{"jobsDashOnsiteApplyApplicationByJobPosting":{"elements":[{"jobSeekerApplicationDetail":{"onsiteApply":true,"resume":{"name":"resume.pdf"}}}]}}}`

const voyagerEasyApplyUnavailableFixture = `{"data":{"jobsDashOnsiteApplyApplicationByJobPosting":{"elements":[{"jobSeekerApplicationDetail":{"onsiteApply":false}}]}}}`

type linkedinApplyTransport struct {
	t           *testing.T
	detailCalls int
	applyCalls  int
	applyBody   string
}

func (tr *linkedinApplyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	tr.t.Helper()
	switch req.URL.Query().Get("queryId") {
	case "voyagerJobsDashJobPostingDetailSections.8195171dc4c610f8c1551eaef6546bd8":
		tr.detailCalls++
		return voyagerJSONResponse(tr.t, voyagerDetailFixture), nil
	case "voyagerJobsDashOnsiteApplyApplication.34ac512c4fd87baec02c710aef4f563b":
		tr.applyCalls++
		return voyagerJSONResponse(tr.t, tr.applyBody), nil
	default:
		tr.t.Fatalf("unexpected Voyager query: %s", req.URL.RawQuery)
		return nil, nil
	}
}

func runApplyLinkedIn(t *testing.T, transport http.RoundTripper, args ...string) (doc envelope, code int) {
	t.Helper()
	dir := t.TempDir()
	writeLinkedInSession(t, dir)
	var stdout, stderr bytes.Buffer
	app := cli.New(&cli.Options{
		Stdout:        &stdout,
		Stderr:        &stderr,
		ConfigDir:     dir,
		BaseTransport: transport,
	})
	code = app.Run(args)
	if lines := strings.Split(strings.TrimSpace(stdout.String()), "\n"); len(lines) != 1 {
		t.Fatalf("stdout must be exactly one JSON document, got %d lines:\n%s", len(lines), stdout.String())
	}
	return decodeOne(t, stdout.String()), code
}

func TestApplyInspectLinkedInNonEasyApplyIsBrowserRequired(t *testing.T) {
	transport := &linkedinApplyTransport{t: t, applyBody: voyagerEasyApplyUnavailableFixture}
	doc, code := runApplyLinkedIn(t, transport, "--json", "apply", "inspect", "linkedin:42", "--authenticated")
	if code != 6 {
		t.Fatalf("exit = %d, want 6 (doc: %+v)", code, doc)
	}
	if doc.Error == nil || doc.Error.Code != "BROWSER_REQUIRED" {
		t.Fatalf("envelope = %+v, want BROWSER_REQUIRED", doc)
	}
	if !strings.Contains(doc.Error.Message, "https://www.linkedin.com/jobs/view/42") {
		t.Fatalf("error message does not return the LinkedIn job URL: %q", doc.Error.Message)
	}
	if transport.detailCalls != 1 || transport.applyCalls != 1 {
		t.Fatalf("calls = detail:%d apply:%d", transport.detailCalls, transport.applyCalls)
	}
}

func TestApplyPrepareLinkedInNonEasyApplyNeverBuildsAnArtifact(t *testing.T) {
	transport := &linkedinApplyTransport{t: t, applyBody: voyagerEasyApplyUnavailableFixture}
	doc, code := runApplyLinkedIn(t, transport, "--json", "apply", "prepare", "linkedin:42", "--authenticated", "--manifest", "-")
	if code != 6 {
		t.Fatalf("exit = %d, want 6 (doc: %+v)", code, doc)
	}
	if doc.Error == nil || doc.Error.Code != "BROWSER_REQUIRED" {
		t.Fatalf("envelope = %+v, want BROWSER_REQUIRED instead of a LinkedIn application artifact", doc)
	}
	if doc.OK {
		t.Fatalf("prepare must not produce an artifact for a non-Easy-Apply job: %+v", doc)
	}
}

func TestApplyInspectLinkedInEasyApplyStillInspects(t *testing.T) {
	transport := &linkedinApplyTransport{t: t, applyBody: voyagerEasyApplyAvailableFixture}
	doc, code := runApplyLinkedIn(t, transport, "--json", "apply", "inspect", "linkedin:42", "--authenticated")
	if code != 0 {
		t.Fatalf("exit = %d (doc: %+v)", code, doc)
	}
	if !doc.OK {
		t.Fatalf("envelope = %+v, want a native Easy Apply inspection", doc)
	}
	provider, _ := doc.Data["provider"].(string)
	if provider != "linkedin" {
		t.Fatalf("provider = %q, want linkedin", provider)
	}
}
