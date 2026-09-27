package cli_test

import (
	"bytes"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"

	"github.com/thedavidweng/jobs-cli/v2/internal/config"
	"github.com/thedavidweng/jobs-cli/v2/internal/domain"
	"github.com/thedavidweng/jobs-cli/v2/internal/greenhouse"
	"github.com/thedavidweng/jobs-cli/v2/internal/registry"
	"github.com/thedavidweng/jobs-cli/v2/internal/testutil"
)

const ghSchema = `{
  "id": 8556658002,
  "title": "Backend Engineer",
  "absolute_url": "https://boards.greenhouse.io/acme/jobs/8556658002",
  "questions": [
    {"label": "First Name", "required": true, "fields": [{"name": "first_name", "type": "input_text", "values": []}]},
    {"label": "Last Name", "required": true, "fields": [{"name": "last_name", "type": "input_text", "values": []}]},
    {"label": "Email", "required": true, "fields": [{"name": "email", "type": "input_text", "values": []}]},
    {"label": "Phone", "required": false, "fields": [{"name": "phone", "type": "input_text", "values": []}]},
    {"label": "Resume", "required": true, "fields": [{"name": "resume", "type": "input_file", "values": []}]},
    {"label": "Cover Letter", "required": false, "fields": [{"name": "cover_letter", "type": "input_file", "values": []}]},
    {"label": "Why do you want this role?", "required": true, "fields": [{"name": "question_36622854002", "type": "textarea", "values": []}]},
    {"label": "How did you hear about us?", "required": false, "fields": [{"name": "question_36622854003", "type": "multi_value_single_select", "values": [{"label": "Job board", "value": "job_board"}, {"label": "Referral", "value": "referral"}]}]}
  ]
}`

type ghStub struct {
	schema     string
	submitCode int
	submitBody string
	gets       int
	posts      int
	postURL    string
	postType   string
	postBody   []byte
}

func (s *ghStub) handle(req *http.Request) (*http.Response, error) {
	switch req.Method {
	case http.MethodGet:
		s.gets++
		return testutil.JSONResponse(http.StatusOK, s.schema), nil
	case http.MethodPost:
		s.posts++
		s.postURL = req.URL.String()
		s.postType = req.Header.Get("Content-Type")
		s.postBody, _ = io.ReadAll(req.Body)
		code := s.submitCode
		if code == 0 {
			code = http.StatusOK
		}
		body := s.submitBody
		if body == "" {
			body = `{"id":987654321,"success":true}`
		}
		return testutil.JSONResponse(code, body), nil
	default:
		return testutil.JSONResponse(http.StatusMethodNotAllowed, `{}`), nil
	}
}

func ghTarget() *domain.ApplicationTarget {
	return &domain.ApplicationTarget{
		URL:           "https://boards.greenhouse.io/acme/jobs/8556658002",
		Provider:      domain.ProviderGreenhouse,
		BoardToken:    "acme",
		ProviderJobID: "8556658002",
		Capabilities:  domain.Capabilities{Inspect: true, Prepare: true, NativeSubmit: true},
		Verification:  domain.VerifiedWorking,
	}
}

func useGreenhouseStub(h *harness, stub *ghStub) *harness {
	h.factory = func(*config.Config, *http.Client) *registry.Registry {
		client := &http.Client{Transport: testutil.RoundTripFunc(stub.handle)}
		return testutil.NewRegistry(
			map[domain.Source]domain.SourceAdapter{domain.SourceIndeed: &testutil.FakeSource{SourceName: domain.SourceIndeed, Job: fakeJob(domain.SourceIndeed, "1")}},
			&testutil.FakeResolver{Target: ghTarget()},
			map[domain.ApplicationProvider]domain.ApplyProvider{domain.ProviderGreenhouse: greenhouse.NewProvider(client)},
		)
	}
	return h
}

func ghManifest(t *testing.T, h *harness) string {
	t.Helper()
	resume := h.writeFile("resume.pdf", "resume-bytes")
	cover := h.writeFile("cover.txt", "cover-bytes")
	return h.writeFile("manifest.json", fmt.Sprintf(
		`{"candidate":{"first_name":"Ada","last_name":"Lovelace","email":"ada@example.com","phone":"+1 555 0100"},"answers":[{"question_id":"question_36622854002","value":"Because I like building tools"},{"question_id":"question_36622854003","value":"referral"}],"resume":%q,"cover_letter":%q}`,
		resume, cover,
	))
}

type ghPart struct {
	filename    string
	contentType string
	body        string
}

func ghParts(t *testing.T, contentType string, body []byte) map[string]ghPart {
	t.Helper()
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		t.Fatalf("parse content type: %v", err)
	}
	if mediaType != "multipart/form-data" || params["boundary"] == "" {
		t.Fatalf("content type = %q", contentType)
	}
	reader := multipart.NewReader(bytes.NewReader(body), params["boundary"])
	parts := map[string]ghPart{}
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("read part: %v", err)
		}
		data, err := io.ReadAll(part)
		if err != nil {
			t.Fatalf("read part body: %v", err)
		}
		parts[part.FormName()] = ghPart{filename: part.FileName(), contentType: part.Header.Get("Content-Type"), body: string(data)}
	}
	return parts
}

func ghPrepare(t *testing.T, h *harness) string {
	t.Helper()
	artifactPath := h.dir + "/artifact.json"
	out, errOut, code := h.run("--json", "apply", "prepare", "indeed:1", "--manifest", ghManifest(t, h), "--out", artifactPath)
	if code != 0 {
		t.Fatalf("prepare exit = %d (stderr: %s, stdout: %s)", code, errOut, out)
	}
	if decodeEnvelope(t, out).Data["path"] != artifactPath {
		t.Fatalf("prepare did not report the artifact path: %s", out)
	}
	return artifactPath
}

func TestGreenhouseApplyJourneyEndToEnd(t *testing.T) {
	stub := &ghStub{schema: ghSchema}
	h := useGreenhouseStub(newHarness(t), stub)

	inspectOut, _, code := h.run("--json", "apply", "inspect", "indeed:1")
	if code != 0 {
		t.Fatalf("inspect exit = %d", code)
	}
	inspectDoc := decodeEnvelope(t, inspectOut)
	if inspectDoc.Data["fingerprint"] == "" {
		t.Fatal("inspect did not return a fingerprint")
	}
	questions, ok := inspectDoc.Data["questions"].([]any)
	if !ok || len(questions) != 2 {
		t.Fatalf("inspect questions = %#v", inspectDoc.Data["questions"])
	}
	first, ok := questions[0].(map[string]any)
	if !ok || first["id"] != "question_36622854002" || first["type"] != "textarea" || first["required"] != true {
		t.Fatalf("normalized question = %#v", questions[0])
	}

	artifactPath := ghPrepare(t, h)

	dryOut, _, code := h.run("--json", "apply", "submit", "--artifact", artifactPath, "--dry-run")
	if code != 0 {
		t.Fatalf("dry-run exit = %d", code)
	}
	dryDoc := decodeEnvelope(t, dryOut)
	if dryDoc.Data["dry_run"] != true {
		t.Fatalf("dry_run = %v", dryDoc.Data["dry_run"])
	}
	if mutations, ok := dryDoc.Data["planned_mutations"].([]any); !ok || len(mutations) != 1 {
		t.Fatalf("planned_mutations = %#v", dryDoc.Data["planned_mutations"])
	}
	if stub.posts != 0 {
		t.Fatalf("dry-run posted %d times", stub.posts)
	}

	submitOut, errOut, code := h.run("--json", "apply", "submit", "--artifact", artifactPath, "--confirm")
	if code != 0 {
		t.Fatalf("submit exit = %d (stderr: %s, stdout: %s)", code, errOut, submitOut)
	}
	if stub.posts != 1 {
		t.Fatalf("posts = %d, want exactly 1", stub.posts)
	}
	submitDoc := decodeEnvelope(t, submitOut)
	if submitDoc.Data["submitted"] != true {
		t.Fatalf("submission result = %s", submitOut)
	}
	if submitDoc.Data["application_id"] != "987654321" {
		t.Fatalf("application_id = %v", submitDoc.Data["application_id"])
	}
	if stub.postURL != "https://boards-api.greenhouse.io/v1/boards/acme/jobs/8556658002" {
		t.Fatalf("post URL = %s", stub.postURL)
	}

	parts := ghParts(t, stub.postType, stub.postBody)
	for name, want := range map[string]string{
		"first_name":           "Ada",
		"last_name":            "Lovelace",
		"email":                "ada@example.com",
		"phone":                "+1 555 0100",
		"question_36622854002": "Because I like building tools",
		"question_36622854003": "referral",
	} {
		part, ok := parts[name]
		if !ok || part.body != want {
			t.Fatalf("part %q = %+v, want %q", name, part, want)
		}
	}
	if part := parts["resume"]; part.filename != "resume.pdf" || part.contentType != "application/pdf" || part.body != "resume-bytes" {
		t.Fatalf("resume part = %+v", part)
	}
	if part := parts["cover_letter"]; part.filename != "cover.txt" || part.contentType != "text/plain" || part.body != "cover-bytes" {
		t.Fatalf("cover part = %+v", part)
	}
}

func TestGreenhouseApplyArtifactStdinRoundTrip(t *testing.T) {
	stub := &ghStub{schema: ghSchema}
	h := useGreenhouseStub(newHarness(t), stub)
	prepareOut, errOut, code := h.run("--json", "apply", "prepare", "indeed:1", "--manifest", ghManifest(t, h))
	if code != 0 {
		t.Fatalf("prepare exit = %d (stderr: %s)", code, errOut)
	}
	h.withStdin(prepareOut)
	submitOut, _, code := h.run("--json", "apply", "submit", "--artifact", "-", "--confirm")
	if code != 0 {
		t.Fatalf("stdin submit exit = %d", code)
	}
	if stub.posts != 1 {
		t.Fatalf("posts = %d, want exactly 1", stub.posts)
	}
	if decodeEnvelope(t, submitOut).Data["submitted"] != true {
		t.Fatalf("submission result = %s", submitOut)
	}
}

func TestGreenhouseApplySubmitGates(t *testing.T) {
	t.Run("missing_confirm", func(t *testing.T) {
		stub := &ghStub{schema: ghSchema}
		h := useGreenhouseStub(newHarness(t), stub)
		artifactPath := ghPrepare(t, h)
		out, _, code := h.run("--json", "apply", "submit", "--artifact", artifactPath)
		doc := decodeEnvelope(t, out)
		requireCode(t, &doc, "CONFIRMATION_REQUIRED", 10, code)
		if stub.posts != 0 {
			t.Fatalf("posts = %d, want 0 without --confirm", stub.posts)
		}
	})

	t.Run("read_only", func(t *testing.T) {
		stub := &ghStub{schema: ghSchema}
		h := useGreenhouseStub(newHarness(t), stub)
		artifactPath := ghPrepare(t, h)
		out, _, code := h.run("--json", "--read-only", "apply", "submit", "--artifact", artifactPath, "--confirm")
		doc := decodeEnvelope(t, out)
		requireCode(t, &doc, "READ_ONLY_VIOLATION", 4, code)
		if stub.posts != 0 {
			t.Fatalf("posts = %d, want 0 in read-only mode", stub.posts)
		}
	})
}

func TestGreenhouseApplyStaleArtifact(t *testing.T) {
	stub := &ghStub{schema: ghSchema}
	h := useGreenhouseStub(newHarness(t), stub)
	artifactPath := ghPrepare(t, h)
	stub.schema = replaceOnce(t, ghSchema, "Why do you want this role?", "Tell us about yourself")
	out, _, code := h.run("--json", "apply", "submit", "--artifact", artifactPath, "--confirm")
	doc := decodeEnvelope(t, out)
	requireCode(t, &doc, "ARTIFACT_STALE", 7, code)
	if stub.posts != 0 {
		t.Fatalf("posts = %d, want 0 for a stale artifact", stub.posts)
	}
}

func TestGreenhouseApplyValidationBeforeSubmit(t *testing.T) {
	t.Run("missing_required_answer", func(t *testing.T) {
		stub := &ghStub{schema: ghSchema}
		h := useGreenhouseStub(newHarness(t), stub)
		manifest := h.writeFile("missing.json", fmt.Sprintf(
			`{"candidate":{"first_name":"Ada","last_name":"Lovelace","email":"ada@example.com"},"resume":%q}`,
			h.writeFile("resume.pdf", "resume-bytes"),
		))
		out, _, code := h.run("--json", "apply", "prepare", "indeed:1", "--manifest", manifest, "--out", h.dir+"/artifact.json")
		doc := decodeEnvelope(t, out)
		requireCode(t, &doc, "APPLICATION_INCOMPLETE", 7, code)
		if stub.posts != 0 {
			t.Fatalf("posts = %d, want 0", stub.posts)
		}
	})

	t.Run("invalid_option", func(t *testing.T) {
		stub := &ghStub{schema: ghSchema}
		h := useGreenhouseStub(newHarness(t), stub)
		manifest := h.writeFile("badoption.json", fmt.Sprintf(
			`{"candidate":{"first_name":"Ada","last_name":"Lovelace","email":"ada@example.com"},"answers":[{"question_id":"question_36622854002","value":"Because"},{"question_id":"question_36622854003","value":"billboard"}],"resume":%q}`,
			h.writeFile("resume.pdf", "resume-bytes"),
		))
		out, _, code := h.run("--json", "apply", "prepare", "indeed:1", "--manifest", manifest, "--out", h.dir+"/artifact.json")
		doc := decodeEnvelope(t, out)
		requireCode(t, &doc, "APPLICATION_INCOMPLETE", 7, code)
		if stub.posts != 0 {
			t.Fatalf("posts = %d, want 0", stub.posts)
		}
	})
}

func TestGreenhouseApplyRemoteValidationFailure(t *testing.T) {
	stub := &ghStub{schema: ghSchema, submitCode: http.StatusUnprocessableEntity, submitBody: `{"errors":[{"message":"Email is invalid"}]}`}
	h := useGreenhouseStub(newHarness(t), stub)
	artifactPath := ghPrepare(t, h)
	out, _, code := h.run("--json", "apply", "submit", "--artifact", artifactPath, "--confirm")
	doc := decodeEnvelope(t, out)
	requireCode(t, &doc, "APPLICATION_INCOMPLETE", 7, code)
	if stub.posts != 1 {
		t.Fatalf("posts = %d, want exactly 1", stub.posts)
	}
}

func TestGreenhouseApplyPrepareAcceptsAnswersFile(t *testing.T) {
	stub := &ghStub{schema: ghSchema}
	h := useGreenhouseStub(newHarness(t), stub)
	resume := h.writeFile("resume.pdf", "resume-bytes")
	manifest := h.writeFile("candidate.json", fmt.Sprintf(
		`{"candidate":{"first_name":"Ada","last_name":"Lovelace","email":"ada@example.com"},"resume":%q}`, resume,
	))
	answers := h.writeFile("answers.json", `{"question_36622854002":"Because I like tools","question_36622854003":"referral"}`)
	out, errOut, code := h.run("--json", "apply", "prepare", "indeed:1", "--manifest", manifest, "--answers", answers, "--out", h.dir+"/artifact.json")
	if code != 0 {
		t.Fatalf("prepare exit = %d (stderr: %s, stdout: %s)", code, errOut, out)
	}
	doc := decodeEnvelope(t, out)
	prepared, ok := doc.Data["artifact"].(map[string]any)
	if !ok {
		t.Fatalf("artifact = %#v", doc.Data["artifact"])
	}
	rawAnswers, ok := prepared["answers"].([]any)
	if !ok || len(rawAnswers) != 2 {
		t.Fatalf("answers = %#v", prepared["answers"])
	}
}

func replaceOnce(t *testing.T, haystack, old, replacement string) string {
	t.Helper()
	if !strings.Contains(haystack, old) {
		t.Fatalf("substring %q not found", old)
	}
	return strings.Replace(haystack, old, replacement, 1)
}
