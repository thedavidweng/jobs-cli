package greenhouse_test

import (
	"bytes"
	"context"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thedavidweng/jobs-cli/v2/internal/domain"
	joberrors "github.com/thedavidweng/jobs-cli/v2/internal/errors"
	"github.com/thedavidweng/jobs-cli/v2/internal/greenhouse"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func jsonResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func newClient(f roundTripFunc) *http.Client {
	return &http.Client{Transport: f}
}

const (
	boardToken = "acme"
	jobID      = "8556658002"
)

const jobSchema = `{
  "id": 8556658002,
  "title": "Backend Engineer",
  "absolute_url": "https://boards.greenhouse.io/acme/jobs/8556658002",
  "location": {"name": "Remote"},
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

func greenhouseTarget() domain.ApplicationTarget {
	return domain.ApplicationTarget{
		URL:      "https://boards.greenhouse.io/acme/jobs/8556658002",
		Provider: domain.ProviderGreenhouse,
	}
}

func requireCode(t *testing.T, err error, code string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error code %s, got nil", code)
	}
	got := joberrors.From(err)
	if got.Code != joberrors.Code(code) {
		t.Fatalf("error code = %s, want %s (%v)", got.Code, code, err)
	}
}

func TestInspectNormalizesQuestionSchema(t *testing.T) {
	var got *http.Request
	client := newClient(func(req *http.Request) (*http.Response, error) {
		got = req
		return jsonResponse(http.StatusOK, jobSchema), nil
	})
	provider := &greenhouse.Provider{Client: client}

	inspection, err := provider.Inspect(context.Background(), &domain.InspectRequest{Target: greenhouseTarget()})
	if err != nil {
		t.Fatalf("inspect: %v", err)
	}
	if got.Method != http.MethodGet {
		t.Fatalf("method = %s, want GET", got.Method)
	}
	if got.URL.Host != "boards-api.greenhouse.io" || got.URL.Path != "/v1/boards/acme/jobs/8556658002" {
		t.Fatalf("request URL = %s", got.URL.String())
	}
	if got.URL.Query().Get("questions") != "true" {
		t.Fatalf("questions query = %q, want true", got.URL.Query().Get("questions"))
	}
	if got.Header.Get("Accept") != "application/json" {
		t.Fatalf("accept header = %q", got.Header.Get("Accept"))
	}
	if inspection.Provider != domain.ProviderGreenhouse {
		t.Fatalf("provider = %s", inspection.Provider)
	}
	if !inspection.AcceptsResume || !inspection.AcceptsCoverLetter {
		t.Fatalf("accepts resume=%t cover=%t", inspection.AcceptsResume, inspection.AcceptsCoverLetter)
	}
	if !inspection.Capabilities.NativeSubmit || inspection.Capabilities.BrowserRequired {
		t.Fatalf("capabilities = %+v", inspection.Capabilities)
	}
	fields := map[string]domain.ApplicationField{}
	for _, field := range inspection.Fields {
		fields[field.Name] = field
	}
	for _, name := range []string{"first_name", "last_name", "email"} {
		field, ok := fields[name]
		if !ok || !field.Required || field.Type != "input_text" {
			t.Fatalf("field %s = %+v", name, field)
		}
	}
	if field, ok := fields["resume"]; !ok || !field.Required || field.Type != "file" {
		t.Fatalf("resume field = %+v (present=%t)", field, ok)
	}
	if len(inspection.Questions) != 2 {
		t.Fatalf("questions = %+v", inspection.Questions)
	}
	text := inspection.Questions[0]
	if text.ID != "question_36622854002" || text.Type != "textarea" || !text.Required || text.Label != "Why do you want this role?" {
		t.Fatalf("textarea question = %+v", text)
	}
	selectQuestion := inspection.Questions[1]
	if len(selectQuestion.Options) != 2 || selectQuestion.Options[0].Value != "job_board" || selectQuestion.Options[1].Label != "Referral" {
		t.Fatalf("select options = %+v", selectQuestion.Options)
	}
	if inspection.Application.BoardToken != boardToken || inspection.Application.ProviderJobID != jobID {
		t.Fatalf("application identifiers = %+v", inspection.Application)
	}
	wantFingerprint := domain.Fingerprint(&inspection.Application, inspection.Fields, inspection.Questions)
	if inspection.Fingerprint != wantFingerprint {
		t.Fatalf("fingerprint = %s, want %s", inspection.Fingerprint, wantFingerprint)
	}
}

func TestInspectDerivesIdentifiersFromEmbedURL(t *testing.T) {
	client := newClient(func(*http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, jobSchema), nil
	})
	provider := &greenhouse.Provider{Client: client}
	target := domain.ApplicationTarget{
		URL:      "https://acme.example.com/careers?gh_jid=8556658002&for=acme",
		Provider: domain.ProviderGreenhouse,
	}
	inspection, err := provider.Inspect(context.Background(), &domain.InspectRequest{Target: target})
	if err != nil {
		t.Fatalf("inspect: %v", err)
	}
	if inspection.Application.BoardToken != boardToken || inspection.Application.ProviderJobID != jobID {
		t.Fatalf("derived identifiers = %+v", inspection.Application)
	}
}

func TestInspectRequiresBoardIdentifiers(t *testing.T) {
	provider := &greenhouse.Provider{Client: newClient(func(*http.Request) (*http.Response, error) {
		t.Fatal("no request should be sent without board identifiers")
		return nil, nil
	})}
	target := domain.ApplicationTarget{URL: "https://example.com/jobs/1", Provider: domain.ProviderGreenhouse}
	_, err := provider.Inspect(context.Background(), &domain.InspectRequest{Target: target})
	requireCode(t, err, "ATS_RESOLUTION_FAILED")
}

func TestInspectSurfacesSchemaDrift(t *testing.T) {
	client := newClient(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"text/html"}},
			Body:       io.NopCloser(strings.NewReader("<html>blocked</html>")),
		}, nil
	})
	provider := &greenhouse.Provider{Client: client}
	_, err := provider.Inspect(context.Background(), &domain.InspectRequest{Target: greenhouseTarget()})
	requireCode(t, err, "API_SCHEMA_CHANGED")
}

func TestListBoardJobsPinsRequestAndParsesJobs(t *testing.T) {
	var got *http.Request
	client := newClient(func(req *http.Request) (*http.Response, error) {
		got = req
		return jsonResponse(http.StatusOK, `{"jobs":[{"id":8556658002,"title":"Backend Engineer","location":{"name":"Remote"},"updated_at":"2026-09-01T00:00:00-04:00","absolute_url":"https://boards.greenhouse.io/acme/jobs/8556658002"}]}`), nil
	})
	provider := &greenhouse.Provider{Client: client}
	jobs, err := provider.ListBoardJobs(context.Background(), boardToken)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if got.URL.String() != "https://boards-api.greenhouse.io/v1/boards/acme/jobs" {
		t.Fatalf("request URL = %s", got.URL.String())
	}
	if len(jobs) != 1 {
		t.Fatalf("jobs = %+v", jobs)
	}
	if jobs[0].ID != jobID || jobs[0].Title != "Backend Engineer" || jobs[0].Location != "Remote" {
		t.Fatalf("job = %+v", jobs[0])
	}
}

type capturedSubmission struct {
	requests int
	url      string
	content  []byte
	header   http.Header
}

func submitFixture(t *testing.T, schema, answerPath string) *capturedSubmission {
	t.Helper()
	dir := t.TempDir()
	resume := filepath.Join(dir, "ada-resume.pdf")
	cover := filepath.Join(dir, "cover.txt")
	if err := os.WriteFile(resume, []byte("resume-bytes"), 0o600); err != nil {
		t.Fatalf("write resume: %v", err)
	}
	if err := os.WriteFile(cover, []byte("cover-bytes"), 0o600); err != nil {
		t.Fatalf("write cover: %v", err)
	}
	captured := &capturedSubmission{}
	client := newClient(func(req *http.Request) (*http.Response, error) {
		captured.requests++
		if req.Method == http.MethodPost {
			captured.url = req.URL.String()
			captured.header = req.Header.Clone()
			data, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("read post body: %v", err)
			}
			captured.content = data
			return jsonResponse(http.StatusOK, `{"id":987654321,"success":true}`), nil
		}
		return jsonResponse(http.StatusOK, schema), nil
	})
	provider := &greenhouse.Provider{Client: client}
	prepared := domain.ApplicationArtifact{
		JobID:       "indeed:1",
		Provider:    domain.ProviderGreenhouse,
		Application: greenhouseTarget(),
		Candidate: domain.Candidate{
			FirstName: "Ada",
			LastName:  "Lovelace",
			Email:     "ada@example.com",
			Phone:     "+1 555 0100",
		},
		Answers: []domain.ApplicationAnswer{
			{QuestionID: "question_36622854002", Value: "Because I like building tools"},
			{QuestionID: "question_36622854003", Value: "referral"},
		},
		Attachments: []domain.Attachment{
			{Kind: "resume", Path: resume, Filename: "ada-resume.pdf", ContentType: "application/pdf"},
			{Kind: "cover_letter", Path: cover, Filename: "cover.txt", ContentType: "text/plain"},
		},
	}
	if answerPath != "" {
		prepared.Answers = append(prepared.Answers, domain.ApplicationAnswer{QuestionID: "question_36622854099", Value: answerPath})
	}
	result, err := provider.Submit(context.Background(), &domain.SubmitRequest{Target: greenhouseTarget(), Artifact: prepared})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if !result.Submitted || result.ApplicationID != "987654321" || result.Status != "submitted" {
		t.Fatalf("result = %+v", result)
	}
	return captured
}

type capturedPart struct {
	filename    string
	contentType string
	body        string
}

func parseParts(t *testing.T, header http.Header, body []byte) map[string]capturedPart {
	t.Helper()
	mediaType, params, err := mime.ParseMediaType(header.Get("Content-Type"))
	if err != nil {
		t.Fatalf("parse content type: %v", err)
	}
	if mediaType != "multipart/form-data" {
		t.Fatalf("content type = %s", mediaType)
	}
	if params["boundary"] == "" {
		t.Fatal("multipart boundary is missing")
	}
	reader := multipart.NewReader(bytes.NewReader(body), params["boundary"])
	parts := map[string]capturedPart{}
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
		parts[part.FormName()] = capturedPart{
			filename:    part.FileName(),
			contentType: part.Header.Get("Content-Type"),
			body:        string(data),
		}
	}
	return parts
}

func TestSubmitSendsSingleMultipartPost(t *testing.T) {
	captured := submitFixture(t, jobSchema, "")
	if captured.requests < 2 {
		t.Fatalf("requests = %d, want at least the schema GET and one POST", captured.requests)
	}
	if captured.url != "https://boards-api.greenhouse.io/v1/boards/acme/jobs/8556658002" {
		t.Fatalf("post URL = %s", captured.url)
	}
	parts := parseParts(t, captured.header, captured.content)
	expectField := map[string]string{
		"first_name":           "Ada",
		"last_name":            "Lovelace",
		"email":                "ada@example.com",
		"phone":                "+1 555 0100",
		"question_36622854002": "Because I like building tools",
		"question_36622854003": "referral",
	}
	for name, want := range expectField {
		part, ok := parts[name]
		if !ok {
			t.Fatalf("multipart part %q is missing (parts: %v)", name, partNames(parts))
		}
		if part.body != want {
			t.Fatalf("part %q = %q, want %q", name, part.body, want)
		}
	}
	resume, ok := parts["resume"]
	if !ok {
		t.Fatal("resume part is missing")
	}
	if resume.filename != "ada-resume.pdf" || resume.contentType != "application/pdf" {
		t.Fatalf("resume disposition = %q, content-type = %q", resume.filename, resume.contentType)
	}
	if resume.body != "resume-bytes" {
		t.Fatal("resume content mismatch")
	}
	cover, ok := parts["cover_letter"]
	if !ok {
		t.Fatal("cover letter part is missing")
	}
	if cover.filename != "cover.txt" || cover.contentType != "text/plain" {
		t.Fatalf("cover disposition = %q, content-type = %q", cover.filename, cover.contentType)
	}
	if cover.body != "cover-bytes" {
		t.Fatal("cover letter content mismatch")
	}
}

func partNames(parts map[string]capturedPart) []string {
	names := make([]string, 0, len(parts))
	for name := range parts {
		names = append(names, name)
	}
	return names
}

func TestSubmitAttachesCustomFileQuestion(t *testing.T) {
	dir := t.TempDir()
	portfolio := filepath.Join(dir, "portfolio.pdf")
	if err := os.WriteFile(portfolio, []byte("portfolio-bytes"), 0o600); err != nil {
		t.Fatalf("write portfolio: %v", err)
	}
	schema := strings.Replace(jobSchema,
		`{"label": "How did you hear about us?", "required": false, "fields": [{"name": "question_36622854003", "type": "multi_value_single_select", "values": [{"label": "Job board", "value": "job_board"}, {"label": "Referral", "value": "referral"}]}]}`,
		`{"label": "Portfolio", "required": true, "fields": [{"name": "question_36622854099", "type": "input_file", "values": []}]}`,
		1)
	if schema == jobSchema {
		t.Fatal("schema replacement failed")
	}
	captured := submitFixture(t, schema, portfolio)
	parts := parseParts(t, captured.header, captured.content)
	part, ok := parts["question_36622854099"]
	if !ok {
		t.Fatalf("file question part is missing (parts: %v)", partNames(parts))
	}
	if part.filename != "portfolio.pdf" || part.body != "portfolio-bytes" {
		t.Fatalf("file question = %q / %q", part.filename, part.body)
	}
}

func TestSubmitRemoteValidationFailure(t *testing.T) {
	dir := t.TempDir()
	resume := filepath.Join(dir, "resume.pdf")
	if err := os.WriteFile(resume, []byte("resume-bytes"), 0o600); err != nil {
		t.Fatalf("write resume: %v", err)
	}
	posts := 0
	client := newClient(func(req *http.Request) (*http.Response, error) {
		if req.Method == http.MethodPost {
			posts++
			return jsonResponse(http.StatusUnprocessableEntity, `{"errors":[{"message":"Email is invalid"}]}`), nil
		}
		return jsonResponse(http.StatusOK, jobSchema), nil
	})
	provider := &greenhouse.Provider{Client: client}
	prepared := domain.ApplicationArtifact{
		Provider:    domain.ProviderGreenhouse,
		Application: greenhouseTarget(),
		Candidate:   domain.Candidate{FirstName: "Ada", LastName: "Lovelace", Email: "ada@example.com"},
		Answers:     []domain.ApplicationAnswer{{QuestionID: "question_36622854002", Value: "Because"}},
		Attachments: []domain.Attachment{{Kind: "resume", Path: resume, Filename: "resume.pdf"}},
	}
	_, err := provider.Submit(context.Background(), &domain.SubmitRequest{Target: greenhouseTarget(), Artifact: prepared})
	requireCode(t, err, "APPLICATION_INCOMPLETE")
	if posts != 1 {
		t.Fatalf("posts = %d, want exactly 1", posts)
	}
	message := joberrors.From(err).Message
	if !strings.Contains(message, "Email is invalid") {
		t.Fatalf("message = %q", message)
	}
	if strings.Contains(message, "ada@example.com") {
		t.Fatalf("remote error message leaked candidate data: %q", message)
	}
}

func TestSubmitTransportFailureIsNotRetried(t *testing.T) {
	dir := t.TempDir()
	resume := filepath.Join(dir, "resume.pdf")
	if err := os.WriteFile(resume, []byte("resume-bytes"), 0o600); err != nil {
		t.Fatalf("write resume: %v", err)
	}
	posts := 0
	client := newClient(func(req *http.Request) (*http.Response, error) {
		if req.Method == http.MethodPost {
			posts++
			return nil, io.ErrUnexpectedEOF
		}
		return jsonResponse(http.StatusOK, jobSchema), nil
	})
	provider := &greenhouse.Provider{Client: client}
	prepared := domain.ApplicationArtifact{
		Provider:    domain.ProviderGreenhouse,
		Application: greenhouseTarget(),
		Candidate:   domain.Candidate{FirstName: "Ada", LastName: "Lovelace", Email: "ada@example.com"},
		Answers:     []domain.ApplicationAnswer{{QuestionID: "question_36622854002", Value: "Because"}},
		Attachments: []domain.Attachment{{Kind: "resume", Path: resume, Filename: "resume.pdf"}},
	}
	_, err := provider.Submit(context.Background(), &domain.SubmitRequest{Target: greenhouseTarget(), Artifact: prepared})
	requireCode(t, err, "NETWORK_UNREACHABLE")
	if posts != 1 {
		t.Fatalf("posts = %d, want exactly 1 (no retry)", posts)
	}
}

func TestSubmitRejectsUnacceptedCoverLetterBeforePost(t *testing.T) {
	dir := t.TempDir()
	resume := filepath.Join(dir, "resume.pdf")
	cover := filepath.Join(dir, "cover.txt")
	for _, path := range []string{resume, cover} {
		if err := os.WriteFile(path, []byte("bytes"), 0o600); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
	posts := 0
	client := newClient(func(req *http.Request) (*http.Response, error) {
		if req.Method == http.MethodPost {
			posts++
		}
		return jsonResponse(http.StatusOK, strings.Replace(jobSchema,
			`{"label": "Cover Letter", "required": false, "fields": [{"name": "cover_letter", "type": "input_file", "values": []}]},`,
			"", 1)), nil
	})
	provider := &greenhouse.Provider{Client: client}
	prepared := domain.ApplicationArtifact{
		Provider:    domain.ProviderGreenhouse,
		Application: greenhouseTarget(),
		Candidate:   domain.Candidate{FirstName: "Ada", LastName: "Lovelace", Email: "ada@example.com"},
		Answers:     []domain.ApplicationAnswer{{QuestionID: "question_36622854002", Value: "Because"}},
		Attachments: []domain.Attachment{
			{Kind: "resume", Path: resume, Filename: "resume.pdf"},
			{Kind: "cover_letter", Path: cover, Filename: "cover.txt"},
		},
	}
	_, err := provider.Submit(context.Background(), &domain.SubmitRequest{Target: greenhouseTarget(), Artifact: prepared})
	requireCode(t, err, "APPLICATION_INCOMPLETE")
	if posts != 0 {
		t.Fatalf("posts = %d, want 0", posts)
	}
}
