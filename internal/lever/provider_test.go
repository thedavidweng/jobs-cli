package lever

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/thedavidweng/jobs-cli/internal/domain"
	"github.com/thedavidweng/jobs-cli/internal/errors"
)

func TestSearchContractAndSchemaDrift(t *testing.T) {
	client := newClient(func(r *http.Request) (*http.Response, error) {
		if r.Method != "GET" || r.URL.Host != "api.lever.co" || r.URL.Path != "/v0/postings/acme" || r.URL.Query().Get("limit") != "2" || r.URL.Query().Get("skip") != "3" {
			t.Fatalf("request=%s", r.URL)
		}
		return jsonResponse(200, `[{"id":"a","text":"Engineer","categories":{"location":"Remote"},"workplaceType":"remote","hostedUrl":"https://jobs.lever.co/acme/a","applyUrl":"https://jobs.lever.co/acme/a/apply"}]`), nil
	})
	got, err := NewSource(client, "acme").Search(context.Background(), &domain.SearchRequest{Limit: 2, Offset: 3})
	if err != nil || len(got.Jobs) != 1 || !got.Jobs[0].Remote {
		t.Fatalf("got=%+v err=%v", got, err)
	}
	bad := NewSource(newClient(func(*http.Request) (*http.Response, error) { return jsonResponse(200, `[{"id":"a"}]`), nil }), "acme")
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
		if r.URL.Path != "/v0/postings/acme/a" {
			t.Fatal(r.URL)
		}
		return jsonResponse(200, `{"id":"a","text":"Engineer","descriptionPlain":"Build systems","applyUrl":"https://jobs.lever.co/acme/a/apply"}`), nil
	})
	job, err := NewSource(client, "acme").Detail(context.Background(), &domain.DetailRequest{SourceJobID: "a"})
	if err != nil || job.Description != "Build systems" {
		t.Fatalf("job=%+v err=%v", job, err)
	}
}
