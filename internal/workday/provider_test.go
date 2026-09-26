package workday

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/thedavidweng/jobs-cli/internal/domain"
	"github.com/thedavidweng/jobs-cli/internal/errors"
)

func TestCXSRequestAndSchemaDrift(t *testing.T) {
	client := newClient(func(r *http.Request) (*http.Response, error) {
		if r.Method != "POST" || r.URL.Host != "acme.wd5.myworkdayjobs.com" || r.URL.Path != "/wday/cxs/acme/site/jobs" {
			t.Fatalf("request=%s %s", r.Method, r.URL)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		offset, ok := body["offset"].(float64)
		if body["searchText"] != "go" || !ok || offset != 4 {
			t.Fatal(body)
		}
		return jsonResponse(200, `{"total":1,"jobPostings":[{"title":"Engineer","externalPath":"/job/Remote/Engineer_REQ-1","locationsText":"Remote","bulletFields":["REQ-1"]}]}`), nil
	})
	got, err := NewSource(client, "acme", "wd5", "site").Search(context.Background(), &domain.SearchRequest{Keywords: "go", Limit: 10, Offset: 4})
	if err != nil || got.Jobs[0].SourceJobID != "Remote/Engineer_REQ-1" {
		t.Fatalf("got=%+v err=%v", got, err)
	}
	bad := NewSource(newClient(func(*http.Request) (*http.Response, error) { return jsonResponse(200, `{}`), nil }), "a", "wd5", "s")
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

func TestDetailContract(t *testing.T) {
	client := newClient(func(r *http.Request) (*http.Response, error) {
		if r.Method != "GET" || r.URL.Path != "/wday/cxs/acme/site/job/REQ-1" {
			t.Fatalf("request=%s %s", r.Method, r.URL)
		}
		return jsonResponse(200, `{"jobPostingInfo":{"title":"Engineer","location":"Remote","jobDescription":"Build systems","postedOn":"Posted Today","externalPath":"/job/Remote/Engineer_REQ-1"}}`), nil
	})
	job, err := NewSource(client, "acme", "wd5", "site").Detail(context.Background(), &domain.DetailRequest{SourceJobID: "REQ-1"})
	if err != nil || job.Description != "Build systems" {
		t.Fatalf("job=%+v err=%v", job, err)
	}
}
