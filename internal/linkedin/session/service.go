package session

import (
	"context"

	"github.com/thedavidweng/jobs-cli/internal/config"
	"github.com/thedavidweng/jobs-cli/internal/cookieimport"
	"github.com/thedavidweng/jobs-cli/internal/errors"
)

type Service struct {
	Store    *config.SessionStore
	Importer cookieimport.CookieReader
}

func New(store *config.SessionStore, importer cookieimport.CookieReader) *Service {
	return &Service{Store: store, Importer: importer}
}

func (s *Service) Login(ctx context.Context, browser string) (*config.LinkedInSession, error) {
	if s == nil || s.Importer == nil || s.Store == nil {
		return nil, errors.New(errors.InternalError, "LinkedIn session storage is unavailable", errors.CatInternal, false, nil)
	}
	session, err := s.Importer.Import(ctx, browser)
	if err != nil {
		return nil, err
	}
	if session == nil || !session.Complete() {
		return nil, errors.New(errors.LinkedInSessionRequired, "LinkedIn session import did not find both li_at and JSESSIONID cookies", errors.CatAuth, false, nil)
	}
	session.Profile = s.Store.Profile()
	if err := config.ValidateLinkedInSession(session, s.Store.Profile()); err != nil {
		return nil, errors.New(errors.LinkedInSessionRequired, "imported LinkedIn session is invalid: "+err.Error(), errors.CatAuth, false, nil)
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
