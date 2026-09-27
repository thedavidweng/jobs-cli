package linkedin

import (
	"context"
	"net/http"

	"github.com/thedavidweng/jobs-cli/internal/config"
	"github.com/thedavidweng/jobs-cli/internal/domain"
	joberrors "github.com/thedavidweng/jobs-cli/internal/errors"
	"github.com/thedavidweng/jobs-cli/internal/linkedin/voyager"
)

type Provider struct {
	Client   *http.Client
	Sessions *config.SessionStore
}

func NewProvider(client *http.Client, sessions *config.SessionStore) domain.ApplyProvider {
	return &Provider{Client: client, Sessions: sessions}
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

func (p *Provider) Inspect(ctx context.Context, req *domain.InspectRequest) (*domain.ApplicationInspection, error) {
	id := req.Target.ProviderJobID
	if id == "" {
		id = req.Job.SourceJobID
	}
	easyApply, err := voyager.InspectEasyApply(ctx, p.Client, p.Sessions, id)
	if err != nil {
		return nil, err
	}
	if !easyApply.Available {
		return nil, domain.BrowserRequiredError("linkedin", "inspection", req.Target.URL)
	}
	inspection := &domain.ApplicationInspection{
		Provider:           domain.ProviderLinkedIn,
		Application:        req.Target,
		Fields:             easyApply.Fields,
		Questions:          easyApply.Questions,
		AcceptsResume:      easyApply.AcceptsResume,
		AcceptsCoverLetter: easyApply.AcceptsCoverLetter,
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
