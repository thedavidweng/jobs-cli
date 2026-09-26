package lever

import (
	"context"
	"net/http"

	"github.com/thedavidweng/jobs-cli/internal/domain"
	"github.com/thedavidweng/jobs-cli/internal/errors"
)

type Provider struct {
	Client *http.Client
}

func NewProvider(client *http.Client) domain.ApplyProvider {
	return &Provider{Client: client}
}

func (p *Provider) Name() domain.ApplicationProvider {
	return domain.ProviderLever
}

func (p *Provider) Capabilities() domain.Capabilities {
	return domain.Capabilities{
		Inspect:         false,
		Prepare:         false,
		NativeSubmit:    false,
		BrowserRequired: true,
		AuthRequired:    false,
	}
}

func (p *Provider) Inspect(_ context.Context, _ *domain.InspectRequest) (*domain.ApplicationInspection, error) {
	return nil, errors.New(errors.NativeApplyUnsupported, "lever native application inspection is not supported; apply in a browser", errors.CatAPI, false, nil)
}

func (p *Provider) Submit(_ context.Context, _ *domain.SubmitRequest) (*domain.SubmissionResult, error) {
	return nil, errors.New(errors.NativeApplyUnsupported, "lever native application submission is not supported; apply in a browser", errors.CatAPI, false, nil)
}
