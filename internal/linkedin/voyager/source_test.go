package voyager

import (
	"context"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/thedavidweng/jobs-cli/internal/config"
	"github.com/thedavidweng/jobs-cli/internal/domain"
	joberrors "github.com/thedavidweng/jobs-cli/internal/errors"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func rawVariables(request *http.Request) string {
	marker := "variables="
	index := strings.Index(request.URL.RawQuery, marker)
	if index < 0 {
		return ""
	}
	return request.URL.RawQuery[index+len(marker):]
}

func TestEncodeVariables(t *testing.T) {
	got, err := EncodeVariables(map[string]any{
		"query": map[string]any{"keywords": "go developer", "urn": "urn:li:fsd_geo:1"},
		"tags":  []string{"R", ""},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := "(query:(keywords:go%20developer,urn:urn%3Ali%3Afsd_geo%3A1),tags:List(R,''))"
	if got != want {
		t.Fatalf("EncodeVariables() = %q, want %q", got, want)
	}
}

func TestSearchEncodesLocationUnionGeoUrn(t *testing.T) {
	store := testStore(t)
	var request *http.Request
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		request = req.Clone(req.Context())
		body := `{"data":{"jobsDashJobCardsByJobSearch":{"paging":{"total":0},"elements":[]}}}`
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}
	source := &Source{Client: client, Sessions: store}
	if _, err := source.Search(context.Background(), &domain.SearchRequest{Keywords: "swift", Location: "Vancouver, BC", Limit: 10}); err != nil {
		t.Fatal(err)
	}
	variables := rawVariables(request)
	want := "locationUnion:(geoUrn:urn%3Ali%3Afsd_geo%3A103366113)"
	if !strings.Contains(variables, want) {
		t.Fatalf("variables missing the locationUnion geoUrn shape %q:\n%s", want, variables)
	}
	if !strings.Contains(variables, "(keywords:swift,locationUnion:(geoUrn:urn%3Ali%3Afsd_geo%3A103366113),origin:JOB_SEARCH_PAGE_SEARCH_BUTTON,spellCorrectionEnabled:true)") {
		t.Fatalf("variables do not pin the donor query shape:\n%s", variables)
	}
	if strings.Contains(variables, "location:Vancouver") {
		t.Fatalf("variables leak the raw location string instead of the geoUrn:\n%s", variables)
	}
}

func TestSearchAcceptsRawGeoUrnAndRejectsUnknownLocations(t *testing.T) {
	store := testStore(t)
	var request *http.Request
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		request = req.Clone(req.Context())
		body := `{"data":{"jobsDashJobCardsByJobSearch":{"paging":{"total":0},"elements":[]}}}`
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}
	source := &Source{Client: client, Sessions: store}
	if _, err := source.Search(context.Background(), &domain.SearchRequest{Keywords: "swift", Location: "urn:li:fsd_geo:91000010"}); err != nil {
		t.Fatal(err)
	}
	variables := rawVariables(request)
	if !strings.Contains(variables, "geoUrn:urn%3Ali%3Afsd_geo%3A91000010") {
		t.Fatalf("raw geo URN was not passed through:\n%s", variables)
	}

	source = &Source{Client: client, Sessions: store}
	_, err := source.Search(context.Background(), &domain.SearchRequest{Keywords: "swift", Location: "Nowhere, ZZ"})
	if got := joberrors.From(err).Code; got != joberrors.InvalidArguments {
		t.Fatalf("error code = %s, want INVALID_ARGUMENTS (message: %v)", got, err)
	}
	if !strings.Contains(err.Error(), "urn:li:fsd_geo") {
		t.Fatalf("error message does not explain the geo URN escape hatch: %v", err)
	}
}

func TestSearchSendsPersistedQueryAndAuthenticatedHeaders(t *testing.T) {
	store := testStore(t)
	var request *http.Request
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		request = req.Clone(req.Context())
		body := `{"data":{"jobsDashJobCardsByJobSearch":{"paging":{"total":1},"elements":[{"jobCard":{"jobPostingCard":{"jobPostingTitle":"Go Developer","primaryDescription":{"text":"Acme"},"secondaryDescription":{"text":"Remote"},"jobPosting":{"entityUrn":"urn:li:fsd_jobPosting:42"}}}}]}}}`
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}
	source := &Source{Client: client, Sessions: store}
	result, err := source.Search(context.Background(), &domain.SearchRequest{Keywords: "go", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if request.Method != http.MethodGet || request.URL.Path != "/voyager/api/graphql" {
		t.Fatalf("request = %s %s", request.Method, request.URL)
	}
	if request.URL.Query().Get("queryId") != searchQueryID || request.URL.Query().Get("queryName") != searchQueryName {
		t.Fatalf("query = %s", request.URL.RawQuery)
	}
	if request.Header.Get("csrf-token") != "ajax:123" || !strings.Contains(request.Header.Get("Cookie"), "li_at=secret") {
		t.Fatalf("authentication headers missing: %#v", request.Header)
	}
	if len(result.Jobs) != 1 || result.Jobs[0].SourceJobID != "42" || result.Jobs[0].Employer != "Acme" {
		t.Fatalf("jobs = %#v", result.Jobs)
	}
}

func TestDetailAndEasyApplyParsing(t *testing.T) {
	store := testStore(t)
	calls := 0
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		var body string
		switch req.URL.Query().Get("queryId") {
		case detailQueryID:
			body = `{"data":{"detail":{"elements":[{"jobPostingDetailSection":[{"topCardV2":{"jobPostingCard":{"jobPostingTitle":"Engineer","primaryDescription":{"text":"Acme"},"tertiaryDescription":{"text":"Remote · 2 days ago"}}}},{"jobDescription":{"jobPosting":{"description":{"text":"Build things"}}}}]}]}}}`
		case applyQueryID:
			body = `{"data":{"jobsDashOnsiteApplyApplicationByJobPosting":{"elements":[{"jobSeekerApplicationDetail":{"onsiteApply":true,"resume":{"name":"resume.pdf"}}}]}}}`
		default:
			t.Fatalf("unexpected query: %s", req.URL.Query().Get("queryId"))
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}
	source := &Source{Client: client, Sessions: store}
	job, err := source.Detail(context.Background(), &domain.DetailRequest{SourceJobID: "42"})
	if err != nil {
		t.Fatal(err)
	}
	if job.Title != "Engineer" || job.Description != "Build things" || job.Location != "Remote" {
		t.Fatalf("job = %#v", job)
	}
	apply, err := InspectEasyApply(context.Background(), client, store, "42")
	if err != nil {
		t.Fatal(err)
	}
	if !apply.Available || !apply.AcceptsResume || calls != 2 {
		t.Fatalf("apply = %#v, calls = %d", apply, calls)
	}
}

func TestMissingOrIncompleteSessionIsRequired(t *testing.T) {
	source := &Source{Sessions: config.NewSessionStore(t.TempDir())}
	_, err := source.Search(context.Background(), &domain.SearchRequest{})
	if got := joberrors.From(err).Code; got != joberrors.LinkedInSessionRequired {
		t.Fatalf("error code = %s", got)
	}
}

func TestInvalidStoredSessionIsRequiredWithReason(t *testing.T) {
	store := config.NewSessionStore(t.TempDir())
	if err := os.MkdirAll(store.Dir(), 0o700); err != nil {
		t.Fatal(err)
	}
	body := `{"schema_version":"1","provider":"linkedin","profile":"default","captured_at":"2026-09-26T12:00:00Z","cookies":{"li_at":"secret"}}`
	if err := os.WriteFile(store.Path(), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	source := &Source{Sessions: store}
	_, err := source.Search(context.Background(), &domain.SearchRequest{})
	e := joberrors.From(err)
	if e == nil || e.Code != joberrors.LinkedInSessionRequired {
		t.Fatalf("error = %v, want LINKEDIN_SESSION_REQUIRED", err)
	}
	if !strings.Contains(err.Error(), "invalid") || !strings.Contains(err.Error(), "JSESSIONID") {
		t.Fatalf("error does not explain the invalid session: %v", err)
	}
}

func testStore(t *testing.T) *config.SessionStore {
	t.Helper()
	store := config.NewSessionStore(t.TempDir())
	if err := store.Save(&config.LinkedInSession{Cookies: map[string]string{
		config.CookieLiAt: "secret", config.CookieJSessionID: `"ajax:123"`,
	}}); err != nil {
		t.Fatal(err)
	}
	return store
}
