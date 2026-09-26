package resolver

import (
	"context"
	"net/http"
	"strings"

	"github.com/thedavidweng/jobs-cli/internal/domain"
	"github.com/thedavidweng/jobs-cli/internal/errors"
)

type Resolver struct {
	Client *http.Client
}

func New(client *http.Client) domain.Resolver {
	return &Resolver{Client: client}
}

func (r *Resolver) Resolve(_ context.Context, rawURL string) (*domain.ApplicationTarget, error) {
	if strings.TrimSpace(rawURL) == "" {
		return nil, errors.New(errors.ATSResolutionFailed, "no application URL to resolve", errors.CatAPI, false, nil)
	}
	return nil, errors.New(errors.ATSResolutionFailed, "application URL resolution is not implemented yet", errors.CatAPI, false, nil)
}
