package voyager

import (
	"context"
	"errors"
	"net/http"

	"github.com/thedavidweng/jobs-cli/internal/config"
	"github.com/thedavidweng/jobs-cli/internal/domain"
	joberrors "github.com/thedavidweng/jobs-cli/internal/errors"
)

type Source struct {
	Client   *http.Client
	Sessions *config.SessionStore
}

func NewSource(client *http.Client, sessions *config.SessionStore) domain.SourceAdapter {
	return &Source{Client: client, Sessions: sessions}
}

func (s *Source) Name() domain.Source {
	return domain.SourceLinkedIn
}

func (s *Source) requireSession() *joberrors.Error {
	if s.Sessions == nil {
		return joberrors.New(joberrors.LinkedInSessionRequired, "no LinkedIn session store configured", joberrors.CatAuth, false, nil)
	}
	session, err := s.Sessions.Load()
	if err != nil {
		if errors.Is(err, config.ErrSessionNotFound) {
			return joberrors.New(joberrors.LinkedInSessionRequired, "LinkedIn session required for authenticated Voyager access; run `jobs-cli auth linkedin login`", joberrors.CatAuth, false, nil)
		}
		return joberrors.New(joberrors.InternalError, err.Error(), joberrors.CatInternal, false, err)
	}
	if !session.Complete() {
		return joberrors.New(joberrors.LinkedInSessionRequired, "stored LinkedIn session is incomplete (needs li_at and JSESSIONID); run `jobs-cli auth linkedin login`", joberrors.CatAuth, false, nil)
	}
	return nil
}

func (s *Source) Search(_ context.Context, _ *domain.SearchRequest) (*domain.SearchPartition, error) {
	if err := s.requireSession(); err != nil {
		return nil, err
	}
	return nil, joberrors.New(joberrors.SourceUnavailable, "LinkedIn Voyager search is not implemented yet", joberrors.CatNetwork, true, nil)
}

func (s *Source) Detail(_ context.Context, _ *domain.DetailRequest) (*domain.Job, error) {
	if err := s.requireSession(); err != nil {
		return nil, err
	}
	return nil, joberrors.New(joberrors.SourceUnavailable, "LinkedIn Voyager job detail is not implemented yet", joberrors.CatNetwork, true, nil)
}
