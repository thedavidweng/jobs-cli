package icims

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/thedavidweng/jobs-cli/internal/domain"
	"github.com/thedavidweng/jobs-cli/internal/errors"
)

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestInspectExtractsJSONLDAndRequiresBrowser(t *testing.T) {
	client := &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		if r.URL.String() != "https://careers.example.com/job/1" {
			t.Fatal(r.URL)
		}
		body := `<html><script type="application/ld+json">{"@context":"https://schema.org","@type":"JobPosting","title":"Engineer","description":"Build systems","identifier":"req-42","datePosted":"2026-09-25","jobLocation":{"address":{"addressLocality":"Austin","addressRegion":"TX","addressCountry":"US"}}}</script></html>`
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{"Content-Type": []string{"text/html"}}}, nil
	})}
	provider := NewProvider(client)
	req := &domain.InspectRequest{Target: domain.ApplicationTarget{URL: "https://careers.example.com/job/1", Provider: domain.ProviderICIMS}}
	inspection, err := provider.Inspect(context.Background(), req)
	if err != nil {
		t.Fatalf("inspect err = %v", err)
	}
	if inspection.Application.ProviderJobID != "req-42" {
		t.Fatalf("provider job id = %q, want req-42 from JSON-LD identifier", inspection.Application.ProviderJobID)
	}
	if !inspection.Capabilities.BrowserRequired || inspection.Capabilities.NativeSubmit {
		t.Fatalf("capabilities = %#v, want browser required without native submit", inspection.Capabilities)
	}
	if req.Job != (domain.Job{}) {
		t.Fatalf("inspect must not mutate the request job: %+v", req.Job)
	}
}

func TestInspectDrift(t *testing.T) {
	client := &http.Client{Transport: roundTrip(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`<script type="application/ld+json">{broken}</script>`))}, nil
	})}
	_, err := NewProvider(client).Inspect(context.Background(), &domain.InspectRequest{Target: domain.ApplicationTarget{URL: "https://example.com/job"}})
	if err == nil || errors.From(err).Code != errors.APISchemaChanged {
		t.Fatal(err)
	}
}
