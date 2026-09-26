package registry

import (
	"context"
	"net/http"

	"github.com/thedavidweng/jobs-cli/internal/ashby"
	"github.com/thedavidweng/jobs-cli/internal/config"
	"github.com/thedavidweng/jobs-cli/internal/cookieimport"
	"github.com/thedavidweng/jobs-cli/internal/domain"
	"github.com/thedavidweng/jobs-cli/internal/errors"
	"github.com/thedavidweng/jobs-cli/internal/greenhouse"
	"github.com/thedavidweng/jobs-cli/internal/icims"
	"github.com/thedavidweng/jobs-cli/internal/indeed"
	"github.com/thedavidweng/jobs-cli/internal/lever"
	"github.com/thedavidweng/jobs-cli/internal/linkedin"
	"github.com/thedavidweng/jobs-cli/internal/linkedin/session"
	"github.com/thedavidweng/jobs-cli/internal/linkedin/voyager"
	"github.com/thedavidweng/jobs-cli/internal/linkedinguest"
	"github.com/thedavidweng/jobs-cli/internal/resolver"
	"github.com/thedavidweng/jobs-cli/internal/smartrecruiters"
	"github.com/thedavidweng/jobs-cli/internal/workday"
)

type Options struct {
	Client *http.Client
	Config *config.Config
}

type Registry struct {
	GuestSources map[domain.Source]domain.SourceAdapter
	AuthSources  map[domain.Source]domain.SourceAdapter
	Resolver     domain.Resolver
	Providers    map[domain.ApplicationProvider]domain.ApplyProvider
	LinkedInAuth *session.Service
}

func New(opts Options) *Registry {
	sessions := (*config.SessionStore)(nil)
	if opts.Config != nil {
		sessions = opts.Config.SessionStore().WithProfile(opts.Config.ProfileName)
	}
	r := &Registry{
		GuestSources: map[domain.Source]domain.SourceAdapter{},
		AuthSources:  map[domain.Source]domain.SourceAdapter{},
		Providers:    map[domain.ApplicationProvider]domain.ApplyProvider{},
	}
	r.GuestSources[domain.SourceIndeed] = indeed.NewSource(opts.Client)
	r.GuestSources[domain.SourceLinkedIn] = linkedinguest.NewSource(opts.Client)
	r.GuestSources[domain.Source("lever")] = lever.NewSource(opts.Client)
	r.GuestSources[domain.Source("ashby")] = ashby.NewSource(opts.Client)
	r.GuestSources[domain.Source("smartrecruiters")] = smartrecruiters.NewSource(opts.Client)
	r.AuthSources[domain.SourceLinkedIn] = voyager.NewSource(opts.Client, sessions)
	r.Resolver = resolver.New(opts.Client)
	r.Providers[domain.ProviderGreenhouse] = greenhouse.NewProvider(opts.Client)
	r.Providers[domain.ProviderLinkedIn] = linkedin.NewProvider(opts.Client, sessions)
	r.Providers[domain.ProviderLever] = lever.NewProvider(opts.Client)
	r.Providers[domain.ProviderAshby] = ashby.NewProvider(opts.Client)
	r.Providers[domain.ProviderWorkday] = workday.NewProvider(opts.Client)
	r.Providers[domain.ProviderSmartRecruiters] = smartrecruiters.NewProvider(opts.Client)
	r.Providers[domain.ProviderICIMS] = icims.NewProvider(opts.Client)
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
