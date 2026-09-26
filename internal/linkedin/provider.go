package linkedin

import (
	"context"
	"errors"
	"net/http"

	"github.com/thedavidweng/jobs-cli/internal/config"
	"github.com/thedavidweng/jobs-cli/internal/domain"
	joberrors "github.com/thedavidweng/jobs-cli/internal/errors"
)

type Provider struct {
	Client   *http.Client
	Sessions *config.SessionStore
}

func NewProvider(client *http.Client, sessions *config.SessionStore) domain.ApplyProvider {
	return &Provider{Client: client, Sessions: sessions}
}

func (p *Provider) Name() domain.ApplicationProvider {
	return domain.ProviderLinkedIn
}

func (p *Provider) Capabilities() domain.Capabilities {
	return domain.Capabilities{
		Inspect:         true,
		Prepare:         true,
		NativeSubmit:    false,
		BrowserRequired: false,
		AuthRequired:    true,
	}
}

func (p *Provider) requireSession() *joberrors.Error {
	if p.Sessions == nil {
		return joberrors.New(joberrors.LinkedInSessionRequired, "no LinkedIn session store configured", joberrors.CatAuth, false, nil)
	}
	session, err := p.Sessions.Load()
	if err != nil {
		if errors.Is(err, config.ErrSessionNotFound) {
			return joberrors.New(joberrors.LinkedInSessionRequired, "LinkedIn session required for Easy Apply; run `jobs-cli auth linkedin login`", joberrors.CatAuth, false, nil)
		}
		return joberrors.New(joberrors.InternalError, err.Error(), joberrors.CatInternal, false, err)
	}
	if !session.Complete() {
		return joberrors.New(joberrors.LinkedInSessionRequired, "stored LinkedIn session is incomplete (needs li_at and JSESSIONID); run `jobs-cli auth linkedin login`", joberrors.CatAuth, false, nil)
	}
	return nil
}

func (p *Provider) Inspect(_ context.Context, req *domain.InspectRequest) (*domain.ApplicationInspection, error) {
	if err := p.requireSession(); err != nil {
		return nil, err
	}
	inspection := &domain.ApplicationInspection{
		Provider:           domain.ProviderLinkedIn,
		Application:        req.Target,
		AcceptsResume:      false,
		AcceptsCoverLetter: false,
		Capabilities:       p.Capabilities(),
	}
	inspection.Fingerprint = domain.Fingerprint(&req.Target, inspection.Fields, inspection.Questions)
	return inspection, nil
}

func (p *Provider) Submit(_ context.Context, _ *domain.SubmitRequest) (*domain.SubmissionResult, error) {
	return nil, joberrors.New(joberrors.LinkedInEasyApplyUnverified,
		"LinkedIn Easy Apply submission is disabled until a controlled authenticated integration verification succeeds",
		joberrors.CatAPI, false, nil)
}
