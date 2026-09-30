package browserapply

import (
	"context"

	"github.com/thedavidweng/jobs-cli/v2/internal/domain"
)

// Provider routes applications that require an external browser.
type Provider struct{ Name domain.ApplicationProvider }

func (p Provider) Capabilities() domain.Capabilities { return domain.CapabilitiesFor(p.Name) }
func (p Provider) Inspect(_ context.Context, req *domain.InspectRequest) (*domain.ApplicationInspection, error) {
	return nil, domain.BrowserRequiredError(string(p.Name), "inspection", req.Target.URL)
}

func (p Provider) Submit(_ context.Context, req *domain.SubmitRequest) (*domain.SubmissionResult, error) {
	return nil, domain.BrowserRequiredError(string(p.Name), "submission", req.Target.URL)
}
