package linkedin

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/thedavidweng/jobs-cli/v2/internal/config"
	"github.com/thedavidweng/jobs-cli/v2/internal/domain"
	joberrors "github.com/thedavidweng/jobs-cli/v2/internal/errors"
)

type failTransport struct {
	called bool
}

func (t *failTransport) RoundTrip(*http.Request) (*http.Response, error) {
	t.called = true
	return nil, context.Canceled
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func storedSession(t *testing.T) *config.SessionStore {
	t.Helper()
	store := config.NewSessionStore(t.TempDir())
	if err := store.Save(&config.LinkedInSession{Cookies: map[string]string{
		config.CookieLiAt: "secret", config.CookieJSessionID: `"ajax:123"`,
	}}); err != nil {
		t.Fatal(err)
	}
	return store
}

func TestSubmitIsGatedWithoutNetworkCall(t *testing.T) {
	transport := &failTransport{}
	provider := &Provider{
		Client:   &http.Client{Transport: transport},
		Sessions: config.NewSessionStore(t.TempDir()),
	}
	_, err := provider.Submit(context.Background(), &domain.SubmitRequest{})
	if got := joberrors.From(err).Code; got != joberrors.LinkedInEasyApplyUnverified {
		t.Fatalf("error code = %s", got)
	}
	if transport.called {
		t.Fatal("Submit performed a network call")
	}
}

func TestInspectReturnsBrowserRequiredWhenEasyApplyUnavailable(t *testing.T) {
	store := storedSession(t)
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		body := `{"data":{"jobsDashOnsiteApplyApplicationByJobPosting":{"elements":[{"jobSeekerApplicationDetail":{"onsiteApply":false}}]}}}`
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}
	provider := NewProvider(client, store)
	_, err := provider.Inspect(context.Background(), &domain.InspectRequest{
		Job:    domain.Job{ID: "linkedin:42", SourceJobID: "42"},
		Target: domain.ApplicationTarget{URL: "https://www.linkedin.com/jobs/view/42", Provider: domain.ProviderLinkedIn},
	})
	e := joberrors.From(err)
	if e == nil || e.Code != joberrors.BrowserRequired {
		t.Fatalf("error = %v, want BROWSER_REQUIRED", err)
	}
	if !strings.Contains(err.Error(), "https://www.linkedin.com/jobs/view/42") {
		t.Fatalf("error does not return the LinkedIn job URL: %v", err)
	}
}

func TestInspectKeepsEasyApplyFlowWhenAvailable(t *testing.T) {
	store := storedSession(t)
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		body := `{"data":{"jobsDashOnsiteApplyApplicationByJobPosting":{"elements":[{"jobSeekerApplicationDetail":{"onsiteApply":true,"resume":{"name":"resume.pdf"}}}]}}}`
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}
	provider := NewProvider(client, store)
	inspection, err := provider.Inspect(context.Background(), &domain.InspectRequest{
		Job:    domain.Job{ID: "linkedin:42", SourceJobID: "42"},
		Target: domain.ApplicationTarget{URL: "https://www.linkedin.com/jobs/view/42", Provider: domain.ProviderLinkedIn},
	})
	if err != nil {
		t.Fatal(err)
	}
	if inspection.Provider != domain.ProviderLinkedIn || !inspection.AcceptsResume {
		t.Fatalf("inspection = %+v, want the native Easy Apply inspection", inspection)
	}
}
