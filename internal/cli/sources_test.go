package cli_test

import (
	"testing"
)

// representativeApplicationURLs covers every Application Provider reported by
// sources status with a URL that resolves to that provider without network use.
var representativeApplicationURLs = map[string]string{
	"greenhouse":      "https://boards.greenhouse.io/acme/jobs/4123456",
	"linkedin":        "https://www.linkedin.com/jobs/view/3912345678/",
	"indeed":          "https://www.indeed.com/viewjob?jk=1442abc",
	"lever":           "https://jobs.lever.co/palantir/6ed76ce8-4156-4b60-b120-403538bd66cd",
	"ashby":           "https://jobs.ashbyhq.com/serverobotics/0b9f3986-9ae6-4361-85b5-2e1edc6e10e2",
	"workday":         "https://nvidia.wd5.myworkdayjobs.com/en-US/NVIDIAExternalCareerSite/job/US-CA-Santa-Clara/Senior-Systems-Software-Engineer_JR19827",
	"smartrecruiters": "https://jobs.smartrecruiters.com/Visa/744000012345678-senior-software-engineer",
	"icims":           "https://careers-acme.icims.com/jobs/12345/senior-engineer/job",
	"external":        "https://careers.example.com/jobs/1234",
	"unknown":         "https://example.org/about",
}

var capabilityFlags = []string{"inspect", "prepare", "native_submit", "browser_required", "auth_required"}

func TestSourcesStatusProviderCapabilitiesMatchResolver(t *testing.T) {
	h := newHarness(t).useRealRegistry()

	sourcesOut, _, code := h.run("--json", "sources", "status")
	if code != 0 {
		t.Fatalf("sources status exit = %d", code)
	}
	providers, ok := decodeEnvelope(t, sourcesOut).Data["providers"].([]any)
	if !ok || len(providers) == 0 {
		t.Fatalf("sources status data.providers = %#v", decodeEnvelope(t, sourcesOut).Data["providers"])
	}
	if len(providers) != len(representativeApplicationURLs) {
		t.Fatalf("sources status reports %d providers; the consistency test covers %d", len(providers), len(representativeApplicationURLs))
	}

	for _, raw := range providers {
		entry, isObject := raw.(map[string]any)
		if !isObject {
			t.Fatalf("provider entry = %#v", raw)
		}
		name, isName := entry["name"].(string)
		if !isName {
			t.Fatalf("provider entry has no name: %#v", entry)
		}
		rawURL, covered := representativeApplicationURLs[name]
		if !covered {
			t.Fatalf("provider %q is not covered by representativeApplicationURLs", name)
		}

		resolveOut, _, code := h.run("--json", "resolve", rawURL)
		if code != 0 {
			t.Fatalf("resolve %s exit = %d", rawURL, code)
		}
		target := decodeEnvelope(t, resolveOut).Data
		if got := target["provider"]; got != name {
			t.Fatalf("resolve %s provider = %v, want %q", rawURL, got, name)
		}
		capabilities, isObject := target["capabilities"].(map[string]any)
		if !isObject {
			t.Fatalf("resolve %s capabilities = %#v", rawURL, target["capabilities"])
		}

		for _, flag := range capabilityFlags {
			if entry[flag] != capabilities[flag] {
				t.Errorf("provider %s: sources status %s = %v, but resolve reports %v", name, flag, entry[flag], capabilities[flag])
			}
		}
	}
}
