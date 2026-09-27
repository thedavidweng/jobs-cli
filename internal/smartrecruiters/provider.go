package smartrecruiters

import (
	"context"

	"github.com/thedavidweng/jobs-cli/internal/domain"
)

type Provider struct{}

func NewProvider() domain.ApplyProvider { return &Provider{} }

func (*Provider) Capabilities() domain.Capabilities {
	return domain.Capabilities{BrowserRequired: true}
}

func (*Provider) Inspect(_ context.Context, req *domain.InspectRequest) (*domain.ApplicationInspection, error) {
	var targetURL string
	if req != nil {
		targetURL = req.Target.URL
	}
	return nil, domain.BrowserRequiredError("smartrecruiters", "inspection", targetURL)
}

func (*Provider) Submit(_ context.Context, req *domain.SubmitRequest) (*domain.SubmissionResult, error) {
	var targetURL string
	if req != nil {
		targetURL = req.Target.URL
	}
	return nil, domain.BrowserRequiredError("smartrecruiters", "submission", targetURL)
}
