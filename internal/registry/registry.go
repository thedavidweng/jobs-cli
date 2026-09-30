package registry

import (
	"context"
	"net/http"

	"github.com/thedavidweng/jobs-cli/v2/internal/ashby"
	"github.com/thedavidweng/jobs-cli/v2/internal/boards"
	"github.com/thedavidweng/jobs-cli/v2/internal/browserapply"
	"github.com/thedavidweng/jobs-cli/v2/internal/config"
	"github.com/thedavidweng/jobs-cli/v2/internal/cookieimport"
	"github.com/thedavidweng/jobs-cli/v2/internal/domain"
	"github.com/thedavidweng/jobs-cli/v2/internal/errors"
	"github.com/thedavidweng/jobs-cli/v2/internal/greenhouse"
	"github.com/thedavidweng/jobs-cli/v2/internal/icims"
	"github.com/thedavidweng/jobs-cli/v2/internal/indeed"
	"github.com/thedavidweng/jobs-cli/v2/internal/lever"
	"github.com/thedavidweng/jobs-cli/v2/internal/linkedin"
	"github.com/thedavidweng/jobs-cli/v2/internal/linkedin/session"
	"github.com/thedavidweng/jobs-cli/v2/internal/linkedin/voyager"
	"github.com/thedavidweng/jobs-cli/v2/internal/linkedinguest"
	"github.com/thedavidweng/jobs-cli/v2/internal/resolver"
	"github.com/thedavidweng/jobs-cli/v2/internal/smartrecruiters"
	"github.com/thedavidweng/jobs-cli/v2/internal/workday"
)

type Registry struct {
	GuestSources map[domain.Source]domain.SourceAdapter
	AuthSources  map[domain.Source]domain.SourceAdapter
	Resolver     domain.Resolver
	Providers    map[domain.ApplicationProvider]domain.ApplyProvider
	LinkedInAuth *session.Service
}

func New(client *http.Client, cfg *config.Config) *Registry {
	sessions := (*config.SessionStore)(nil)
	if cfg != nil {
		sessions = cfg.SessionStore().WithProfile(cfg.ProfileName)
	}
	r := &Registry{
		GuestSources: map[domain.Source]domain.SourceAdapter{},
		AuthSources:  map[domain.Source]domain.SourceAdapter{},
		Providers:    map[domain.ApplicationProvider]domain.ApplyProvider{},
	}
	r.GuestSources[domain.SourceIndeed] = indeed.NewSource(client)
	r.GuestSources[domain.SourceLinkedIn] = linkedinguest.NewSource(client)
	for _, name := range []domain.Source{domain.SourceYZi, domain.SourceCivicInfo, domain.SourceTransLink} {
		r.GuestSources[name] = boards.New(name, client)
	}
	r.AuthSources[domain.SourceLinkedIn] = voyager.NewSource(client, sessions)
	r.Resolver = resolver.New(client)
	r.Providers[domain.ProviderGreenhouse] = greenhouse.NewProvider(client)
	r.Providers[domain.ProviderLinkedIn] = linkedin.NewProvider(client, sessions)
	r.Providers[domain.ProviderLever] = lever.NewProvider()
	r.Providers[domain.ProviderAshby] = ashby.NewProvider()
	r.Providers[domain.ProviderWorkday] = workday.NewProvider()
	r.Providers[domain.ProviderSmartRecruiters] = smartrecruiters.NewProvider()
	r.Providers[domain.ProviderICIMS] = icims.NewProvider(client)
	for _, name := range []domain.ApplicationProvider{domain.ProviderYZi, domain.ProviderPeopleSoft, domain.ProviderExternal} {
		r.Providers[name] = browserapply.Provider{Name: name}
	}
	r.LinkedInAuth = session.New(sessions, cookieimport.New())
	return r
}

func (r *Registry) Source(name domain.Source, authenticated bool) (domain.SourceAdapter, *errors.Error) {
	if authenticated && r.AuthSources != nil {
		if source, ok := r.AuthSources[name]; ok && source != nil {
			return source, nil
		}
	}
	if r.GuestSources != nil {
		if source, ok := r.GuestSources[name]; ok && source != nil {
			return source, nil
		}
	}
	return nil, errors.New(errors.SourceUnavailable, "source "+string(name)+" is not available", errors.CatNetwork, false, nil)
}

func (r *Registry) Provider(name domain.ApplicationProvider) (domain.ApplyProvider, *errors.Error) {
	if r.Providers != nil {
		if provider, ok := r.Providers[name]; ok && provider != nil {
			return provider, nil
		}
	}
	return nil, errors.New(errors.NativeApplyUnsupported, "no application provider registered for "+string(name), errors.CatAPI, false, nil)
}

func (r *Registry) Resolve(ctx context.Context, rawURL string) (*domain.ApplicationTarget, *errors.Error) {
	if r.Resolver == nil {
		return nil, errors.New(errors.ATSResolutionFailed, "application URL resolution is not available", errors.CatAPI, false, nil)
	}
	target, err := r.Resolver.Resolve(ctx, rawURL)
	if err != nil {
		return nil, errors.From(err)
	}
	return target, nil
}
