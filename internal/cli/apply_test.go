package cli_test

import (
	"fmt"
	"testing"

	"github.com/thedavidweng/jobs-cli/internal/domain"
	"github.com/thedavidweng/jobs-cli/internal/errors"
	"github.com/thedavidweng/jobs-cli/internal/testutil"
)

func prepareFixture(t *testing.T) (*harness, *testutil.FakeProvider) {
	t.Helper()
	target := greenhouseTarget()
	provider := &testutil.FakeProvider{
		ProviderName: domain.ProviderGreenhouse,
		Inspection:   inspectionWithQuestions(target, domain.ProviderGreenhouse),
	}
	reg := testutil.NewRegistry(
		map[domain.Source]domain.SourceAdapter{
			domain.SourceIndeed: &testutil.FakeSource{SourceName: domain.SourceIndeed, Job: fakeJob(domain.SourceIndeed, "1")},
		},
		&testutil.FakeResolver{Target: target},
		map[domain.ApplicationProvider]domain.ApplyProvider{domain.ProviderGreenhouse: provider},
	)
	h := newHarness(t).useRegistry(reg)
	resume := h.writeFile("resume.pdf", "resume-bytes")
	manifest := h.writeFile("manifest.json", fmt.Sprintf(
		`{"candidate":{"first_name":"Ada","last_name":"Lovelace","email":"ada@example.com"},"answers":[{"question_id":"q1","value":"Because"}],"attachments":[{"kind":"resume","path":%q}]}`,
		resume,
	))
	out, errOut, code := h.run("--json", "apply", "prepare", "indeed:1", "--manifest", manifest)
	if code != 0 {
		t.Fatalf("prepare failed: exit=%d stderr=%s stdout=%s", code, errOut, out)
	}
	h.withStdin(out)
	return h, provider
}

func TestApplyInspectReturnsStructuredQuestions(t *testing.T) {
	target := greenhouseTarget()
	provider := &testutil.FakeProvider{
		ProviderName: domain.ProviderGreenhouse,
		Inspection:   inspectionWithQuestions(target, domain.ProviderGreenhouse),
	}
	reg := testutil.NewRegistry(
		map[domain.Source]domain.SourceAdapter{
			domain.SourceIndeed: &testutil.FakeSource{SourceName: domain.SourceIndeed, Job: fakeJob(domain.SourceIndeed, "1")},
		},
		&testutil.FakeResolver{Target: target},
		map[domain.ApplicationProvider]domain.ApplyProvider{domain.ProviderGreenhouse: provider},
	)
	h := newHarness(t).useRegistry(reg)
	out, _, code := h.run("--json", "apply", "inspect", "indeed:1")
	if code != 0 {
		t.Fatalf("exit = %d", code)
	}
	doc := decodeEnvelope(t, out)
	if doc.Data["fingerprint"] == "" {
		t.Fatal("inspection is missing a schema fingerprint")
	}
	questions, ok := doc.Data["questions"].([]any)
	if !ok || len(questions) != 1 {
		t.Fatalf("questions = %#v, want one question", doc.Data["questions"])
	}
	first, ok := questions[0].(map[string]any)
	if !ok || first["id"] != "q1" || first["required"] != true {
		t.Fatalf("question = %#v", questions[0])
	}
}

func TestApplyPrepareWritesArtifactAndSubmitsRoundTrip(t *testing.T) {
	h, provider := prepareFixture(t)
	artifactPath := h.dir + "/artifact.json"
	resume := h.dir + "/resume.pdf"
	manifest := h.writeFile("roundtrip.json", fmt.Sprintf(
		`{"candidate":{"first_name":"Ada","last_name":"Lovelace","email":"ada@example.com"},"answers":[{"question_id":"q1","value":"Because"}],"attachments":[{"kind":"resume","path":%q}]}`,
		resume,
	))

	out, _, code := h.run("--json", "apply", "prepare", "indeed:1", "--manifest", manifest, "--out", artifactPath)
	if code != 0 {
		t.Fatalf("exit = %d", code)
	}
	doc := decodeEnvelope(t, out)
	if doc.Data["path"] != artifactPath {
		t.Fatalf("data.path = %v, want %s", doc.Data["path"], artifactPath)
	}
	artifactData, ok := doc.Data["artifact"].(map[string]any)
	if !ok {
		t.Fatalf("data.artifact missing: %#v", doc.Data)
	}
	if artifactData["artifact_version"] != float64(domain.ArtifactVersion) {
		t.Fatalf("artifact_version = %v", artifactData["artifact_version"])
	}
	if artifactData["fingerprint"] != provider.Inspection.Fingerprint {
		t.Fatalf("artifact fingerprint = %v, want %s", artifactData["fingerprint"], provider.Inspection.Fingerprint)
	}

	submitted, _, code := h.run("--json", "apply", "submit", "--artifact", artifactPath, "--confirm")
	if code != 0 {
		t.Fatalf("submit exit = %d", code)
	}
	if provider.SubmitCalls != 1 {
		t.Fatalf("submit calls = %d, want 1", provider.SubmitCalls)
	}
	if decodeEnvelope(t, submitted).Data["submitted"] != true {
		t.Fatalf("submission result missing: %s", submitted)
	}

	stdinRoundTrip, _, code := h.run("--json", "apply", "submit", "--artifact", "-", "--confirm")
	if code != 0 {
		t.Fatalf("stdin submit exit = %d", code)
	}
	if provider.SubmitCalls != 2 {
		t.Fatalf("submit calls after stdin round trip = %d, want 2", provider.SubmitCalls)
	}
	if decodeEnvelope(t, stdinRoundTrip).Data["submitted"] != true {
		t.Fatalf("stdin submission result missing")
	}

	human, _, code := h.run("apply", "prepare", "indeed:1", "--manifest", manifest)
	if code != 2 {
		t.Fatalf("human prepare without --out exit = %d, want 2", code)
	}
	_ = human
}

func TestApplySubmitGates(t *testing.T) {
	t.Run("dry_run_plans_and_never_posts", func(t *testing.T) {
		h, provider := prepareFixture(t)
		out, _, code := h.run("--json", "apply", "submit", "--artifact", "-", "--dry-run")
		if code != 0 {
			t.Fatalf("exit = %d", code)
		}
		doc := decodeEnvelope(t, out)
		if doc.Data["dry_run"] != true {
			t.Fatalf("dry_run = %v", doc.Data["dry_run"])
		}
		mutations, ok := doc.Data["planned_mutations"].([]any)
		if !ok || len(mutations) != 1 {
			t.Fatalf("planned_mutations = %#v", doc.Data["planned_mutations"])
		}
		first, fok := mutations[0].(map[string]any)
		if !fok {
			t.Fatalf("planned_mutations[0] = %#v", mutations[0])
		}
		if first["action"] != "submit_application" {
			t.Fatalf("action = %v", first["action"])
		}
		if provider.SubmitCalls != 0 {
			t.Fatalf("submit called %d times during --dry-run", provider.SubmitCalls)
		}
	})

	t.Run("confirm_posts_once", func(t *testing.T) {
		h, provider := prepareFixture(t)
		out, _, code := h.run("--json", "apply", "submit", "--artifact", "-", "--confirm")
		if code != 0 {
			t.Fatalf("exit = %d", code)
		}
		if provider.SubmitCalls != 1 {
			t.Fatalf("submit calls = %d, want exactly 1", provider.SubmitCalls)
		}
		if decodeEnvelope(t, out).Data["submitted"] != true {
			t.Fatal("result not reported as submitted")
		}
	})
}

func TestResolveReturnsOnlyApplicationTarget(t *testing.T) {
	target := greenhouseTarget()
	resolver := &testutil.FakeResolver{Target: target}
	job := fakeJob(domain.SourceIndeed, "1")
	reg := testutil.NewRegistry(
		map[domain.Source]domain.SourceAdapter{domain.SourceIndeed: &testutil.FakeSource{SourceName: domain.SourceIndeed, Job: job}},
		resolver,
		map[domain.ApplicationProvider]domain.ApplyProvider{},
	)
	h := newHarness(t).useRegistry(reg)
	out, _, code := h.run("--json", "resolve", "indeed:1")
	if code != 0 {
		t.Fatalf("exit = %d", code)
	}
	doc := decodeEnvelope(t, out)
	if doc.Data["provider"] != "greenhouse" {
		t.Fatalf("provider = %v", doc.Data["provider"])
	}
	if doc.Data["url"] != target.URL {
		t.Fatalf("url = %v", doc.Data["url"])
	}
	for _, forbidden := range []string{"id", "title", "employer", "description", "source_job_id"} {
		if _, ok := doc.Data[forbidden]; ok {
			t.Fatalf("resolve leaked Job field %q: %#v", forbidden, doc.Data)
		}
	}
	if len(resolver.Calls) != 1 || resolver.Calls[0] != job.ApplicationURL {
		t.Fatalf("resolver calls = %#v, want [%s]", resolver.Calls, job.ApplicationURL)
	}
}

func TestShowResolveIsOptIn(t *testing.T) {
	target := greenhouseTarget()
	resolver := &testutil.FakeResolver{Target: target}
	reg := testutil.NewRegistry(
		map[domain.Source]domain.SourceAdapter{domain.SourceIndeed: &testutil.FakeSource{SourceName: domain.SourceIndeed, Job: fakeJob(domain.SourceIndeed, "1")}},
		resolver,
		map[domain.ApplicationProvider]domain.ApplyProvider{},
	)
	h := newHarness(t).useRegistry(reg)

	out, _, code := h.run("--json", "show", "indeed:1")
	if code != 0 {
		t.Fatalf("exit = %d", code)
	}
	if _, ok := decodeEnvelope(t, out).Data["application"]; ok {
		t.Fatal("show attached an Application Target without --resolve")
	}
	if len(resolver.Calls) != 0 {
		t.Fatalf("resolver was called %d times without --resolve", len(resolver.Calls))
	}

	out, _, code = h.run("--json", "show", "indeed:1", "--resolve")
	if code != 0 {
		t.Fatalf("exit = %d", code)
	}
	if _, ok := decodeEnvelope(t, out).Data["application"]; !ok {
		t.Fatalf("show --resolve did not attach the target: %s", out)
	}

	failing := testutil.NewRegistry(
		map[domain.Source]domain.SourceAdapter{domain.SourceIndeed: &testutil.FakeSource{SourceName: domain.SourceIndeed, Job: fakeJob(domain.SourceIndeed, "1")}},
		&testutil.FakeResolver{Err: errors.New(errors.ATSResolutionFailed, "resolution unavailable", errors.CatAPI, false, nil)},
		map[domain.ApplicationProvider]domain.ApplyProvider{},
	)
	h2 := newHarness(t).useRegistry(failing)
	out, _, code = h2.run("--json", "show", "indeed:1", "--resolve")
	if code != 0 {
		t.Fatalf("best-effort resolve failure should exit 0, got %d", code)
	}
	doc := decodeEnvelope(t, out)
	if _, ok := doc.Data["application"]; ok {
		t.Fatal("failed resolve should not attach a target")
	}
	if len(doc.Meta.Warnings) == 0 {
		t.Fatal("failed resolve should record a warning")
	}
}

func TestLinkedInEasyApplySubmitIsGatedAsUnverified(t *testing.T) {
	target := &domain.ApplicationTarget{
		URL:      "https://www.linkedin.com/jobs/view/1",
		Provider: domain.ProviderLinkedIn,
		Capabilities: domain.Capabilities{
			Inspect: true, Prepare: true, AuthRequired: true,
		},
		Verification: domain.VerifiedSourceImpl,
	}
	inspection := &domain.ApplicationInspection{
		Provider:     domain.ProviderLinkedIn,
		Application:  *target,
		Capabilities: domain.Capabilities{Inspect: true, Prepare: true, AuthRequired: true},
	}
	inspection.Fingerprint = domain.Fingerprint(target, inspection.Fields, inspection.Questions)
	provider := &testutil.FakeProvider{
		ProviderName: domain.ProviderLinkedIn,
		Caps:         domain.Capabilities{Inspect: true, Prepare: true, AuthRequired: true},
		Inspection:   inspection,
		SubmitErr: errors.New(errors.LinkedInEasyApplyUnverified,
			"Easy Apply submission is disabled until the Voyager implementation is verified with a live session",
			errors.CatAPI, false, nil),
	}
	job := fakeJob(domain.SourceLinkedIn, "1")
	job.ApplicationURL = target.URL
	reg := testutil.NewRegistry(
		map[domain.Source]domain.SourceAdapter{domain.SourceLinkedIn: &testutil.FakeSource{SourceName: domain.SourceLinkedIn, Job: job}},
		&testutil.FakeResolver{Target: target},
		map[domain.ApplicationProvider]domain.ApplyProvider{domain.ProviderLinkedIn: provider},
	)
	h := newHarness(t).useRegistry(reg)
	manifest := h.writeFile("linkedin.json", "{}")

	out, errOut, code := h.run("--json", "apply", "prepare", "linkedin:1", "--manifest", manifest)
	if code != 0 {
		t.Fatalf("prepare exit = %d (stderr: %s, stdout: %s)", code, errOut, out)
	}
	h.withStdin(out)
	submitOut, _, code := h.run("--json", "apply", "submit", "--artifact", "-", "--confirm")
	doc := decodeEnvelope(t, submitOut)
	requireCode(t, &doc, "LINKEDIN_EASY_APPLY_UNVERIFIED", 6, code)
	if provider.SubmitCalls != 1 {
		t.Fatalf("submit calls = %d, want 1 (gating happens at the provider boundary)", provider.SubmitCalls)
	}
}
