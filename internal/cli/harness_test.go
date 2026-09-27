package cli_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thedavidweng/jobs-cli/internal/cli"
	"github.com/thedavidweng/jobs-cli/internal/config"
	"github.com/thedavidweng/jobs-cli/internal/domain"
	"github.com/thedavidweng/jobs-cli/internal/registry"
)

type harness struct {
	t       *testing.T
	dir     string
	stdin   string
	factory func(cfg *config.Config, client *http.Client) *registry.Registry
	base    http.RoundTripper
	out     bytes.Buffer
	errOut  bytes.Buffer
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	return &harness{t: t, dir: t.TempDir()}
}

func (h *harness) withTransport(rt http.RoundTripper) *harness {
	h.base = rt
	return h
}

func (h *harness) useRealRegistry() *harness {
	h.factory = func(cfg *config.Config, client *http.Client) *registry.Registry {
		return registry.New(client, cfg)
	}
	return h
}

func (h *harness) useRegistry(reg *registry.Registry) *harness {
	h.factory = func(*config.Config, *http.Client) *registry.Registry { return reg }
	return h
}

func (h *harness) withStdin(s string) {
	h.stdin = s
}

func (h *harness) run(args ...string) (stdout, stderr string, code int) {
	h.t.Helper()
	h.out.Reset()
	h.errOut.Reset()
	factory := h.factory
	if factory == nil {
		factory = func(cfg *config.Config, client *http.Client) *registry.Registry {
			return registry.New(client, cfg)
		}
	}
	app := cli.New(&cli.Options{
		Stdout:          &h.out,
		Stderr:          &h.errOut,
		Stdin:           strings.NewReader(h.stdin),
		ConfigDir:       h.dir,
		BaseTransport:   h.base,
		RegistryFactory: factory,
	})
	code = app.Run(args)
	return h.out.String(), h.errOut.String(), code
}

func (h *harness) writeFile(name, content string) string {
	h.t.Helper()
	path := filepath.Join(h.dir, name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		h.t.Fatalf("write %s: %v", path, err)
	}
	return path
}

func (h *harness) writeConfig(content string) {
	h.t.Helper()
	path := filepath.Join(h.dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		h.t.Fatalf("write config: %v", err)
	}
}

type envelopeDoc struct {
	OK    bool           `json:"ok"`
	Data  map[string]any `json:"data"`
	Error *errorDoc      `json:"error"`
	Meta  metaDoc        `json:"meta"`
}

type errorDoc struct {
	Code         string `json:"code"`
	Message      string `json:"message"`
	Category     string `json:"category"`
	Retryable    bool   `json:"retryable"`
	RetryAfterMS int64  `json:"retry_after_ms"`
}

type metaDoc struct {
	Command       string         `json:"command"`
	Profile       string         `json:"profile"`
	DurationMS    int64          `json:"duration_ms"`
	SchemaVersion string         `json:"schema_version"`
	RequestID     string         `json:"request_id"`
	Warnings      []string       `json:"warnings"`
	Pagination    map[string]any `json:"pagination"`
	Partitions    []partitionDoc `json:"partitions"`
}

type partitionDoc struct {
	Source     string         `json:"source"`
	OK         bool           `json:"ok"`
	Pagination map[string]any `json:"pagination"`
}

func decodeEnvelope(t *testing.T, out string) envelopeDoc {
	t.Helper()
	if strings.TrimSpace(out) == "" {
		t.Fatal("stdout is empty; expected one JSON document")
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 1 {
		t.Fatalf("stdout must be exactly one JSON document, got %d lines:\n%s", len(lines), out)
	}
	var doc envelopeDoc
	decoder := json.NewDecoder(strings.NewReader(out))
	if err := decoder.Decode(&doc); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\n%s", err, out)
	}
	return doc
}

func requireCode(t *testing.T, doc *envelopeDoc, wantCode string, wantExit, gotExit int) {
	t.Helper()
	if gotExit != wantExit {
		t.Fatalf("exit code = %d, want %d (error: %+v)", gotExit, wantExit, doc.Error)
	}
	if doc.OK {
		t.Fatalf("envelope ok=true, want ok=false")
	}
	if doc.Error == nil {
		t.Fatalf("error envelope has no error object")
	}
	if doc.Error.Code != wantCode {
		t.Fatalf("error.code = %q, want %q", doc.Error.Code, wantCode)
	}
}

func fakeJob(source domain.Source, sourceJobID string) *domain.Job {
	job := domain.NewJob(source, sourceJobID)
	job.Title = "Backend Engineer"
	job.Employer = "Acme"
	job.Location = "Vancouver, BC"
	job.Workplace = domain.WorkplaceRemote
	job.Remote = true
	job.SourceURL = "https://www.indeed.com/viewjob?jk=" + sourceJobID
	job.ApplicationURL = "https://boards.greenhouse.io/acme/jobs/1"
	job.Description = strings.Repeat("long description ", 40)
	job.Diagnostics = &domain.Diagnostics{SourcePayload: json.RawMessage(`{"raw":true}`)}
	return &job
}

func greenhouseTarget() *domain.ApplicationTarget {
	return &domain.ApplicationTarget{
		URL:      "https://boards.greenhouse.io/acme/jobs/1",
		Provider: domain.ProviderGreenhouse,
		Capabilities: domain.Capabilities{
			Inspect: true, Prepare: true, NativeSubmit: true,
		},
		Verification: domain.VerifiedWorking,
	}
}

func inspectionWithQuestions(target *domain.ApplicationTarget, provider domain.ApplicationProvider) *domain.ApplicationInspection {
	inspection := &domain.ApplicationInspection{
		Provider:      provider,
		Application:   *target,
		AcceptsResume: true,
		Fields: []domain.ApplicationField{
			{Name: "email", Label: "Email", Type: "email", Required: true},
		},
		Questions: []domain.ApplicationQuestion{
			{ID: "q1", Label: "Why do you want this role?", Type: "textarea", Required: true},
		},
		Capabilities: domain.Capabilities{Inspect: true, Prepare: true, NativeSubmit: true},
	}
	inspection.Fingerprint = domain.Fingerprint(target, inspection.Fields, inspection.Questions)
	return inspection
}
