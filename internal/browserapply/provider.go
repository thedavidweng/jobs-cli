package browserapply

import (
	"context"

	"github.com/thedavidweng/jobs-cli/v2/internal/domain"
)

// Provider routes applications that require an external browser.
type Provider struct{ Name domain.ApplicationProvider }

func (p Provider) Capabilities() domain.Capabilities { return domain.CapabilitiesFor(p.Name) }
func (p Provider) Inspect(_ context.Context, req *domain.InspectRequest) (*domain.ApplicationInspection, error) {
	if p.Name == domain.ProviderIndeed {
		target := req.Target
		target.Capabilities = p.Capabilities()
		return &domain.ApplicationInspection{Provider: p.Name, Application: target, Capabilities: p.Capabilities(), PendingAction: "official_handoff", Fingerprint: domain.Fingerprint(&target, nil, nil)}, nil
	}
	return nil, domain.BrowserRequiredError(string(p.Name), "inspection", req.Target.URL)
}

func (p Provider) Submit(_ context.Context, req *domain.SubmitRequest) (*domain.SubmissionResult, error) {
	return nil, domain.BrowserRequiredError(string(p.Name), "submission", req.Target.URL)
}
