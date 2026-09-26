package indeed_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/thedavidweng/jobs-cli/internal/domain"
	joberrors "github.com/thedavidweng/jobs-cli/internal/errors"
	"github.com/thedavidweng/jobs-cli/internal/indeed"
	"github.com/thedavidweng/jobs-cli/internal/testutil"
)

// pinnedAPIKey mirrors the Indeed mobile client API key shipped in
// https://github.com/speedyapply/JobSpy/blob/4ec308a302e35b2a765a6bb73cee659c4011ff91/jobspy/indeed/constant.py
const pinnedAPIKey = "161092c2017b5bbab13edb12461a62d5a833871e7cad6d9d475304573de67ac8"

const (
	firstJobKey  = "b8ef297959c26ac5"
	firstJobDate = 1790435045000
)

type recorder struct {
	t       *testing.T
	calls   int
	request *http.Request
	body    []byte
	respond func(call int) *http.Response
}

func (r *recorder) RoundTrip(req *http.Request) (*http.Response, error) {
	r.calls++
	body, err := io.ReadAll(req.Body)
	if err != nil {
		r.t.Fatalf("read request body: %v", err)
	}
	r.request = req
	r.body = body
	return r.respond(r.calls), nil
}

func newSource(r *recorder) domain.SourceAdapter {
	return indeed.NewSource(testutil.NewClient(testutil.RoundTripFunc(r.RoundTrip)))
}

func fixture(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return string(data)
}

func fixtureResponse(t *testing.T, name string) *http.Response {
	t.Helper()
	return testutil.JSONResponse(http.StatusOK, fixture(t, name))
}

func queryText(t *testing.T, body []byte) string {
	t.Helper()
	var payload struct {
		Query string `json:"query"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("request body is not the GraphQL envelope: %v\n%s", err, body)
	}
	if payload.Query == "" {
		t.Fatalf("request body carries no GraphQL query:\n%s", body)
	}
	return payload.Query
}

func requireErrorCode(t *testing.T, err error, want joberrors.Code) *joberrors.Error {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error %s, got nil", want)
	}
	var e *joberrors.Error
	if !errors.As(err, &e) {
		t.Fatalf("expected a jobs error, got %T: %v", err, err)
	}
	if e.Code != want {
		t.Fatalf("error code = %s, want %s (message: %s)", e.Code, want, e.Message)
	}
	return e
}

func floatPtr(v float64) *float64 { return &v }

func TestSearchSendsPinnedMobileClientRequest(t *testing.T) {
	r := &recorder{t: t, respond: func(int) *http.Response { return fixtureResponse(t, "search.json") }}
	if _, err := newSource(r).Search(context.Background(), &domain.SearchRequest{
		Keywords: "software engineer",
		Location: "Vancouver, BC",
		Limit:    25,
	}); err != nil {
		t.Fatalf("search: %v", err)
	}

	req := r.request
	if req.Method != http.MethodPost {
		t.Errorf("method = %s, want POST", req.Method)
	}
	if req.URL.String() != "https://apis.indeed.com/graphql" {
		t.Errorf("url = %s, want https://apis.indeed.com/graphql", req.URL)
	}
	if req.Host != "apis.indeed.com" {
		t.Errorf("host = %s, want apis.indeed.com", req.Host)
	}

	headers := []struct{ name, want string }{
		{"Content-Type", "application/json"},
		{"Accept", "application/json"},
		{"Accept-Language", "en-US,en;q=0.9"},
		{"User-Agent", "Mozilla/5.0 (iPhone; CPU iPhone OS 16_6_1 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Mobile/15E148 Indeed App 193.1"},
		{"Indeed-Api-Key", pinnedAPIKey},
		{"Indeed-App-Info", "appv=193.1; appid=com.indeed.jobsearch; osv=16.6.1; os=ios; dtype=phone"},
		{"Indeed-Locale", "en-US"},
		{"Indeed-Co", "US"},
	}
	for _, header := range headers {
		if got := req.Header.Get(header.name); got != header.want {
			t.Errorf("%s = %q, want %q", header.name, got, header.want)
		}
	}

	query := queryText(t, r.body)
	for _, want := range []string{
		"query GetJobData {",
		"jobSearch(",
		`what: "software engineer"`,
		`location: {where: "Vancouver, BC", radius: 25, radiusUnit: MILES}`,
		"limit: 25",
		"sort: RELEVANCE",
		"pageInfo {",
		"nextCursor",
		"recruit {",
		"viewJobUrl",
		"attributes {",
	} {
		if !strings.Contains(query, want) {
			t.Errorf("GraphQL query missing %q:\n%s", want, query)
		}
	}
}

func TestSearchPropagatesKeywordsLocationAndEscaping(t *testing.T) {
	r := &recorder{t: t, respond: func(int) *http.Response { return fixtureResponse(t, "search.json") }}
	if _, err := newSource(r).Search(context.Background(), &domain.SearchRequest{
		Keywords: `senior "go" engineer\platform`,
		Location: "Vancouver, BC",
		Cursor:   `cur"sor`,
	}); err != nil {
		t.Fatalf("search: %v", err)
	}
	query := queryText(t, r.body)
	for _, want := range []string{
		`what: "senior \"go\" engineer\\platform"`,
		`location: {where: "Vancouver, BC", radius: 25, radiusUnit: MILES}`,
		`cursor: "cur\"sor"`,
	} {
		if !strings.Contains(query, want) {
			t.Errorf("GraphQL query missing %q:\n%s", want, query)
		}
	}

	r = &recorder{t: t, respond: func(int) *http.Response { return fixtureResponse(t, "search.json") }}
	if _, err := newSource(r).Search(context.Background(), &domain.SearchRequest{Location: "Vancouver, BC"}); err != nil {
		t.Fatalf("location-only search: %v", err)
	}
	if query := queryText(t, r.body); strings.Contains(query, "what:") {
		t.Errorf("location-only search sent a keyword:\n%s", query)
	}
}

func TestSearchMapsLimitRadiusSortAndRemoteFilter(t *testing.T) {
	r := &recorder{t: t, respond: func(int) *http.Response { return fixtureResponse(t, "search.json") }}
	if _, err := newSource(r).Search(context.Background(), &domain.SearchRequest{
		Keywords: "platform engineer",
		Location: "Vancouver, BC",
		Radius:   40,
		Sort:     "date",
		Limit:    500,
		Remote:   func() *bool { v := true; return &v }(),
	}); err != nil {
		t.Fatalf("search: %v", err)
	}
	query := queryText(t, r.body)
	for _, want := range []string{
		"limit: 100",
		`radius: 40, radiusUnit: MILES`,
		"sort: DATE",
		`filters: {composite: {filters: [{keyword: {field: "attributes", keys: ["DSQF7"]}}]}}`,
	} {
		if !strings.Contains(query, want) {
			t.Errorf("GraphQL query missing %q:\n%s", want, query)
		}
	}

	r = &recorder{t: t, respond: func(int) *http.Response { return fixtureResponse(t, "search.json") }}
	remote := false
	if _, err := newSource(r).Search(context.Background(), &domain.SearchRequest{
		Keywords: "platform engineer",
		Remote:   &remote,
	}); err != nil {
		t.Fatalf("search: %v", err)
	}
	if query := queryText(t, r.body); strings.Contains(query, "filters:") {
		t.Errorf("remote=false must not add a filters block:\n%s", query)
	}
}

func TestSearchCursorPaginationRoundtrip(t *testing.T) {
	r := &recorder{t: t, respond: func(call int) *http.Response {
		if call == 1 {
			return fixtureResponse(t, "search.json")
		}
		return fixtureResponse(t, "search_last_page.json")
	}}
	source := newSource(r)

	first, err := source.Search(context.Background(), &domain.SearchRequest{Keywords: "go", Location: "Vancouver, BC", Limit: 25})
	if err != nil {
		t.Fatalf("first page: %v", err)
	}
	if first.Pagination == nil {
		t.Fatal("search partition has no pagination")
	}
	if !first.Pagination.HasMore || first.Pagination.NextCursor != "eyJvZmZzZXQiOjI1fQ" {
		t.Fatalf("first page pagination = %+v", first.Pagination)
	}
	if first.Pagination.Native == nil || first.Pagination.Native.Kind != domain.PaginationCursor || first.Pagination.Native.Cursor != "eyJvZmZzZXQiOjI1fQ" {
		t.Fatalf("first page native pagination = %+v", first.Pagination.Native)
	}

	second, err := source.Search(context.Background(), &domain.SearchRequest{
		Keywords: "go",
		Location: "Vancouver, BC",
		Limit:    25,
		Cursor:   first.Pagination.NextCursor,
	})
	if err != nil {
		t.Fatalf("second page: %v", err)
	}
	if !strings.Contains(queryText(t, r.body), `cursor: "eyJvZmZzZXQiOjI1fQ"`) {
		t.Errorf("continuation request did not propagate the cursor:\n%s", queryText(t, r.body))
	}
	if second.Pagination == nil || second.Pagination.HasMore || second.Pagination.NextCursor != "" {
		t.Fatalf("last page pagination = %+v", second.Pagination)
	}
	if len(second.Jobs) != 1 {
		t.Fatalf("last page jobs = %d, want 1", len(second.Jobs))
	}
}

func TestSearchNormalizesVerifiedResponseShape(t *testing.T) {
	r := &recorder{t: t, respond: func(int) *http.Response { return fixtureResponse(t, "search.json") }}
	partition, err := newSource(r).Search(context.Background(), &domain.SearchRequest{
		Keywords: "software engineer",
		Location: "Vancouver, BC",
		Limit:    25,
	})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if partition.Source != domain.SourceIndeed {
		t.Errorf("partition source = %s, want indeed", partition.Source)
	}
	if len(partition.Jobs) != 3 {
		t.Fatalf("jobs = %d, want 3", len(partition.Jobs))
	}

	first := partition.Jobs[0]
	if first.ID != "indeed:"+firstJobKey || first.SourceJobID != firstJobKey || first.Source != domain.SourceIndeed {
		t.Fatalf("first job identity = %+v", first)
	}
	if first.Title != "Sr. Embedded Software Engineer, Robotics Platform" {
		t.Errorf("first title = %q", first.Title)
	}
	if first.Employer != "Serve Robotics" {
		t.Errorf("first employer = %q", first.Employer)
	}
	if first.Location != "Vancouver, BC" {
		t.Errorf("first location = %q, want the formatted short label", first.Location)
	}
	if first.PostedDate != time.UnixMilli(firstJobDate).UTC().Format(time.RFC3339) {
		t.Errorf("first posted date = %q", first.PostedDate)
	}
	if first.Workplace != domain.WorkplaceRemote || !first.Remote {
		t.Errorf("first workplace = %s remote=%v, want remote", first.Workplace, first.Remote)
	}
	if first.SourceURL != "https://www.indeed.com/viewjob?jk="+firstJobKey {
		t.Errorf("first source url = %q", first.SourceURL)
	}
	if first.ApplicationURL != "https://jobs.ashbyhq.com/serverobotics/0b9f3986-9ae6-4361-85b5-2e1edc6e10e2?utm_source=j20ZWL4oeG" {
		t.Errorf("first application url = %q, want recruit.viewJobUrl", first.ApplicationURL)
	}
	if first.Compensation == nil {
		t.Fatal("first job has no compensation")
	}
	if first.Compensation.Currency != "CAD" || first.Compensation.Interval != domain.IntervalYear || first.Compensation.Summary != "$167,800 - $204,000 a year" {
		t.Errorf("first compensation = %+v", first.Compensation)
	}
	if len(first.Compensation.Amounts) != 1 || first.Compensation.Amounts[0].Kind != "range" ||
		*first.Compensation.Amounts[0].Min != 167800 || *first.Compensation.Amounts[0].Max != 204000 {
		t.Errorf("first compensation amounts = %+v", first.Compensation.Amounts)
	}

	second := partition.Jobs[1]
	if second.Employer != "" {
		t.Errorf("second employer = %q, want empty for a null employer node", second.Employer)
	}
	if second.Location != "Vancouver, BC (Hybrid)" {
		t.Errorf("second location = %q, want the long label when short is null", second.Location)
	}
	if second.Workplace != domain.WorkplaceHybrid || second.Remote {
		t.Errorf("second workplace = %s remote=%v, want hybrid", second.Workplace, second.Remote)
	}
	if second.Compensation == nil || len(second.Compensation.Amounts) != 1 || second.Compensation.Amounts[0].Kind != "exact" ||
		*second.Compensation.Amounts[0].Min != 210000 || *second.Compensation.Amounts[0].Max != 210000 {
		t.Fatalf("second compensation = %+v, want an exact amount", second.Compensation)
	}
	if second.ApplicationURL != "https://grnh.se/672hg3zi7us" {
		t.Errorf("second application url = %q", second.ApplicationURL)
	}

	third := partition.Jobs[2]
	if third.Compensation == nil || third.Compensation.Currency != "USD" || third.Compensation.Interval != domain.IntervalHour {
		t.Fatalf("third compensation = %+v, want the estimated fallback in USD per hour", third.Compensation)
	}
	if len(third.Compensation.Amounts) != 1 || third.Compensation.Amounts[0].Kind != "range" ||
		*third.Compensation.Amounts[0].Min != 56.5 || *third.Compensation.Amounts[0].Max != 84 {
		t.Errorf("third compensation amounts = %+v", third.Compensation.Amounts)
	}
	if third.PostedDate != "" {
		t.Errorf("third posted date = %q, want empty for a null datePublished", third.PostedDate)
	}
	if third.Workplace != domain.WorkplaceUnknown || third.Remote {
		t.Errorf("third workplace = %s remote=%v, want unknown", third.Workplace, third.Remote)
	}
	if third.ApplicationURL != "https://workday.wd5.myworkdayjobs.com/en-US/Workday/job/Canada-BC-Vancouver/DevOps-Engineer--Evisort-AI_JR-0110243" {
		t.Errorf("third application url = %q", third.ApplicationURL)
	}
}

func TestSearchPreservesAttributesAndProviderPayloadAsDiagnostics(t *testing.T) {
	r := &recorder{t: t, respond: func(int) *http.Response { return fixtureResponse(t, "search.json") }}
	partition, err := newSource(r).Search(context.Background(), &domain.SearchRequest{
		Keywords: "software engineer",
		Location: "Vancouver, BC",
	})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	first := partition.Jobs[0]
	if first.Diagnostics == nil || len(first.Diagnostics.SourcePayload) == 0 {
		t.Fatal("first job has no source payload diagnostics")
	}
	var raw map[string]any
	if err := json.Unmarshal(first.Diagnostics.SourcePayload, &raw); err != nil {
		t.Fatalf("source payload is not JSON: %v\n%s", err, first.Diagnostics.SourcePayload)
	}
	attributes, ok := raw["attributes"].([]any)
	if !ok || len(attributes) != 2 {
		t.Fatalf("source payload attributes = %#v", raw["attributes"])
	}
	if _, ok := raw["recruit"]; !ok {
		t.Fatal("source payload lost the recruit node")
	}
}

func TestSearchJobsUseTheDomainContractOnly(t *testing.T) {
	r := &recorder{t: t, respond: func(int) *http.Response { return fixtureResponse(t, "search.json") }}
	partition, err := newSource(r).Search(context.Background(), &domain.SearchRequest{
		Keywords: "software engineer",
		Location: "Vancouver, BC",
	})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	encoded, err := json.Marshal(partition.Jobs[0])
	if err != nil {
		t.Fatalf("marshal normalized job: %v", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(encoded, &raw); err != nil {
		t.Fatalf("unmarshal normalized job: %v", err)
	}
	want := map[string]bool{
		"id": true, "source": true, "source_job_id": true, "title": true, "employer": true,
		"location": true, "workplace": true, "remote": true, "posted_date": true,
		"compensation": true, "source_url": true, "application_url": true, "diagnostics": true,
	}
	for key := range raw {
		if !want[key] {
			t.Errorf("normalized job leaked the provider key %q:\n%s", key, encoded)
		}
	}
	for key := range want {
		if _, ok := raw[key]; !ok {
			t.Errorf("normalized job is missing the domain key %q:\n%s", key, encoded)
		}
	}
}

func TestSearchCompensationVariants(t *testing.T) {
	cases := []struct {
		name string
		body string
		want *domain.Compensation
	}{
		{
			name: "no compensation node",
			body: `{"data":{"jobSearch":{"pageInfo":{"nextCursor":null},"results":[{"job":{"key":"a","title":"Job A"}}]}}}`,
			want: nil,
		},
		{
			name: "salary without a range",
			body: `{"data":{"jobSearch":{"pageInfo":{"nextCursor":null},"results":[{"job":{"key":"a","title":"Job A","compensation":{"baseSalary":{"unitOfWork":"YEAR"},"currencyCode":"USD"}}}]}}}`,
			want: &domain.Compensation{Amounts: nil, Currency: "USD", Interval: domain.IntervalYear},
		},
		{
			name: "open ended minimum",
			body: `{"data":{"jobSearch":{"pageInfo":{"nextCursor":null},"results":[{"job":{"key":"a","title":"Job A","compensation":{"baseSalary":{"unitOfWork":"MONTH","range":{"min":7000}},"currencyCode":"USD"}}}]}}}`,
			want: &domain.Compensation{
				Amounts:  []domain.CompensationAmount{{Kind: "min", Min: floatPtr(7000)}},
				Currency: "USD", Interval: domain.IntervalMonth,
			},
		},
		{
			name: "capped maximum",
			body: `{"data":{"jobSearch":{"pageInfo":{"nextCursor":null},"results":[{"job":{"key":"a","title":"Job A","compensation":{"baseSalary":{"unitOfWork":"WEEK","range":{"max":2000}},"currencyCode":"USD"}}}]}}}`,
			want: &domain.Compensation{
				Amounts:  []domain.CompensationAmount{{Kind: "max", Max: floatPtr(2000)}},
				Currency: "USD", Interval: domain.IntervalWeek,
			},
		},
		{
			name: "unknown interval stays explicit",
			body: `{"data":{"jobSearch":{"pageInfo":{"nextCursor":null},"results":[{"job":{"key":"a","title":"Job A","compensation":{"baseSalary":{"unitOfWork":"QUARTER","range":{"min":10,"max":20}},"currencyCode":"USD"}}}]}}}`,
			want: &domain.Compensation{
				Amounts:  []domain.CompensationAmount{{Kind: "range", Min: floatPtr(10), Max: floatPtr(20)}},
				Currency: "USD", Interval: domain.IntervalUnknown,
			},
		},
		{
			name: "empty compensation without a base salary",
			body: `{"data":{"jobSearch":{"pageInfo":{"nextCursor":null},"results":[{"job":{"key":"a","title":"Job A","compensation":{"estimated":null,"baseSalary":null,"currencyCode":"USD"}}}]}}}`,
			want: nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := &recorder{t: t, respond: func(int) *http.Response { return testutil.JSONResponse(http.StatusOK, tc.body) }}
			partition, err := newSource(r).Search(context.Background(), &domain.SearchRequest{Keywords: "engineer"})
			if err != nil {
				t.Fatalf("search: %v", err)
			}
			got := partition.Jobs[0].Compensation
			switch {
			case tc.want == nil && got != nil:
				t.Fatalf("compensation = %+v, want none", got)
			case tc.want != nil && got == nil:
				t.Fatalf("compensation = nil, want %+v", tc.want)
			case tc.want == nil:
				return
			}
			if got.Currency != tc.want.Currency || got.Interval != tc.want.Interval || got.Summary != tc.want.Summary {
				t.Errorf("compensation = %+v, want %+v", got, tc.want)
			}
			if len(got.Amounts) != len(tc.want.Amounts) {
				t.Fatalf("amounts = %+v, want %+v", got.Amounts, tc.want.Amounts)
			}
			for i, want := range tc.want.Amounts {
				if got.Amounts[i].Kind != want.Kind || !sameFloat(got.Amounts[i].Min, want.Min) || !sameFloat(got.Amounts[i].Max, want.Max) {
					t.Errorf("amount[%d] = %+v, want %+v", i, got.Amounts[i], want)
				}
			}
		})
	}
}

func sameFloat(got, want *float64) bool {
	if got == nil || want == nil {
		return got == nil && want == nil
	}
	return *got == *want
}

func TestSearchSendsConfiguredCAMarket(t *testing.T) {
	r := &recorder{t: t, respond: func(int) *http.Response { return fixtureResponse(t, "search.json") }}
	partition, err := newSource(r).Search(context.Background(), &domain.SearchRequest{
		Keywords: "software engineer",
		Location: "Vancouver, BC",
		Country:  "CA",
		Locale:   "en-CA",
	})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	req := r.request
	for _, header := range []struct{ name, want string }{
		{"Indeed-Locale", "en-CA"},
		{"Indeed-Co", "CA"},
		{"Accept-Language", "en-CA,en;q=0.9"},
	} {
		if got := req.Header.Get(header.name); got != header.want {
			t.Errorf("%s = %q, want %q", header.name, got, header.want)
		}
	}
	if len(partition.Jobs) != 3 {
		t.Fatalf("jobs = %d, want 3", len(partition.Jobs))
	}
	for _, job := range partition.Jobs {
		if job.SourceURL != "https://ca.indeed.com/viewjob?jk="+job.SourceJobID {
			t.Errorf("job %s source url = %q, want the ca.indeed.com market host", job.ID, job.SourceURL)
		}
	}
}

func TestSearchDerivesLocaleFromCountryAndCountryFromLocale(t *testing.T) {
	r := &recorder{t: t, respond: func(int) *http.Response { return fixtureResponse(t, "search.json") }}
	if _, err := newSource(r).Search(context.Background(), &domain.SearchRequest{Keywords: "go", Country: "ca"}); err != nil {
		t.Fatalf("search: %v", err)
	}
	if got := r.request.Header.Get("Indeed-Locale"); got != "en-CA" {
		t.Errorf("Indeed-Locale = %q, want the derived en-CA", got)
	}
	if got := r.request.Header.Get("Indeed-Co"); got != "CA" {
		t.Errorf("Indeed-Co = %q, want CA", got)
	}

	r = &recorder{t: t, respond: func(int) *http.Response { return fixtureResponse(t, "search.json") }}
	if _, err := newSource(r).Search(context.Background(), &domain.SearchRequest{Keywords: "go", Locale: "en-CA"}); err != nil {
		t.Fatalf("search: %v", err)
	}
	if got := r.request.Header.Get("Indeed-Co"); got != "CA" {
		t.Errorf("Indeed-Co = %q, want the locale-derived CA", got)
	}
}
