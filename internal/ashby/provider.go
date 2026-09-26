package ashby

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/thedavidweng/jobs-cli/internal/domain"
	"github.com/thedavidweng/jobs-cli/internal/errors"
)

const baseURL = "https://api.ashbyhq.com"

type Provider struct{ Client *http.Client }

func NewProvider(client *http.Client) domain.ApplyProvider { return &Provider{Client: client} }
func (p *Provider) Name() domain.ApplicationProvider       { return domain.ProviderAshby }
func (p *Provider) Capabilities() domain.Capabilities {
	return domain.Capabilities{BrowserRequired: true}
}

func (p *Provider) Inspect(_ context.Context, req *domain.InspectRequest) (*domain.ApplicationInspection, error) {
	var targetURL string
	if req != nil {
		targetURL = req.Target.URL
	}
	return nil, domain.BrowserRequiredError("ashby", "inspection", targetURL)
}

func (p *Provider) Submit(_ context.Context, req *domain.SubmitRequest) (*domain.SubmissionResult, error) {
	var targetURL string
	if req != nil {
		targetURL = req.Target.URL
	}
	return nil, domain.BrowserRequiredError("ashby", "submission", targetURL)
}

type Source struct {
	client       *http.Client
	organization string
}

func NewSource(client *http.Client, organization ...string) domain.SourceAdapter {
	s := &Source{client: client}
	if len(organization) > 0 {
		s.organization = organization[0]
	}
	return s
}
func (s *Source) Name() domain.Source { return domain.Source("ashby") }
func (s *Source) board(req *domain.SearchRequest) string {
	if s.organization != "" {
		return s.organization
	}
	return req.Keywords
}

func (s *Source) Search(ctx context.Context, req *domain.SearchRequest) (*domain.SearchPartition, error) {
	org := s.board(req)
	if org == "" {
		return nil, schema("missing Ashby organization")
	}
	jobs, err := s.load(ctx, org)
	if err != nil {
		return nil, err
	}
	normalized := make([]domain.Job, 0, len(jobs))
	for i := range jobs {
		job, err := normalize(&jobs[i])
		if err != nil {
			return nil, err
		}
		job.Employer = org
		normalized = append(normalized, job)
	}
	return &domain.SearchPartition{Source: s.Name(), Jobs: normalized, Pagination: &domain.Pagination{Total: len(normalized), Native: &domain.NativePagination{Kind: domain.PaginationNone}}}, nil
}

func (s *Source) Detail(ctx context.Context, req *domain.DetailRequest) (*domain.Job, error) {
	org, id := s.organization, req.SourceJobID
	if a, b, ok := strings.Cut(id, "/"); ok {
		org, id = a, b
	}
	if org == "" || id == "" {
		return nil, schema("missing Ashby organization or job id")
	}
	jobs, err := s.load(ctx, org)
	if err != nil {
		return nil, err
	}
	for i := range jobs {
		if jobs[i].ID != id {
			continue
		}
		job, err := normalize(&jobs[i])
		if err != nil {
			return nil, err
		}
		job.Employer = org
		return &job, nil
	}
	return nil, errors.New(errors.ResourceNotFound, "Ashby job was not found", errors.CatAPI, false, nil)
}

type response struct {
	Jobs []posting `json:"jobs"`
}
type posting struct {
	ID                      string            `json:"id"`
	Title                   string            `json:"title"`
	Department              string            `json:"department"`
	Location                string            `json:"location"`
	IsRemote                bool              `json:"isRemote"`
	WorkplaceType           string            `json:"workplaceType"`
	DescriptionPlain        string            `json:"descriptionPlain"`
	DescriptionHTML         string            `json:"descriptionHtml"`
	ApplyURL                string            `json:"applyUrl"`
	PublishedAt             string            `json:"publishedAt"`
	CompensationTierSummary string            `json:"compensationTierSummary"`
	CompensationTiers       []tier            `json:"compensationTiers"`
	Compensation            *compensationData `json:"compensation"`
	JobURL                  string            `json:"jobUrl"`
}
type compensationData struct {
	CompensationTierSummary string `json:"compensationTierSummary"`
	CompensationTiers       []tier `json:"compensationTiers"`
}
type tier struct {
	Components []component `json:"components"`
}
type component struct {
	CompensationType string   `json:"compensationType"`
	Interval         string   `json:"interval"`
	CurrencyCode     string   `json:"currencyCode"`
	MinValue         *float64 `json:"minValue"`
	MaxValue         *float64 `json:"maxValue"`
	IsEquity         bool     `json:"isEquity"`
	OffersEquity     bool     `json:"offersEquity"`
}

func (s *Source) load(ctx context.Context, org string) ([]posting, error) {
	client := s.client
	if client == nil {
		client = http.DefaultClient
	}
	u := baseURL + "/posting-api/job-board/" + url.PathEscape(org) + "?includeCompensation=true"
	r, err := http.NewRequestWithContext(ctx, http.MethodGet, u, http.NoBody)
	if err != nil {
		return nil, err
	}
	r.Header.Set("Accept", "application/json")
	resp, err := client.Do(r)
	if err != nil {
		return nil, errors.New(errors.NetworkUnreachable, "Ashby request failed", errors.CatNetwork, true, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, errors.New(errors.APIError, fmt.Sprintf("Ashby returned HTTP %d", resp.StatusCode), errors.CatAPI, resp.StatusCode >= 500, nil)
	}
	var body response
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, schema("invalid Ashby response")
	}
	if body.Jobs == nil {
		return nil, schema("Ashby response is missing jobs")
	}
	return body.Jobs, nil
}

func normalize(item *posting) (domain.Job, error) {
	if item.ID == "" || item.Title == "" {
		return domain.Job{}, schema("Ashby job is missing id or title")
	}
	job := domain.NewJob(domain.Source("ashby"), item.ID)
	job.Title, job.Location, job.Remote, job.Description, job.SourceURL, job.ApplicationURL, job.PostedDate = item.Title, item.Location, item.IsRemote, item.DescriptionPlain, item.JobURL, item.ApplyURL, item.PublishedAt
	if job.Description == "" {
		job.Description = item.DescriptionHTML
	}
	switch strings.ToLower(item.WorkplaceType) {
	case "remote":
		job.Workplace = domain.WorkplaceRemote
		job.Remote = true
	case "hybrid":
		job.Workplace = domain.WorkplaceHybrid
	case "onsite", "on-site":
		job.Workplace = domain.WorkplaceOnsite
	}
	if c := compensation(item); c != nil {
		job.Compensation = c
	}
	return job, nil
}

func compensation(item *posting) *domain.Compensation {
	summary, tiers := item.CompensationTierSummary, item.CompensationTiers
	if item.Compensation != nil {
		summary, tiers = item.Compensation.CompensationTierSummary, item.Compensation.CompensationTiers
	}
	c := &domain.Compensation{Summary: summary}
	for _, t := range tiers {
		for _, v := range t.Components {
			if v.MinValue == nil && v.MaxValue == nil && !v.IsEquity && !v.OffersEquity {
				continue
			}
			kind := strings.ToLower(v.CompensationType)
			if v.IsEquity || v.OffersEquity || strings.Contains(kind, "equity") {
				kind = "equity"
			}
			c.Amounts = append(c.Amounts, domain.CompensationAmount{Kind: kind, Min: v.MinValue, Max: v.MaxValue})
			if c.Currency == "" {
				c.Currency = v.CurrencyCode
			}
			if c.Interval == domain.IntervalUnknown || c.Interval == "" {
				c.Interval = interval(v.Interval)
			}
		}
	}
	if len(c.Amounts) == 0 && c.Summary == "" {
		return nil
	}
	return c
}

func interval(value string) domain.CompensationInterval {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "year", "1 year", "annual":
		return domain.IntervalYear
	case "month", "1 month":
		return domain.IntervalMonth
	case "week", "1 week":
		return domain.IntervalWeek
	case "day", "1 day":
		return domain.IntervalDay
	case "hour", "1 hour":
		return domain.IntervalHour
	default:
		return domain.IntervalUnknown
	}
}

func schema(message string) error {
	return errors.New(errors.APISchemaChanged, message, errors.CatAPI, false, nil)
}
