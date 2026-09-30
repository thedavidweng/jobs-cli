package resolver_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/thedavidweng/jobs-cli/v2/internal/domain"
	joberrors "github.com/thedavidweng/jobs-cli/v2/internal/errors"
	"github.com/thedavidweng/jobs-cli/v2/internal/resolver"
	"github.com/thedavidweng/jobs-cli/v2/internal/testutil"
)

func publicLookup(_ context.Context, _ string) ([]net.IP, error) {
	return []net.IP{net.ParseIP("93.184.216.34")}, nil
}

func offlineClient(t *testing.T) *http.Client {
	t.Helper()
	return testutil.NewClient(func(req *http.Request) (*http.Response, error) {
		return nil, fmt.Errorf("unexpected network request to %s", req.URL)
	})
}

func redirectResponse(location string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusFound,
		Header:     http.Header{"Location": []string{location}},
		Body:       io.NopCloser(strings.NewReader("")),
	}
}

func requireResolutionFailure(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected an ATS_RESOLUTION_FAILED error, got nil")
	}
	var jerr *joberrors.Error
	if !errors.As(err, &jerr) || jerr.Code != joberrors.ATSResolutionFailed {
		t.Fatalf("error = %v, want ATS_RESOLUTION_FAILED", err)
	}
}

func TestResolveClassifiesProviders(t *testing.T) {
	r := resolver.New(offlineClient(t))
	browserRequired := domain.Capabilities{BrowserRequired: true}

	cases := []struct {
		name         string
		url          string
		provider     domain.ApplicationProvider
		jobID        string
		boardToken   string
		tenant       string
		site         string
		wantURL      string
		capabilities domain.Capabilities
		verification domain.VerificationPosture
	}{
		{
			name:         "greenhouse_board",
			url:          "https://boards.greenhouse.io/acme/jobs/4123456",
			provider:     domain.ProviderGreenhouse,
			jobID:        "4123456",
			boardToken:   "acme",
			wantURL:      "https://boards.greenhouse.io/acme/jobs/4123456",
			capabilities: domain.Capabilities{Inspect: true, Prepare: true, AuthRequired: true},
			verification: domain.PartiallyVerified,
		},
		{
			name:         "greenhouse_embed",
			url:          "https://boards.greenhouse.io/embed/job_app?for=acme&token=4123456&utm_source=indeed",
			provider:     domain.ProviderGreenhouse,
			jobID:        "4123456",
			boardToken:   "acme",
			wantURL:      "https://job-boards.greenhouse.io/acme/jobs/4123456",
			capabilities: domain.Capabilities{Inspect: true, Prepare: true, AuthRequired: true},
			verification: domain.PartiallyVerified,
		},
		{
			name:         "lever",
			url:          "https://jobs.lever.co/palantir/6ed76ce8-4156-4b60-b120-403538bd66cd",
			provider:     domain.ProviderLever,
			jobID:        "6ed76ce8-4156-4b60-b120-403538bd66cd",
			boardToken:   "palantir",
			wantURL:      "https://jobs.lever.co/palantir/6ed76ce8-4156-4b60-b120-403538bd66cd",
			capabilities: browserRequired,
			verification: domain.BrowserRequiredPosture,
		},
		{
			name:         "ashby",
			url:          "https://jobs.ashbyhq.com/serverobotics/0b9f3986-9ae6-4361-85b5-2e1edc6e10e2?utm_source=j20ZWL4oeG",
			provider:     domain.ProviderAshby,
			jobID:        "0b9f3986-9ae6-4361-85b5-2e1edc6e10e2",
			boardToken:   "serverobotics",
			wantURL:      "https://jobs.ashbyhq.com/serverobotics/0b9f3986-9ae6-4361-85b5-2e1edc6e10e2",
			capabilities: browserRequired,
			verification: domain.BrowserRequiredPosture,
		},
		{
			name:         "workday",
			url:          "https://nvidia.wd5.myworkdayjobs.com/en-US/NVIDIAExternalCareerSite/job/US-CA-Santa-Clara/Senior-Systems-Software-Engineer_JR19827",
			provider:     domain.ProviderWorkday,
			jobID:        "JR19827",
			tenant:       "nvidia",
			site:         "NVIDIAExternalCareerSite",
			wantURL:      "https://nvidia.wd5.myworkdayjobs.com/en-US/NVIDIAExternalCareerSite/job/US-CA-Santa-Clara/Senior-Systems-Software-Engineer_JR19827",
			capabilities: domain.CapabilitiesFor(domain.ProviderWorkday),
			verification: domain.PartiallyVerified,
		},
		{
			name:         "workday_site_only",
			url:          "https://workday.wd5.myworkdayjobs.com/en-US/Workday",
			provider:     domain.ProviderWorkday,
			tenant:       "workday",
			site:         "Workday",
			wantURL:      "https://workday.wd5.myworkdayjobs.com/en-US/Workday",
			capabilities: domain.CapabilitiesFor(domain.ProviderWorkday),
			verification: domain.PartiallyVerified,
		},
		{
			name:         "smartrecruiters",
			url:          "https://jobs.smartrecruiters.com/Visa/744000012345678-senior-software-engineer",
			provider:     domain.ProviderSmartRecruiters,
			jobID:        "744000012345678",
			boardToken:   "Visa",
			wantURL:      "https://jobs.smartrecruiters.com/Visa/744000012345678-senior-software-engineer",
			capabilities: browserRequired,
			verification: domain.BrowserRequiredPosture,
		},
		{
			name:         "smartrecruiters_careers_host",
			url:          "https://careers.smartrecruiters.com/Visa/744000012345678",
			provider:     domain.ProviderSmartRecruiters,
			jobID:        "744000012345678",
			boardToken:   "Visa",
			wantURL:      "https://careers.smartrecruiters.com/Visa/744000012345678",
			capabilities: browserRequired,
			verification: domain.BrowserRequiredPosture,
		},
		{
			name:         "icims",
			url:          "https://careers-acme.icims.com/jobs/12345/senior-engineer/job",
			provider:     domain.ProviderICIMS,
			jobID:        "12345",
			boardToken:   "acme",
			wantURL:      "https://careers-acme.icims.com/jobs/12345/senior-engineer/job",
			capabilities: browserRequired,
			verification: domain.BrowserRequiredPosture,
		},
		{
			name:         "linkedin_easy_apply",
			url:          "https://www.linkedin.com/jobs/view/3912345678/",
			provider:     domain.ProviderLinkedIn,
			jobID:        "3912345678",
			wantURL:      "https://www.linkedin.com/jobs/view/3912345678",
			capabilities: domain.Capabilities{Inspect: true, Prepare: true, AuthRequired: true},
			verification: domain.VerifiedSourceImpl,
		},
		{
			name:         "linkedin_slug",
			url:          "https://www.linkedin.com/jobs/view/senior-engineer-at-acme-3912345678?refId=abc&trackingId=xyz",
			provider:     domain.ProviderLinkedIn,
			jobID:        "3912345678",
			wantURL:      "https://www.linkedin.com/jobs/view/senior-engineer-at-acme-3912345678",
			capabilities: domain.Capabilities{Inspect: true, Prepare: true, AuthRequired: true},
			verification: domain.VerifiedSourceImpl,
		},
		{
			name:         "indeed_native_apply",
			url:          "https://www.indeed.com/viewjob?jk=1442abc&from=serp",
			provider:     domain.ProviderIndeed,
			jobID:        "1442abc",
			wantURL:      "https://www.indeed.com/viewjob?jk=1442abc",
			capabilities: domain.CapabilitiesFor(domain.ProviderIndeed),
			verification: domain.BrowserRequiredPosture,
		},
		{
			name:         "generic_external",
			url:          "https://careers.example.com/jobs/1234",
			provider:     domain.ProviderExternal,
			wantURL:      "https://careers.example.com/jobs/1234",
			capabilities: browserRequired,
			verification: domain.BrowserRequiredPosture,
		},
		{
			name:         "undetectable",
			url:          "https://example.org/about",
			provider:     domain.ProviderUnknown,
			wantURL:      "https://example.org/about",
			capabilities: browserRequired,
			verification: domain.BrowserRequiredPosture,
		},
		{
			name:         "public_literal_ip",
			url:          "https://198.51.100.7/jobs/1",
			provider:     domain.ProviderUnknown,
			wantURL:      "https://198.51.100.7/jobs/1",
			capabilities: browserRequired,
			verification: domain.BrowserRequiredPosture,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			target, err := r.Resolve(context.Background(), tc.url)
			if err != nil {
				t.Fatalf("resolve %s: %v", tc.url, err)
			}
			if target.Provider != tc.provider {
				t.Errorf("provider = %q, want %q", target.Provider, tc.provider)
			}
			if target.URL != tc.wantURL {
				t.Errorf("url = %q, want %q", target.URL, tc.wantURL)
			}
			if target.ProviderJobID != tc.jobID {
				t.Errorf("provider_job_id = %q, want %q", target.ProviderJobID, tc.jobID)
			}
			if target.BoardToken != tc.boardToken {
				t.Errorf("board_token = %q, want %q", target.BoardToken, tc.boardToken)
			}
			if target.Tenant != tc.tenant {
				t.Errorf("tenant = %q, want %q", target.Tenant, tc.tenant)
			}
			if target.Site != tc.site {
				t.Errorf("site = %q, want %q", target.Site, tc.site)
			}
			if target.Capabilities != tc.capabilities {
				t.Errorf("capabilities = %+v, want %+v", target.Capabilities, tc.capabilities)
			}
			if target.Verification != tc.verification {
				t.Errorf("verification = %q, want %q", target.Verification, tc.verification)
			}
			if target.ResolvedFrom != "" {
				t.Errorf("resolved_from = %q, want empty", target.ResolvedFrom)
			}
		})
	}
}

func TestResolveFollowsGreenhouseTrackingRedirect(t *testing.T) {
	requests := 0
	client := testutil.NewClient(func(req *http.Request) (*http.Response, error) {
		requests++
		switch req.URL.Host {
		case "grnh.se":
			return redirectResponse("https://job-boards.greenhouse.io/acme/jobs/4123456?gh_src=abc&utm_source=indeed"), nil
		case "job-boards.greenhouse.io":
			return testutil.HTMLResponse(http.StatusOK, "<html></html>"), nil
		default:
			return nil, fmt.Errorf("unexpected host %s", req.URL.Host)
		}
	})
	r := &resolver.Resolver{Client: client, LookupIP: publicLookup}

	target, err := r.Resolve(context.Background(), "https://grnh.se/672hg3zi7us")
	if err != nil {
		t.Fatalf("resolve tracking link: %v", err)
	}
	if target.Provider != domain.ProviderGreenhouse {
		t.Errorf("provider = %q, want greenhouse", target.Provider)
	}
	if target.BoardToken != "acme" || target.ProviderJobID != "4123456" {
		t.Errorf("ids = %q/%q, want acme/4123456", target.BoardToken, target.ProviderJobID)
	}
	if target.URL != "https://job-boards.greenhouse.io/acme/jobs/4123456" {
		t.Errorf("url = %q", target.URL)
	}
	if target.ResolvedFrom != "https://grnh.se/672hg3zi7us" {
		t.Errorf("resolved_from = %q, want the tracking link", target.ResolvedFrom)
	}
	if requests != 2 {
		t.Errorf("requests = %d, want 2 (tracking host plus canonical)", requests)
	}
}

func TestResolveEnforcesRedirectLimit(t *testing.T) {
	requests := 0
	client := testutil.NewClient(func(req *http.Request) (*http.Response, error) {
		requests++
		return redirectResponse("https://grnh.se/hop" + strconv.Itoa(requests)), nil
	})
	r := &resolver.Resolver{Client: client, LookupIP: publicLookup}

	_, err := r.Resolve(context.Background(), "https://grnh.se/start")
	requireResolutionFailure(t, err)
	if requests != 6 {
		t.Errorf("requests = %d, want 6 (five redirects plus the rejected sixth)", requests)
	}
}

func TestResolveRejectsHTTPSDowngrade(t *testing.T) {
	requests := 0
	client := testutil.NewClient(func(req *http.Request) (*http.Response, error) {
		requests++
		return redirectResponse("http://job-boards.greenhouse.io/acme/jobs/1"), nil
	})
	r := &resolver.Resolver{Client: client, LookupIP: publicLookup}

	_, err := r.Resolve(context.Background(), "https://grnh.se/672hg3zi7us")
	requireResolutionFailure(t, err)
	if requests != 1 {
		t.Errorf("requests = %d, want 1 (clear-text target must not be contacted)", requests)
	}
}

func TestResolveRejectsHostnameResolvingToPrivateAddress(t *testing.T) {
	calls := 0
	r := &resolver.Resolver{
		Client: testutil.NewClient(func(req *http.Request) (*http.Response, error) {
			calls++
			return redirectResponse("https://job-boards.greenhouse.io/acme/jobs/1"), nil
		}),
		LookupIP: func(_ context.Context, _ string) ([]net.IP, error) {
			return []net.IP{net.ParseIP("10.4.5.6")}, nil
		},
	}

	_, err := r.Resolve(context.Background(), "https://grnh.se/672hg3zi7us")
	requireResolutionFailure(t, err)
	if calls != 0 {
		t.Fatalf("transport called %d times, want 0 for a private-address target", calls)
	}
}

func TestResolveRejectsPrivateAndMetadataTargets(t *testing.T) {
	r := resolver.New(offlineClient(t))
	for _, rawURL := range []string{
		"https://127.0.0.1/jobs/1",
		"https://10.1.2.3/jobs/1",
		"https://172.16.0.9/jobs/1",
		"https://192.168.1.10/jobs/1",
		"https://169.254.169.254/latest/meta-data/",
		"https://[::1]/jobs/1",
		"https://[fc00::1]/jobs/1",
		"https://localhost/jobs/1",
		"https://internal.local/jobs/1",
	} {
		t.Run(rawURL, func(t *testing.T) {
			_, err := r.Resolve(context.Background(), rawURL)
			requireResolutionFailure(t, err)
		})
	}
}

func TestResolveRejectsNonApplicationInputs(t *testing.T) {
	r := resolver.New(offlineClient(t))
	for _, input := range []string{"", "   ", "not-a-url", "mailto:candidate@example.com", "ftp://example.com/jobs/1", "example.com/jobs/1"} {
		t.Run(input, func(t *testing.T) {
			_, err := r.Resolve(context.Background(), input)
			requireResolutionFailure(t, err)
		})
	}
}
