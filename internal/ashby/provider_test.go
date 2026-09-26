package ashby

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/thedavidweng/jobs-cli/internal/domain"
	"github.com/thedavidweng/jobs-cli/internal/errors"
)

func TestBoardCompensationAndSchemaDrift(t *testing.T) {
	client := newClient(func(r *http.Request) (*http.Response, error) {
		if r.Method != "GET" || r.URL.Path != "/posting-api/job-board/acme" || r.URL.Query().Get("includeCompensation") != "true" {
			t.Fatalf("request=%s", r.URL)
		}
		return jsonResponse(200, `{"jobs":[{"id":"a","title":"Engineer","isRemote":true,"compensation":{"compensationTierSummary":"$182K – $202K • Offers Equity","compensationTiers":[{"components":[{"compensationType":"Salary","interval":"1 YEAR","currencyCode":"USD","minValue":182000,"maxValue":202000},{"compensationType":"Equity","offersEquity":true}]}]}}]}`), nil
	})
	got, err := NewSource(client, "acme").Search(context.Background(), &domain.SearchRequest{})
	if err != nil ||
		got.Jobs[0].Employer != "acme" ||
		got.Jobs[0].Compensation.Currency != "USD" ||
		got.Jobs[0].Compensation.Interval != domain.IntervalYear ||
		len(got.Jobs[0].Compensation.Amounts) != 2 ||
		got.Jobs[0].Compensation.Amounts[0].Min == nil ||
		*got.Jobs[0].Compensation.Amounts[0].Min != 182000 ||
		got.Jobs[0].Compensation.Amounts[1].Kind != "equity" {
		t.Fatalf("got=%+v err=%v", got, err)
	}
	bad := NewSource(newClient(func(*http.Request) (*http.Response, error) { return jsonResponse(200, `{}`), nil }), "a")
	_, err = bad.Search(context.Background(), &domain.SearchRequest{})
	if errors.From(err).Code != errors.APISchemaChanged {
		t.Fatal(err)
	}
}

func newClient(f func(*http.Request) (*http.Response, error)) *http.Client {
	return &http.Client{Transport: roundTrip(f)}
}

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func jsonResponse(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{"Content-Type": []string{"application/json"}}}
}

func TestDetailUsesBoard(t *testing.T) {
	client := newClient(func(r *http.Request) (*http.Response, error) {
		return jsonResponse(200, `{"jobs":[{"id":"a","title":"Engineer","descriptionPlain":"Build systems"}]}`), nil
	})
	job, err := NewSource(client, "acme").Detail(context.Background(), &domain.DetailRequest{SourceJobID: "a"})
	if err != nil || job.Description != "Build systems" {
		t.Fatalf("job=%+v err=%v", job, err)
	}
}
