package smartrecruiters

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/thedavidweng/jobs-cli/internal/domain"
	"github.com/thedavidweng/jobs-cli/internal/errors"
)

func TestPostingsPaginationAndSchemaDrift(t *testing.T) {
	client := newClient(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/v1/companies/acme/postings" || r.URL.Query().Get("offset") != "2" || r.URL.Query().Get("limit") != "5" {
			t.Fatalf("request=%s", r.URL)
		}
		return jsonResponse(200, `{"totalFound":9,"content":[{"id":"a","name":"Engineer","location":{"city":"Austin","region":"TX","country":"US"}}]}`), nil
	})
	got, err := NewSource(client, "acme").Search(context.Background(), &domain.SearchRequest{Limit: 5, Offset: 2})
	if err != nil || got.Pagination.Total != 9 || got.Jobs[0].Location != "Austin, TX, US" {
		t.Fatalf("got=%+v err=%v", got, err)
	}
	bad := NewSource(newClient(func(*http.Request) (*http.Response, error) {
		return jsonResponse(200, `{"content":[{}]}`), nil
	}), "a")
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
		if r.URL.Path != "/v1/companies/acme/postings/a" {
			t.Fatal(r.URL)
		}
		return jsonResponse(200, `{"id":"a","name":"Engineer","jobAd":{"sections":{"jobDescription":{"text":"Build systems"}}}}`), nil
	})
	job, err := NewSource(client, "acme").Detail(context.Background(), &domain.DetailRequest{SourceJobID: "a"})
	if err != nil || job.Description != "Build systems" {
		t.Fatalf("job=%+v err=%v", job, err)
	}
}
