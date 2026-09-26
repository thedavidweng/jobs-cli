package testutil

import (
	"context"

	"github.com/thedavidweng/jobs-cli/internal/domain"
	"github.com/thedavidweng/jobs-cli/internal/registry"
)

type FakeSource struct {
	SourceName domain.Source
	Partition  *domain.SearchPartition
	SearchErr  error
	Job        *domain.Job
	DetailErr  error
	Requests   []domain.SearchRequest
	Details    []domain.DetailRequest
}

func (f *FakeSource) Name() domain.Source {
	return f.SourceName
}

func (f *FakeSource) Search(_ context.Context, req *domain.SearchRequest) (*domain.SearchPartition, error) {
	f.Requests = append(f.Requests, *req)
	if f.SearchErr != nil {
		return nil, f.SearchErr
	}
	if f.Partition != nil {
		return f.Partition, nil
	}
	return &domain.SearchPartition{Source: f.SourceName, Jobs: []domain.Job{}}, nil
}

func (f *FakeSource) Detail(_ context.Context, req *domain.DetailRequest) (*domain.Job, error) {
	f.Details = append(f.Details, *req)
	if f.DetailErr != nil {
		return nil, f.DetailErr
	}
	if f.Job != nil {
		return f.Job, nil
	}
	job := domain.NewJob(f.SourceName, req.SourceJobID)
	job.Title = "Fake Job"
	job.Employer = "Fake Employer"
	return &job, nil
}

type FakeResolver struct {
	Targets map[string]*domain.ApplicationTarget
	Target  *domain.ApplicationTarget
	Err     error
	Calls   []string
}

func (f *FakeResolver) Resolve(_ context.Context, rawURL string) (*domain.ApplicationTarget, error) {
	f.Calls = append(f.Calls, rawURL)
	if f.Err != nil {
		return nil, f.Err
	}
	if f.Targets != nil {
		if target, ok := f.Targets[rawURL]; ok {
			return target, nil
		}
	}
	if f.Target != nil {
		return f.Target, nil
	}
	return &domain.ApplicationTarget{
		URL:      rawURL,
		Provider: domain.ProviderGreenhouse,
		Capabilities: domain.Capabilities{
			Inspect: true, Prepare: true, NativeSubmit: true,
		},
	}, nil
}

type FakeProvider struct {
	ProviderName domain.ApplicationProvider
	Caps         domain.Capabilities
	Inspection   *domain.ApplicationInspection
	InspectErr   error
	SubmitResult *domain.SubmissionResult
	SubmitErr    error
	InspectCalls int
	SubmitCalls  int
	LastRequest  domain.SubmitRequest
}

func (f *FakeProvider) Name() domain.ApplicationProvider {
	return f.ProviderName
}

func (f *FakeProvider) Capabilities() domain.Capabilities {
	if f.Caps == (domain.Capabilities{}) {
		return domain.Capabilities{Inspect: true, Prepare: true, NativeSubmit: true}
	}
	return f.Caps
}

func (f *FakeProvider) Inspect(_ context.Context, req *domain.InspectRequest) (*domain.ApplicationInspection, error) {
	f.InspectCalls++
	if f.InspectErr != nil {
		return nil, f.InspectErr
	}
	if f.Inspection != nil {
		return f.Inspection, nil
	}
	inspection := &domain.ApplicationInspection{
		Provider:     f.ProviderName,
		Application:  req.Target,
		Capabilities: f.Capabilities(),
	}
	inspection.Fingerprint = domain.Fingerprint(&req.Target, inspection.Fields, inspection.Questions)
	return inspection, nil
}

func (f *FakeProvider) Submit(_ context.Context, req *domain.SubmitRequest) (*domain.SubmissionResult, error) {
	f.SubmitCalls++
	f.LastRequest = *req
	if f.SubmitErr != nil {
		return nil, f.SubmitErr
	}
	if f.SubmitResult != nil {
		return f.SubmitResult, nil
	}
	return &domain.SubmissionResult{
		Provider:  f.ProviderName,
		Submitted: true,
		Status:    "submitted",
	}, nil
}

func NewRegistry(sources map[domain.Source]domain.SourceAdapter, resolver domain.Resolver, providers map[domain.ApplicationProvider]domain.ApplyProvider) *registry.Registry {
	return &registry.Registry{
		GuestSources: sources,
		AuthSources:  sources,
		Resolver:     resolver,
		Providers:    providers,
	}
}
