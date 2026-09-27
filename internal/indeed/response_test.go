package indeed_test

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/thedavidweng/jobs-cli/internal/domain"
	joberrors "github.com/thedavidweng/jobs-cli/internal/errors"
	"github.com/thedavidweng/jobs-cli/internal/indeed"
	"github.com/thedavidweng/jobs-cli/internal/testutil"
)

func searchKeywords() *domain.SearchRequest {
	return &domain.SearchRequest{Keywords: "software engineer", Location: "Vancouver, BC", Market: searchMarket("CA", "en-CA")}
}

func TestSearchAntiBotHTMLIsAnAPIError(t *testing.T) {
	cases := []struct {
		name   string
		status int
	}{
		{name: "cloudflare challenge on 403", status: http.StatusForbidden},
		{name: "cloudflare challenge on 200", status: http.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := &recorder{t: t, respond: func(int) *http.Response {
				return testutil.HTMLResponse(tc.status, fixture(t, "antibot.html"))
			}}
			partition, err := newSource(r).Search(context.Background(), searchKeywords())
			e := requireErrorCode(t, err, joberrors.APIError)
			if !strings.Contains(strings.ToLower(e.Message), "anti-bot") {
				t.Errorf("message = %q, want an anti-bot explanation", e.Message)
			}
			if partition != nil {
				t.Fatalf("an anti-bot response must not normalize into a partition: %+v", partition)
			}
		})
	}
}

func TestSearchForbiddenJSONResponse(t *testing.T) {
	r := &recorder{t: t, respond: func(int) *http.Response {
		return testutil.JSONResponse(http.StatusForbidden, `{"errors":[{"message":"forbidden"}]}`)
	}}
	_, err := newSource(r).Search(context.Background(), searchKeywords())
	e := requireErrorCode(t, err, joberrors.APIAccessForbidden)
	if e.Retryable {
		t.Error("a forbidden response must not be retryable")
	}
}

func TestSearchRateLimitedRespectsRetryAfter(t *testing.T) {
	r := &recorder{t: t, respond: func(int) *http.Response {
		return &http.Response{
			StatusCode: http.StatusTooManyRequests,
			Header: http.Header{
				"Content-Type": []string{"application/json"},
				"Retry-After":  []string{"30"},
			},
			Body: io.NopCloser(strings.NewReader(`{"errors":[{"message":"rate limited"}]}`)),
		}
	}}
	_, err := newSource(r).Search(context.Background(), searchKeywords())
	e := requireErrorCode(t, err, joberrors.RateLimited)
	if !e.Retryable {
		t.Error("rate limiting must be retryable")
	}
	if e.RetryAfterMS != 30000 {
		t.Errorf("retry after = %dms, want 30000ms", e.RetryAfterMS)
	}
}

func TestSearchServerErrorIsRetryable(t *testing.T) {
	r := &recorder{t: t, respond: func(int) *http.Response {
		return testutil.JSONResponse(http.StatusInternalServerError, `{"errors":[{"message":"boom"}]}`)
	}}
	_, err := newSource(r).Search(context.Background(), searchKeywords())
	e := requireErrorCode(t, err, joberrors.APIError)
	if !e.Retryable {
		t.Error("a 5xx response must be retryable")
	}
}

func TestSearchNonJSONBodyIsAnAPIError(t *testing.T) {
	r := &recorder{t: t, respond: func(int) *http.Response {
		return testutil.JSONResponse(http.StatusOK, "gateway error")
	}}
	partition, err := newSource(r).Search(context.Background(), searchKeywords())
	e := requireErrorCode(t, err, joberrors.APIError)
	if !strings.Contains(e.Message, "non-JSON response") {
		t.Errorf("message = %q, want a non-JSON response explanation", e.Message)
	}
	if partition != nil {
		t.Fatalf("a non-JSON response must not normalize into a partition: %+v", partition)
	}
}

func TestSearchTruncatedJSONIsSchemaDrift(t *testing.T) {
	r := &recorder{t: t, respond: func(int) *http.Response {
		return testutil.JSONResponse(http.StatusOK, `{"data":{"jobSearch":`)
	}}
	_, err := newSource(r).Search(context.Background(), searchKeywords())
	e := requireErrorCode(t, err, joberrors.APISchemaChanged)
	if !strings.Contains(e.Message, "schema changed") {
		t.Errorf("message = %q, want a schema drift explanation", e.Message)
	}
}

func TestSearchNetworkFailures(t *testing.T) {
	source := indeed.NewSource(testutil.NewClient(testutil.RoundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, context.DeadlineExceeded
	})))
	_, err := source.Search(context.Background(), searchKeywords())
	e := requireErrorCode(t, err, joberrors.NetworkTimeout)
	if !e.Retryable {
		t.Error("a network timeout must be retryable")
	}

	source = indeed.NewSource(testutil.NewClient(testutil.RoundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, io.ErrUnexpectedEOF
	})))
	_, err = source.Search(context.Background(), searchKeywords())
	e = requireErrorCode(t, err, joberrors.NetworkUnreachable)
	if !e.Retryable {
		t.Error("a network failure must be retryable")
	}
}

func TestSearchGraphQLError(t *testing.T) {
	r := &recorder{t: t, respond: func(int) *http.Response { return fixtureResponse(t, "graphql_error.json") }}
	_, err := newSource(r).Search(context.Background(), &domain.SearchRequest{
		Keywords: "software engineer",
		Location: "Vancouver, BC",
		Cursor:   "stale-cursor",
		Market:   searchMarket("CA", "en-CA"),
	})
	e := requireErrorCode(t, err, joberrors.APIError)
	if !strings.Contains(e.Message, "Invalid cursor token") {
		t.Errorf("message = %q, want the GraphQL error message", e.Message)
	}
}

func TestSearchSchemaDrift(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{name: "missing jobSearch connection", body: `{"data":{}}`},
		{name: "missing pageInfo object", body: `{"data":{"jobSearch":{"results":[]}}}`},
		{name: "null results array", body: `{"data":{"jobSearch":{"pageInfo":{"nextCursor":null},"results":null}}}`},
		{name: "result without a job node", body: `{"data":{"jobSearch":{"pageInfo":{"nextCursor":null},"results":[{"trackingKey":"t1"}]}}}`},
		{name: "job without a key", body: `{"data":{"jobSearch":{"pageInfo":{"nextCursor":null},"results":[{"job":{"title":"Keyless"}}]}}}`},
		{name: "job without a title", body: `{"data":{"jobSearch":{"pageInfo":{"nextCursor":null},"results":[{"job":{"key":"a1b2c3d4e5f6g7h8"}}]}}}`},
		{name: "data is not an object", body: `{"data":42}`},
		{name: "truncated response", body: `{"data":{"jobSearch":`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := &recorder{t: t, respond: func(int) *http.Response {
				return testutil.JSONResponse(http.StatusOK, tc.body)
			}}
			partition, err := newSource(r).Search(context.Background(), searchKeywords())
			e := requireErrorCode(t, err, joberrors.APISchemaChanged)
			if e.Retryable {
				t.Error("schema drift must not be retryable")
			}
			if partition != nil {
				t.Fatalf("schema drift must not normalize into a partition: %+v", partition)
			}
		})
	}
}

func TestSearchValidatesRequest(t *testing.T) {
	cases := []struct {
		name string
		req  *domain.SearchRequest
	}{
		{name: "no keywords or location", req: &domain.SearchRequest{}},
		{name: "unsupported sort", req: &domain.SearchRequest{Keywords: "go", Sort: "salary"}},
		{name: "offset continuation", req: &domain.SearchRequest{Keywords: "go", Offset: 10}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := &recorder{t: t, respond: func(int) *http.Response { return fixtureResponse(t, "search.json") }}
			_, err := newSource(r).Search(context.Background(), tc.req)
			e := requireErrorCode(t, err, joberrors.InvalidArguments)
			if e.Category != joberrors.CatValidation {
				t.Errorf("category = %s, want validation", e.Category)
			}
			if r.calls != 0 {
				t.Fatalf("an invalid search request still made %d HTTP calls", r.calls)
			}
		})
	}
}
