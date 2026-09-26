package cli_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/thedavidweng/jobs-cli/internal/domain"
	"github.com/thedavidweng/jobs-cli/internal/errors"
	"github.com/thedavidweng/jobs-cli/internal/greenhouse"
	"github.com/thedavidweng/jobs-cli/internal/testutil"
)

func TestVersionEnvelopeHasMetaFields(t *testing.T) {
	h := newHarness(t).useRealRegistry()
	out, errOut, code := h.run("--json", "version")
	if code != 0 {
		t.Fatalf("exit = %d, want 0 (stderr: %s)", code, errOut)
	}
	if errOut != "" {
		t.Fatalf("version wrote diagnostics to stderr in JSON mode: %q", errOut)
	}
	doc := decodeEnvelope(t, out)
	if !doc.OK {
		t.Fatal("ok = false")
	}
	if doc.Meta.Command != "version" {
		t.Fatalf("meta.command = %q, want version", doc.Meta.Command)
	}
	if doc.Meta.SchemaVersion == "" {
		t.Fatal("meta.schema_version is empty")
	}
	if doc.Meta.RequestID == "" {
		t.Fatal("meta.request_id is empty")
	}
	if doc.Meta.Profile == "" {
		t.Fatal("meta.profile is empty")
	}
	if doc.Meta.DurationMS < 0 {
		t.Fatal("meta.duration_ms is negative")
	}
	if doc.Data["version"] == "" {
		t.Fatal("data.version is empty")
	}
}

func TestExitCodeTaxonomy(t *testing.T) {
	t.Run("invalid_arguments", func(t *testing.T) {
		h := newHarness(t).useRealRegistry()
		out, _, code := h.run("--json", "search")
		doc := decodeEnvelope(t, out)
		requireCode(t, &doc, "INVALID_ARGUMENTS", 2, code)
		if doc.Error.Category != "validation" {
			t.Fatalf("category = %q, want validation", doc.Error.Category)
		}
		if doc.Meta.Command != "search" {
			t.Fatalf("meta.command = %q, want search", doc.Meta.Command)
		}
	})

	t.Run("unsupported_source_value", func(t *testing.T) {
		h := newHarness(t).useRealRegistry()
		out, _, code := h.run("--json", "search", "-q", "go", "--source", "greenhouse")
		doc := decodeEnvelope(t, out)
		requireCode(t, &doc, "INVALID_ARGUMENTS", 2, code)
	})

	t.Run("source_unavailable", func(t *testing.T) {
		h := newHarness(t).useRealRegistry()
		out, _, code := h.run("--json", "search", "-q", "go", "--source", "indeed")
		doc := decodeEnvelope(t, out)
		requireCode(t, &doc, "SOURCE_UNAVAILABLE", 5, code)
	})

	t.Run("linkedin_session_required", func(t *testing.T) {
		h := newHarness(t).useRealRegistry()
		out, _, code := h.run("--json", "search", "-q", "go", "--source", "linkedin", "--authenticated")
		doc := decodeEnvelope(t, out)
		requireCode(t, &doc, "LINKEDIN_SESSION_REQUIRED", 3, code)
		if doc.Error.Category != "auth" {
			t.Fatalf("category = %q, want auth", doc.Error.Category)
		}
	})

	t.Run("ats_resolution_failed", func(t *testing.T) {
		h := newHarness(t).useRealRegistry()
		out, _, code := h.run("--json", "resolve", "https://example.com/jobs/1")
		doc := decodeEnvelope(t, out)
		requireCode(t, &doc, "ATS_RESOLUTION_FAILED", 6, code)
	})

	t.Run("native_apply_unsupported", func(t *testing.T) {
		source := &testutil.FakeSource{SourceName: domain.SourceIndeed, Job: fakeJob(domain.SourceIndeed, "1")}
		reg := testutil.NewRegistry(
			map[domain.Source]domain.SourceAdapter{domain.SourceIndeed: source},
			&testutil.FakeResolver{Target: greenhouseTarget()},
			map[domain.ApplicationProvider]domain.ApplyProvider{domain.ProviderGreenhouse: greenhouse.NewProvider(nil)},
		)
		h := newHarness(t).useRegistry(reg)
		out, _, code := h.run("--json", "apply", "inspect", "indeed:1")
		doc := decodeEnvelope(t, out)
		requireCode(t, &doc, "NATIVE_APPLY_UNSUPPORTED", 6, code)
	})

	t.Run("read_only_violation", func(t *testing.T) {
		h, provider := prepareFixture(t)
		out, _, code := h.run("--json", "--read-only", "apply", "submit", "--artifact", "-", "--confirm")
		doc := decodeEnvelope(t, out)
		requireCode(t, &doc, "READ_ONLY_VIOLATION", 4, code)
		if doc.Error.Category != "safety" {
			t.Fatalf("category = %q, want safety", doc.Error.Category)
		}
		if provider.SubmitCalls != 0 {
			t.Fatalf("submit called %d times in read-only mode", provider.SubmitCalls)
		}
	})

	t.Run("application_incomplete", func(t *testing.T) {
		h, _ := prepareFixture(t)
		manifest := h.writeFile("incomplete.json", `{"candidate":{"first_name":"Ada"},"answers":[{"question_id":"q1","value":"Because"}]}`)
		out, _, code := h.run("--json", "apply", "prepare", "indeed:1", "--manifest", manifest, "--out", h.dir+"/incomplete-artifact.json")
		doc := decodeEnvelope(t, out)
		requireCode(t, &doc, "APPLICATION_INCOMPLETE", 7, code)
	})

	t.Run("confirmation_required", func(t *testing.T) {
		h, provider := prepareFixture(t)
		out, _, code := h.run("--json", "apply", "submit", "--artifact", "-")
		doc := decodeEnvelope(t, out)
		requireCode(t, &doc, "CONFIRMATION_REQUIRED", 10, code)
		if provider.SubmitCalls != 0 {
			t.Fatalf("submit called %d times without --confirm", provider.SubmitCalls)
		}
	})

	t.Run("artifact_stale", func(t *testing.T) {
		h, provider := prepareFixture(t)
		provider.Inspection.Fingerprint = "sha256:changed"
		out, _, code := h.run("--json", "apply", "submit", "--artifact", "-", "--confirm")
		doc := decodeEnvelope(t, out)
		requireCode(t, &doc, "ARTIFACT_STALE", 7, code)
		if provider.SubmitCalls != 0 {
			t.Fatalf("submit called %d times with a stale artifact", provider.SubmitCalls)
		}
	})

	t.Run("not_implemented_auth_login", func(t *testing.T) {
		h := newHarness(t).useRealRegistry()
		out, _, code := h.run("--json", "auth", "linkedin", "login")
		doc := decodeEnvelope(t, out)
		requireCode(t, &doc, "NOT_IMPLEMENTED", 1, code)
	})
}

func TestJSONModeStdoutIsOnlyTheDocument(t *testing.T) {
	linkedinSource := &testutil.FakeSource{
		SourceName: domain.SourceLinkedIn,
		SearchErr:  errors.New(errors.SourceUnavailable, "LinkedIn Guest discovery is having an outage", errors.CatNetwork, true, nil),
	}
	reg := testutil.NewRegistry(
		map[domain.Source]domain.SourceAdapter{
			domain.SourceIndeed:   &testutil.FakeSource{SourceName: domain.SourceIndeed, Partition: &domain.SearchPartition{Source: domain.SourceIndeed, Jobs: []domain.Job{}}},
			domain.SourceLinkedIn: linkedinSource,
		},
		&testutil.FakeResolver{},
		map[domain.ApplicationProvider]domain.ApplyProvider{},
	)
	h := newHarness(t).useRegistry(reg)
	out, errOut, code := h.run("--json", "search", "-q", "go")
	if code != 0 {
		t.Fatalf("exit = %d, want 0 (stderr: %s)", code, errOut)
	}
	doc := decodeEnvelope(t, out)
	if strings.Contains(out, "Warning") {
		t.Fatalf("diagnostics leaked to stdout:\n%s", out)
	}
	if !strings.Contains(errOut, "Warning:") {
		t.Fatalf("expected warnings on stderr, got %q", errOut)
	}
	if len(doc.Meta.Warnings) == 0 {
		t.Fatal("meta.warnings is empty for a partial-success search")
	}
}

func TestPrettyAndFullBehavior(t *testing.T) {
	t.Run("pretty", func(t *testing.T) {
		h := newHarness(t).useRealRegistry()
		out, _, code := h.run("--json", "--pretty", "version")
		if code != 0 {
			t.Fatalf("exit = %d", code)
		}
		if !strings.Contains(out, "\n  ") {
			t.Fatalf("--pretty output is not indented:\n%s", out)
		}
		var doc envelopeDoc
		if err := json.Unmarshal([]byte(out), &doc); err != nil {
			t.Fatalf("--pretty output is not valid JSON: %v", err)
		}
		if !doc.OK {
			t.Fatal("ok = false")
		}
	})

	t.Run("full_controls_diagnostics", func(t *testing.T) {
		reg := testutil.NewRegistry(
			map[domain.Source]domain.SourceAdapter{domain.SourceIndeed: &testutil.FakeSource{SourceName: domain.SourceIndeed, Job: fakeJob(domain.SourceIndeed, "1")}},
			&testutil.FakeResolver{},
			map[domain.ApplicationProvider]domain.ApplyProvider{},
		)
		h := newHarness(t).useRegistry(reg)

		out, _, code := h.run("--json", "show", "indeed:1")
		if code != 0 {
			t.Fatalf("exit = %d", code)
		}
		if _, ok := decodeEnvelope(t, out).Data["diagnostics"]; ok {
			t.Fatal("diagnostics present without --full")
		}

		out, _, code = h.run("--json", "--full", "show", "indeed:1")
		if code != 0 {
			t.Fatalf("exit = %d", code)
		}
		if _, ok := decodeEnvelope(t, out).Data["diagnostics"]; !ok {
			t.Fatal("diagnostics missing with --full")
		}

		human, _, _ := h.run("show", "indeed:1")
		if !strings.Contains(human, "Backend Engineer") {
			t.Fatalf("human show output missing title:\n%s", human)
		}
		if !strings.Contains(human, "...") {
			t.Fatalf("human show output should truncate the description:\n%s", human)
		}

		humanFull, _, _ := h.run("show", "indeed:1", "--full")
		if strings.Contains(humanFull, "...") {
			t.Fatalf("--full should not truncate the description:\n%s", humanFull)
		}
	})
}
