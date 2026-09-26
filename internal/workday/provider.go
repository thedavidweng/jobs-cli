package workday

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/thedavidweng/jobs-cli/internal/domain"
	"github.com/thedavidweng/jobs-cli/internal/errors"
)

type Provider struct{ Client *http.Client }

func NewProvider(c *http.Client) domain.ApplyProvider { return &Provider{Client: c} }
func (p *Provider) Name() domain.ApplicationProvider  { return domain.ProviderWorkday }
func (p *Provider) Capabilities() domain.Capabilities {
	return domain.Capabilities{BrowserRequired: true}
}

func (p *Provider) Inspect(_ context.Context, req *domain.InspectRequest) (*domain.ApplicationInspection, error) {
	var targetURL string
	if req != nil {
		targetURL = req.Target.URL
	}
	return nil, domain.BrowserRequiredError("workday", "inspection", targetURL)
}

func (p *Provider) Submit(_ context.Context, req *domain.SubmitRequest) (*domain.SubmissionResult, error) {
	var targetURL string
	if req != nil {
		targetURL = req.Target.URL
	}
	return nil, domain.BrowserRequiredError("workday", "submission", targetURL)
}

type Source struct {
	client            *http.Client
	tenant, sub, site string
}

func NewSource(c *http.Client, parts ...string) domain.SourceAdapter {
	s := &Source{client: c}
	if len(parts) > 0 {
		s.tenant = parts[0]
	}
	if len(parts) > 1 {
		s.sub = parts[1]
	}
	if len(parts) > 2 {
		s.site = parts[2]
	}
	return s
}
func (s *Source) Name() domain.Source { return domain.Source("workday") }
func (s *Source) Search(ctx context.Context, r *domain.SearchRequest) (*domain.SearchPartition, error) {
	if s.tenant == "" || s.sub == "" || s.site == "" {
		return nil, schema("missing Workday tenant, subdomain, or site")
	}
	limit := r.Limit
	if limit <= 0 {
		limit = 20
	}
	body := map[string]any{"appliedFacets": map[string]any{}, "limit": limit, "offset": r.Offset, "searchText": r.Keywords}
	var response searchResponse
	if err := s.request(ctx, http.MethodPost, "jobs", body, &response); err != nil {
		return nil, err
	}
	if response.JobPostings == nil {
		return nil, schema("Workday response is missing jobPostings")
	}
	jobs := make([]domain.Job, 0, len(response.JobPostings))
	for i := range response.JobPostings {
		p := &response.JobPostings[i]
		j, err := normalizePosting(p)
		if err != nil {
			return nil, err
		}
		j.SourceURL = s.base() + "/" + s.site + p.ExternalPath
		j.ApplicationURL = j.SourceURL
		jobs = append(jobs, j)
	}
	return &domain.SearchPartition{Source: s.Name(), Jobs: jobs, Pagination: &domain.Pagination{Limit: limit, Offset: r.Offset, Total: response.Total, HasMore: r.Offset+len(jobs) < response.Total, Native: &domain.NativePagination{Kind: domain.PaginationOffset, Limit: limit, Offset: r.Offset}}}, nil
}

func (s *Source) Detail(ctx context.Context, r *domain.DetailRequest) (*domain.Job, error) {
	id := r.SourceJobID
	if s.tenant == "" || s.sub == "" || s.site == "" || id == "" {
		return nil, schema("missing Workday tenant, subdomain, site, or job id")
	}
	var detail jobDetail
	if err := s.request(ctx, http.MethodGet, "job/"+escapePath(id), nil, &detail); err != nil {
		return nil, err
	}
	if detail.JobPostingInfo.Title == "" {
		return nil, schema("Workday detail is missing jobPostingInfo.title")
	}
	j := domain.NewJob(s.Name(), id)
	j.Title = detail.JobPostingInfo.Title
	j.Location = detail.JobPostingInfo.Location
	j.Description = detail.JobPostingInfo.JobDescription
	j.PostedDate = detail.JobPostingInfo.PostedOn
	j.SourceURL = s.base() + "/" + s.site + detail.JobPostingInfo.ExternalPath
	j.ApplicationURL = j.SourceURL
	return &j, nil
}

type searchResponse struct {
	Total       int          `json:"total"`
	JobPostings []jobPosting `json:"jobPostings"`
}
type jobPosting struct {
	Title         string   `json:"title"`
	ExternalPath  string   `json:"externalPath"`
	LocationsText string   `json:"locationsText"`
	PostedOn      string   `json:"postedOn"`
	BulletFields  []string `json:"bulletFields"`
}
type jobDetail struct {
	JobPostingInfo struct {
		Title          string `json:"title"`
		Location       string `json:"location"`
		JobDescription string `json:"jobDescription"`
		PostedOn       string `json:"postedOn"`
		ExternalPath   string `json:"externalPath"`
	} `json:"jobPostingInfo"`
}

func normalizePosting(p *jobPosting) (domain.Job, error) {
	if p.Title == "" || p.ExternalPath == "" {
		return domain.Job{}, schema("Workday posting is missing title or externalPath")
	}
	id := strings.TrimPrefix(p.ExternalPath, "/")
	id = strings.TrimPrefix(id, "job/")
	j := domain.NewJob(domain.Source("workday"), id)
	j.Title, j.Location, j.PostedDate = p.Title, p.LocationsText, p.PostedOn
	j.SourceURL = p.ExternalPath
	return j, nil
}
func (s *Source) base() string { return "https://" + s.tenant + "." + s.sub + ".myworkdayjobs.com" }
func (s *Source) request(ctx context.Context, method, path string, body, out any) error {
	var b *bytes.Reader
	if body == nil {
		b = bytes.NewReader(nil)
	} else {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		b = bytes.NewReader(data)
	}
	r, err := http.NewRequestWithContext(ctx, method, s.base()+"/wday/cxs/"+url.PathEscape(s.tenant)+"/"+url.PathEscape(s.site)+"/"+path, b)
	if err != nil {
		return err
	}
	r.Header.Set("Accept", "application/json")
	if method == http.MethodPost {
		r.Header.Set("Content-Type", "application/json")
	}
	c := s.client
	if c == nil {
		c = http.DefaultClient
	}
	resp, err := c.Do(r)
	if err != nil {
		return errors.New(errors.NetworkUnreachable, "Workday request failed", errors.CatNetwork, true, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return errors.New(errors.APIError, fmt.Sprintf("Workday returned HTTP %d", resp.StatusCode), errors.CatAPI, resp.StatusCode >= 500, nil)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return schema("invalid Workday response")
	}
	return nil
}

func escapePath(value string) string {
	parts := strings.Split(strings.TrimPrefix(value, "/job/"), "/")
	for i := range parts {
		parts[i] = url.PathEscape(parts[i])
	}
	return strings.Join(parts, "/")
}

func schema(m string) error { return errors.New(errors.APISchemaChanged, m, errors.CatAPI, false, nil) }
