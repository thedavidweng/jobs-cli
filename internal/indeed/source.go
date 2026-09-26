package indeed

import (
	"context"
	"net/http"

	"github.com/thedavidweng/jobs-cli/internal/domain"
	"github.com/thedavidweng/jobs-cli/internal/errors"
)

type Source struct {
	Client *http.Client
}

func NewSource(client *http.Client) domain.SourceAdapter {
	return &Source{Client: client}
}

func (s *Source) Name() domain.Source {
	return domain.SourceIndeed
}

func (s *Source) Search(_ context.Context, _ *domain.SearchRequest) (*domain.SearchPartition, error) {
	return nil, errors.New(errors.SourceUnavailable, "indeed discovery is not implemented yet", errors.CatNetwork, true, nil)
}

func (s *Source) Detail(_ context.Context, _ *domain.DetailRequest) (*domain.Job, error) {
	return nil, errors.New(errors.SourceUnavailable, "indeed job detail is not implemented yet", errors.CatNetwork, true, nil)
}
