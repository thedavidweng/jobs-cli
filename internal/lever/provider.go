package lever

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

const baseURL = "https://api.lever.co"

type Provider struct {
	Client  *http.Client
	Company string
}

func NewProvider(client *http.Client) domain.ApplyProvider { return &Provider{Client: client} }

type Source struct {
	client  *http.Client
	company string
}

func NewSource(client *http.Client, company ...string) domain.SourceAdapter {
	s := &Source{client: client}
	if len(company) > 0 {
		s.company = company[0]
	}
	return s
}
func (s *Source) Name() domain.Source { return domain.Source("lever") }
func (s *Source) Search(ctx context.Context, req *domain.SearchRequest) (*domain.SearchPartition, error) {
	return (&Provider{Client: s.client, Company: s.company}).Search(ctx, req)
}

func (s *Source) Detail(ctx context.Context, req *domain.DetailRequest) (*domain.Job, error) {
	return (&Provider{Client: s.client, Company: s.company}).Detail(ctx, req)
}
func (p *Provider) Name() domain.ApplicationProvider { return domain.ProviderLever }
func (p *Provider) Capabilities() domain.Capabilities {
	return domain.Capabilities{BrowserRequired: true}
}

func (p *Provider) Inspect(_ context.Context, req *domain.InspectRequest) (*domain.ApplicationInspection, error) {
	var targetURL string
	if req != nil {
		targetURL = req.Target.URL
	}
	return nil, domain.BrowserRequiredError("lever", "inspection", targetURL)
}

func (p *Provider) Submit(_ context.Context, req *domain.SubmitRequest) (*domain.SubmissionResult, error) {
	var targetURL string
	if req != nil {
		targetURL = req.Target.URL
	}
	return nil, domain.BrowserRequiredError("lever", "submission", targetURL)
}

func (p *Provider) sourceCompany(req *domain.SearchRequest) string {
	if p.Company != "" {
		return p.Company
	}
	return req.Keywords
}

func (p *Provider) Search(ctx context.Context, req *domain.SearchRequest) (*domain.SearchPartition, error) {
	company := p.sourceCompany(req)
	if company == "" {
		return nil, schema("missing Lever company")
	}
	limit := req.Limit
	if limit <= 0 {
		limit = 20
	}
	u, _ := url.Parse(baseURL + "/v0/postings/" + url.PathEscape(company))
	q := u.Query()
	q.Set("limit", fmt.Sprint(limit))
	q.Set("skip", fmt.Sprint(req.Offset))
	u.RawQuery = q.Encode()
	var postings []posting
	if err := p.get(ctx, u.String(), &postings); err != nil {
		return nil, err
	}
	if postings == nil {
		return nil, schema("Lever response is not a postings array")
	}
	jobs := make([]domain.Job, 0, len(postings))
	for i := range postings {
		job, err := normalize(company, &postings[i])
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	return &domain.SearchPartition{Source: domain.Source("lever"), Jobs: jobs, Pagination: &domain.Pagination{Limit: limit, Offset: req.Offset, HasMore: len(jobs) == limit, Native: &domain.NativePagination{Kind: domain.PaginationLimitSkip, Limit: limit, Skip: req.Offset}}}, nil
}

func (p *Provider) Detail(ctx context.Context, req *domain.DetailRequest) (*domain.Job, error) {
	company, id := p.Company, req.SourceJobID
	if a, b, ok := strings.Cut(id, "/"); ok {
		company, id = a, b
	}
	if company == "" || id == "" {
		return nil, schema("missing Lever company or posting id")
	}
	var item posting
	if err := p.get(ctx, baseURL+"/v0/postings/"+url.PathEscape(company)+"/"+url.PathEscape(id), &item); err != nil {
		return nil, err
	}
	job, err := normalize(company, &item)
	if err != nil {
		return nil, err
	}
	return &job, nil
}

type posting struct {
	ID         string `json:"id"`
	Text       string `json:"text"`
	Categories struct {
		Location string `json:"location"`
	} `json:"categories"`
	WorkplaceType    string `json:"workplaceType"`
	HostedURL        string `json:"hostedUrl"`
	ApplyURL         string `json:"applyUrl"`
	DescriptionPlain string `json:"descriptionPlain"`
	Description      string `json:"description"`
}

func normalize(company string, item *posting) (domain.Job, error) {
	if item.ID == "" || item.Text == "" {
		return domain.Job{}, schema("Lever posting is missing id or text")
	}
	job := domain.NewJob(domain.Source("lever"), item.ID)
	job.Title, job.Employer, job.Location, job.SourceURL, job.ApplicationURL = item.Text, company, item.Categories.Location, item.HostedURL, item.ApplyURL
	job.Description = item.DescriptionPlain
	if job.Description == "" {
		job.Description = item.Description
	}
	switch strings.ToLower(item.WorkplaceType) {
	case "remote":
		job.Workplace, job.Remote = domain.WorkplaceRemote, true
	case "hybrid":
		job.Workplace = domain.WorkplaceHybrid
	case "onsite", "on-site":
		job.Workplace = domain.WorkplaceOnsite
	}
	return job, nil
}

func (p *Provider) get(ctx context.Context, raw string, out any) error {
	client := p.Client
	if client == nil {
		client = http.DefaultClient
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, http.NoBody)
	if err != nil {
		return err
	}
	request.Header.Set("Accept", "application/json")
	response, err := client.Do(request)
	if err != nil {
		return errors.New(errors.NetworkUnreachable, "Lever request failed", errors.CatNetwork, true, err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return errors.New(errors.APIError, fmt.Sprintf("Lever returned HTTP %d", response.StatusCode), errors.CatAPI, response.StatusCode >= 500, nil)
	}
	if err := json.NewDecoder(response.Body).Decode(out); err != nil {
		return schema("invalid Lever response")
	}
	return nil
}

func schema(message string) error {
	return errors.New(errors.APISchemaChanged, message, errors.CatAPI, false, nil)
}
