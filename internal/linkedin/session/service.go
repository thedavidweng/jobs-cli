package session

import (
	"context"

	"github.com/thedavidweng/jobs-cli/internal/config"
	"github.com/thedavidweng/jobs-cli/internal/errors"
)

type Importer interface {
	Import(ctx context.Context, browser string) (*config.LinkedInSession, error)
}

type Service struct {
	Store    *config.SessionStore
	Importer Importer
}

func New(store *config.SessionStore, importer Importer) *Service {
	return &Service{Store: store, Importer: importer}
}

func (s *Service) Login(ctx context.Context, browser string) (*config.LinkedInSession, error) {
	if s.Importer == nil {
		return nil, errors.New(errors.NotImplemented, "guided LinkedIn session import is not implemented yet", errors.CatInternal, false, nil)
	}
	session, err := s.Importer.Import(ctx, browser)
	if err != nil {
		return nil, err
	}
	if err := s.Store.Save(session); err != nil {
		return nil, errors.New(errors.InternalError, err.Error(), errors.CatInternal, false, err)
	}
	return session, nil
}

func (s *Service) Logout() (bool, error) {
	return s.Store.Remove()
}

func (s *Service) Status() config.SessionStatus {
	return s.Store.Status()
}
