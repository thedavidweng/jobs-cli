package indeed

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/thedavidweng/jobs-cli/internal/domain"
	joberrors "github.com/thedavidweng/jobs-cli/internal/errors"
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

func (s *Source) Search(ctx context.Context, req *domain.SearchRequest) (*domain.SearchPartition, error) {
	if err := validateSearch(req); err != nil {
		return nil, err
	}
	body, err := s.call(ctx, searchQuery(req))
	if err != nil {
		return nil, err
	}
	jobs, nextCursor, perr := parseSearchResponse(body)
	if perr != nil {
		return nil, perr
	}
	return &domain.SearchPartition{
		Source: domain.SourceIndeed,
		Jobs:   jobs,
		Pagination: &domain.Pagination{
			Limit:      searchLimit(req),
			HasMore:    nextCursor != "",
			NextCursor: nextCursor,
			Native:     &domain.NativePagination{Kind: domain.PaginationCursor, Cursor: nextCursor},
		},
	}, nil
}

func (s *Source) Detail(ctx context.Context, req *domain.DetailRequest) (*domain.Job, error) {
	if req == nil || strings.TrimSpace(req.SourceJobID) == "" {
		return nil, invalidArguments("a source job ID is required")
	}
	key := strings.TrimPrefix(strings.TrimSpace(req.SourceJobID), string(domain.SourceIndeed)+":")
	body, err := s.call(ctx, detailQuery(key))
	if err != nil {
		return nil, err
	}
	job, perr := parseDetailResponse(body, key)
	if perr != nil {
		return nil, perr
	}
	return job, nil
}

func validateSearch(req *domain.SearchRequest) *joberrors.Error {
	if req == nil {
		return invalidArguments("a search request is required")
	}
	if strings.TrimSpace(req.Keywords) == "" && strings.TrimSpace(req.Location) == "" {
		return invalidArguments("provide keywords and/or a location")
	}
	if req.Offset > 0 {
		return invalidArguments("indeed paginates with a cursor; pass a cursor instead of an offset")
	}
	if _, ok := sortArgument(req.Sort); !ok {
		return invalidArguments(fmt.Sprintf("unsupported sort %q for indeed; supported values: relevance, date", req.Sort))
	}
	return nil
}

func invalidArguments(message string) *joberrors.Error {
	return joberrors.New(joberrors.InvalidArguments, message, joberrors.CatValidation, false, nil)
}
