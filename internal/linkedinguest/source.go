package linkedinguest

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/thedavidweng/jobs-cli/internal/httpclient"

	"golang.org/x/net/html"

	"github.com/thedavidweng/jobs-cli/internal/domain"
	joberrors "github.com/thedavidweng/jobs-cli/internal/errors"
)

const (
	baseURL      = "https://www.linkedin.com"
	searchPath   = "/jobs-guest/jobs/api/seeMoreJobPostings/search"
	maxBodyBytes = 4 << 20
)

var requestHeaders = map[string]string{
	"accept":                    "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7",
	"accept-language":           "en-US,en;q=0.9",
	"cache-control":             "max-age=0",
	"upgrade-insecure-requests": "1",
	"user-agent":                "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
}

type Source struct {
	Client *http.Client
}

func NewSource(client *http.Client) domain.SourceAdapter {
	return &Source{Client: client}
}

func (s *Source) Name() domain.Source {
	return domain.SourceLinkedIn
}

func (s *Source) Search(ctx context.Context, req *domain.SearchRequest) (*domain.SearchPartition, error) {
	if req == nil {
		return nil, joberrors.New(joberrors.ValidationFailed, "a search request is required", joberrors.CatValidation, false, nil)
	}
	start, aerr := startOffset(req)
	if aerr != nil {
		return nil, aerr
	}
	body, aerr := s.fetch(ctx, searchURL(req, start))
	if aerr != nil {
		return nil, aerr
	}
	if len(bytes.TrimSpace(body)) == 0 {
		return partition(start, 0, nil), nil
	}
	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return nil, schemaChanged("LinkedIn Guest returned unparsable HTML")
	}
	cards := parseCards(doc)
	if len(cards) == 0 {
		return nil, schemaChanged("LinkedIn Guest returned HTML without job cards")
	}
	jobs := make([]domain.Job, 0, len(cards))
	seen := make(map[string]struct{}, len(cards))
	for _, card := range cards {
		job, aerr := jobFromCard(card)
		if aerr != nil {
			return nil, aerr
		}
		if _, ok := seen[job.SourceJobID]; ok {
			continue
		}
		seen[job.SourceJobID] = struct{}{}
		jobs = append(jobs, job)
	}
	if req.Limit > 0 && len(jobs) > req.Limit {
		jobs = jobs[:req.Limit]
	}
	return partition(start, len(cards), jobs), nil
}

func (s *Source) Detail(_ context.Context, _ *domain.DetailRequest) (*domain.Job, error) {
	return nil, joberrors.New(joberrors.SourceUnavailable, "LinkedIn Guest discovery has no job detail surface", joberrors.CatNetwork, false, nil)
}

func (s *Source) fetch(ctx context.Context, rawURL string) ([]byte, *joberrors.Error) {
	client := s.Client
	if client == nil {
		client = http.DefaultClient
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, http.NoBody)
	if err != nil {
		return nil, joberrors.New(joberrors.InternalError, "build LinkedIn Guest request: "+err.Error(), joberrors.CatInternal, false, err)
	}
	for key, value := range requestHeaders {
		httpReq.Header.Set(key, value)
	}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, transportError(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, statusError(resp)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		return nil, joberrors.New(joberrors.NetworkUnreachable, "read LinkedIn Guest response failed", joberrors.CatNetwork, true, err)
	}
	return body, nil
}

func startOffset(req *domain.SearchRequest) (int, *joberrors.Error) {
	if req.Cursor != "" && req.Offset == 0 {
		start, err := strconv.Atoi(strings.TrimSpace(req.Cursor))
		if err != nil || start < 0 {
			return 0, joberrors.New(joberrors.ValidationFailed, "LinkedIn Guest cursor must be a non-negative start offset", joberrors.CatValidation, false, nil)
		}
		return start, nil
	}
	if req.Offset < 0 {
		return 0, joberrors.New(joberrors.ValidationFailed, "LinkedIn Guest offset must not be negative", joberrors.CatValidation, false, nil)
	}
	return req.Offset, nil
}

func searchURL(req *domain.SearchRequest, start int) string {
	values := url.Values{}
	if req.Keywords != "" {
		values.Set("keywords", req.Keywords)
	}
	if req.Location != "" {
		values.Set("location", req.Location)
	}
	if req.Radius > 0 {
		values.Set("distance", strconv.Itoa(req.Radius))
	}
	if req.Remote != nil && *req.Remote {
		values.Set("f_WT", "2")
	}
	values.Set("pageNum", "0")
	values.Set("start", strconv.Itoa(start))
	return baseURL + searchPath + "?" + values.Encode()
}

func partition(start, cards int, jobs []domain.Job) *domain.SearchPartition {
	if jobs == nil {
		jobs = []domain.Job{}
	}
	hasMore := cards > 0
	next := start + cards
	pagination := &domain.Pagination{
		Limit:   len(jobs),
		Offset:  start,
		HasMore: hasMore,
		Native:  &domain.NativePagination{Kind: domain.PaginationStartCount, Start: start, Count: cards},
	}
	if hasMore {
		pagination.NextCursor = strconv.Itoa(next)
	}
	return &domain.SearchPartition{Source: domain.SourceLinkedIn, Jobs: jobs, Pagination: pagination}
}

func statusError(resp *http.Response) *joberrors.Error {
	switch resp.StatusCode {
	case http.StatusTooManyRequests:
		return joberrors.NewWithRetryAfter(joberrors.RateLimited, "LinkedIn Guest rate limited the request", joberrors.CatNetwork, true, httpclient.RetryAfter(resp, 0), nil)
	case http.StatusUnauthorized, http.StatusForbidden:
		return joberrors.New(joberrors.APIAccessForbidden, "LinkedIn Guest denied the anonymous request", joberrors.CatAPI, false, nil)
	}
	message := fmt.Sprintf("LinkedIn Guest returned HTTP %d", resp.StatusCode)
	if resp.StatusCode >= 500 {
		return joberrors.New(joberrors.APIError, message, joberrors.CatAPI, true, nil)
	}
	return joberrors.New(joberrors.APIError, message, joberrors.CatAPI, false, nil)
}

func transportError(err error) *joberrors.Error {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return joberrors.New(joberrors.NetworkTimeout, "LinkedIn Guest request timed out", joberrors.CatNetwork, true, err)
	case errors.Is(err, context.Canceled):
		return joberrors.New(joberrors.NetworkUnreachable, "LinkedIn Guest request was canceled", joberrors.CatNetwork, true, err)
	default:
		return joberrors.New(joberrors.NetworkUnreachable, "LinkedIn Guest request failed", joberrors.CatNetwork, true, err)
	}
}

func schemaChanged(message string) *joberrors.Error {
	return joberrors.New(joberrors.APISchemaChanged, message, joberrors.CatAPI, false, nil)
}
