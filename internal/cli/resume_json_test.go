package cli_test

import (
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/thedavidweng/jobs-cli/v2/internal/browserapply"
	"github.com/thedavidweng/jobs-cli/v2/internal/config"
	"github.com/thedavidweng/jobs-cli/v2/internal/domain"
	"github.com/thedavidweng/jobs-cli/v2/internal/greenhouse"
	"github.com/thedavidweng/jobs-cli/v2/internal/registry"
	"github.com/thedavidweng/jobs-cli/v2/internal/testutil"
)

func TestPrepareRejectsEmbeddedResumeJSON(t *testing.T) {
	for _, flag := range []string{"--manifest", "--answers"} {
		t.Run(flag, func(t *testing.T) {
			h, _ := prepareFixture(t)
			input := h.writeFile("embedded.json", `{"resume_json":{"basics":{"email":"ada@example.com"}},"answers":[{"question_id":"q1","value":"Because"}]}`)
			out, _, code := h.run("--json", "apply", "prepare", "indeed:1", flag, input)
			doc := decodeEnvelope(t, out)
			requireCode(t, &doc, "INVALID_ARGUMENTS", 2, code)
			if !strings.Contains(doc.Error.Message, "--resume-json") {
				t.Fatalf("missing supported input instruction: %s", out)
			}
		})
	}
}

func TestPrepareRejectsAmbiguousAttachmentKindsAndAllowsFlagOverride(t *testing.T) {
	for _, test := range []struct {
		name              string
		duplicateProperty bool
		flagOverride      bool
	}{
		{name: "duplicate array kind"},
		{name: "duplicate property kind", duplicateProperty: true},
		{name: "explicit flag override", flagOverride: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			h, _ := prepareFixture(t)
			first := h.writeFile("first.pdf", "first")
			second := h.writeFile("second.pdf", "second")
			body := fmt.Sprintf(`{"candidate":{"email":"ada@example.com"},"answers":[{"question_id":"q1","value":"Because"}],"attachments":[{"kind":"resume","path":%q}`, first)
			switch {
			case test.flagOverride:
				body += `]}`
			case test.duplicateProperty:
				body += fmt.Sprintf(`],"resume":%q}`, second)
			default:
				body += fmt.Sprintf(`,{"kind":"resume","path":%q}]}`, second)
			}
			manifest := h.writeFile("attachments.json", body)
			args := []string{"--json", "apply", "prepare", "indeed:1", "--manifest", manifest}
			if test.flagOverride {
				args = append(args, "--resume", second)
			}
			out, _, code := h.run(args...)
			doc := decodeEnvelope(t, out)
			if !test.flagOverride {
				requireCode(t, &doc, "INVALID_ARGUMENTS", 2, code)
				return
			}
			if code != 0 {
				t.Fatalf("flag override: %s", out)
			}
			attachments, ok := doc.Data["attachments"].([]any)
			if !ok || len(attachments) != 1 {
				t.Fatalf("attachments: %s", out)
			}
			attachment, ok := attachments[0].(map[string]any)
			if !ok || attachment["path"] != second {
				t.Fatalf("flag did not override manifest: %s", out)
			}
		})
	}
}

func TestPrepareJSONResumePreservesIdentityHistoryAndOverrides(t *testing.T) {
	h, _ := prepareFixture(t)
	resume := h.writeFile("resume.json", `{"basics":{"name":"Ada Byron Lovelace","email":"base@example.com","profiles":[{"network":" linkedin ","url":"https://www.linkedin.com/in/ada"}]},"work":[{"name":"Engine","position":"Programmer","startDate":"1842"}],"projects":[{"name":"Notes"}]}`)
	manifest := h.writeFile("override.json", `{"candidate":{"email":"reviewed@example.com"},"answers":[{"question_id":"q1","value":"Because"}]}`)
	out, _, code := h.run("--json", "apply", "prepare", "indeed:1", "--resume-json", resume, "--manifest", manifest)
	if code != 0 {
		t.Fatalf("prepare: %s", out)
	}
	doc := decodeEnvelope(t, out)
	candidate, ok := doc.Data["candidate"].(map[string]any)
	if !ok {
		t.Fatalf("candidate: %s", out)
	}
	if candidate["full_name"] != "Ada Byron Lovelace" || candidate["email"] != "reviewed@example.com" || candidate["first_name"] != nil || candidate["linkedin"] != "https://www.linkedin.com/in/ada" {
		t.Fatalf("candidate: %#v", candidate)
	}
	rows, ok := candidate["work"].([]any)
	if !ok || len(rows) != 1 {
		t.Fatalf("work rows: %s", out)
	}
	work, ok := rows[0].(map[string]any)
	if !ok {
		t.Fatalf("work entry: %s", out)
	}
	if work["startDate"] != "1842" {
		t.Fatalf("date precision: %#v", work)
	}
	raw, ok := doc.Data["resume_json"].(map[string]any)
	if !ok || raw["projects"] == nil {
		t.Fatal("unmapped projects were lost")
	}
}

func TestPrepareJSONResumeNestedManifestValuesReplaceResumeValues(t *testing.T) {
	h, _ := prepareFixture(t)
	resume := h.writeFile("resume.json", `{"basics":{"email":"base@example.com","location":{"address":"Old street","postalCode":"V1V 1V1","city":"Vancouver","countryCode":"CA","region":"BC"}},"work":[{"name":"Old employer","position":"Old title","summary":"Old description","startDate":"2020"}],"education":[{"institution":"Old school","studyType":"Old degree","area":"Old subject","startDate":"2010"}]}`)
	manifest := h.writeFile("override.json", `{"candidate":{"address":{"country":"US"},"work":[{"name":"New employer","position":"New title"}],"education":[{"institution":"New school"}]},"answers":[{"question_id":"q1","value":"Because"}]}`)
	out, _, code := h.run("--json", "apply", "prepare", "indeed:1", "--resume-json", resume, "--manifest", manifest)
	if code != 0 {
		t.Fatalf("prepare: %s", out)
	}
	doc := decodeEnvelope(t, out)
	candidate, ok := doc.Data["candidate"].(map[string]any)
	if !ok {
		t.Fatalf("candidate: %s", out)
	}
	want := map[string]any{
		"address":   map[string]any{"country": "US"},
		"work":      []any{map[string]any{"name": "New employer", "position": "New title"}},
		"education": []any{map[string]any{"institution": "New school"}},
	}
	for key, expected := range want {
		if !reflect.DeepEqual(candidate[key], expected) {
			t.Errorf("%s retained stale resume data: got %#v, want %#v", key, candidate[key], expected)
		}
	}
}

func TestPrepareJSONResumeRejectsMalformedHistory(t *testing.T) {
	h, _ := prepareFixture(t)
	resume := h.writeFile("invalid.json", `{"basics":{"email":"ada@example.com"},"work":[{"startDate":"yesterday"}]}`)
	out, _, code := h.run("--json", "apply", "prepare", "indeed:1", "--resume-json", resume)
	doc := decodeEnvelope(t, out)
	requireCode(t, &doc, "VALIDATION_FAILED", 7, code)
}

func TestJSONResumeSplitNameAndExplicitEmptyOverride(t *testing.T) {
	h, provider := prepareFixture(t)
	provider.Inspection.Fields = append(provider.Inspection.Fields, domain.ApplicationField{Name: "first_name", Label: "Given name", Required: true})
	resume := h.writeFile("identity.json", `{"basics":{"name":"Ada Byron Lovelace","email":"ada@example.com"}}`)
	manifest := h.writeFile("identity-manifest.json", `{"candidate":{"first_name":""},"answers":[{"question_id":"q1","value":"Because"}]}`)
	out, _, code := h.run("--json", "apply", "prepare", "indeed:1", "--resume-json", resume, "--manifest", manifest)
	doc := decodeEnvelope(t, out)
	requireCode(t, &doc, "APPLICATION_INCOMPLETE", 7, code)
	if !strings.Contains(doc.Error.Message, "Given name") {
		t.Fatalf("missing identity requirement: %s", out)
	}
}

func TestGreenhouseWithoutEmployerKeyCannotSubmit(t *testing.T) {
	stub := &ghStub{schema: ghSchema}
	h := useGreenhouseStub(newHarness(t), stub)
	h.factory = func(*config.Config, *http.Client) *registry.Registry {
		client := &http.Client{Transport: testutil.RoundTripFunc(stub.handle)}
		return testutil.NewRegistry(map[domain.Source]domain.SourceAdapter{domain.SourceIndeed: &testutil.FakeSource{Job: fakeJob(domain.SourceIndeed, "1")}}, &testutil.FakeResolver{Target: ghTarget()}, map[domain.ApplicationProvider]domain.ApplyProvider{domain.ProviderGreenhouse: greenhouse.NewProvider(client)})
	}
	path := ghPrepare(t, h)
	out, _, code := h.run("--json", "apply", "submit", "--artifact", path, "--confirm")
	doc := decodeEnvelope(t, out)
	requireCode(t, &doc, "AUTH_REQUIRED", 3, code)
	if stub.posts != 0 {
		t.Fatal("anonymous POST attempted")
	}
}

func TestIndeedPrepareProducesOfficialHandoffWithUnvalidatedRequirements(t *testing.T) {
	h := newHarness(t)
	target := &domain.ApplicationTarget{URL: "https://www.indeed.com/viewjob?jk=1", Provider: domain.ProviderIndeed}
	reg := testutil.NewRegistry(map[domain.Source]domain.SourceAdapter{domain.SourceIndeed: &testutil.FakeSource{Job: fakeJob(domain.SourceIndeed, "1")}}, &testutil.FakeResolver{Target: target}, map[domain.ApplicationProvider]domain.ApplyProvider{domain.ProviderIndeed: browserapply.Provider{Name: domain.ProviderIndeed}})
	h.useRegistry(reg)
	resume := h.writeFile("indeed-resume.json", `{"basics":{"name":"Ada Lovelace","email":"ada@example.com"}}`)
	out, _, code := h.run("--json", "apply", "prepare", "indeed:1", "--resume-json", resume)
	if code != 0 {
		t.Fatalf("official handoff prepare: %s", out)
	}
	doc := decodeEnvelope(t, out)
	if doc.Data["requirements_validated"] != false || doc.Data["pending_action"] != "official_handoff" {
		t.Fatalf("unvalidated handoff: %s", out)
	}
}
