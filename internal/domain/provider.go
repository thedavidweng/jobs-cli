package domain

import "context"

type SourceAdapter interface {
	Search(ctx context.Context, req *SearchRequest) (*SearchPartition, error)
	Detail(ctx context.Context, req *DetailRequest) (*Job, error)
}

type Resolver interface {
	Resolve(ctx context.Context, rawURL string) (*ApplicationTarget, error)
}

type InspectRequest struct {
	Job    Job
	Target ApplicationTarget
}

type SubmitRequest struct {
	Job      Job
	Target   ApplicationTarget
	Artifact ApplicationArtifact
}

type ApplyProvider interface {
	Capabilities() Capabilities
	Inspect(ctx context.Context, req *InspectRequest) (*ApplicationInspection, error)
	Submit(ctx context.Context, req *SubmitRequest) (*SubmissionResult, error)
}
