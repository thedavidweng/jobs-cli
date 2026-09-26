package linkedin

import (
	"context"
	"net/http"
	"testing"

	"github.com/thedavidweng/jobs-cli/internal/config"
	"github.com/thedavidweng/jobs-cli/internal/domain"
	joberrors "github.com/thedavidweng/jobs-cli/internal/errors"
)

type failTransport struct {
	called bool
}

func (t *failTransport) RoundTrip(*http.Request) (*http.Response, error) {
	t.called = true
	return nil, context.Canceled
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
