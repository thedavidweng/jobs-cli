package smartrecruiters

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

const baseURL = "https://api.smartrecruiters.com/v1"

type Provider struct{ Client *http.Client }

func NewProvider(c *http.Client) domain.ApplyProvider { return &Provider{Client: c} }
func (p *Provider) Name() domain.ApplicationProvider  { return domain.ProviderSmartRecruiters }
func (p *Provider) Capabilities() domain.Capabilities {
	return domain.Capabilities{BrowserRequired: true}
}

func (p *Provider) Inspect(_ context.Context, req *domain.InspectRequest) (*domain.ApplicationInspection, error) {
	var targetURL string
	if req != nil {
		targetURL = req.Target.URL
	}
	return nil, domain.BrowserRequiredError("smartrecruiters", "inspection", targetURL)
}

func (p *Provider) Submit(_ context.Context, req *domain.SubmitRequest) (*domain.SubmissionResult, error) {
	var targetURL string
	if req != nil {
		targetURL = req.Target.URL
	}
	return nil, domain.BrowserRequiredError("smartrecruiters", "submission", targetURL)
}

type Source struct {
	client  *http.Client
	company string
}

func NewSource(c *http.Client, company ...string) domain.SourceAdapter {
	s := &Source{client: c}
	if len(company) > 0 {
		s.company = company[0]
	}
	return s
}
func (s *Source) Name() domain.Source { return domain.Source("smartrecruiters") }
func (s *Source) companyFor(r *domain.SearchRequest) string {
	if s.company != "" {
		return s.company
	}
	return r.Keywords
}

func (s *Source) Search(ctx context.Context, r *domain.SearchRequest) (*domain.SearchPartition, error) {
	company := s.companyFor(r)
	if company == "" {
		return nil, schema("missing SmartRecruiters company")
	}
	limit := r.Limit
	if limit <= 0 {
		limit = 20
	}
	u, _ := url.Parse(baseURL + "/companies/" + url.PathEscape(company) + "/postings")
	q := u.Query()
	q.Set("limit", fmt.Sprint(limit))
	q.Set("offset", fmt.Sprint(r.Offset))
	u.RawQuery = q.Encode()
	var result listing
	if err := s.get(ctx, u.String(), &result); err != nil {
		return nil, err
	}
	if result.Content == nil {
		return nil, schema("SmartRecruiters response is missing content")
	}
	jobs := make([]domain.Job, 0, len(result.Content))
	for i := range result.Content {
		j, err := normalize(&result.Content[i])
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, j)
	}
	total := result.TotalFound
	if total == 0 {
		total = len(jobs)
	}
	return &domain.SearchPartition{Source: s.Name(), Jobs: jobs, Pagination: &domain.Pagination{Limit: limit, Offset: r.Offset, Total: total, HasMore: r.Offset+len(jobs) < total, Native: &domain.NativePagination{Kind: domain.PaginationOffset, Limit: limit, Offset: r.Offset}}}, nil
}

func (s *Source) Detail(ctx context.Context, r *domain.DetailRequest) (*domain.Job, error) {
	company, id := s.company, r.SourceJobID
	if a, b, ok := strings.Cut(id, "/"); ok {
		company, id = a, b
	}
	if company == "" || id == "" {
		return nil, schema("missing SmartRecruiters company or posting id")
	}
	var p posting
	if err := s.get(ctx, baseURL+"/companies/"+url.PathEscape(company)+"/postings/"+url.PathEscape(id), &p); err != nil {
		return nil, err
	}
	j, err := normalize(&p)
	if err != nil {
		return nil, err
	}
	return &j, nil
}

type listing struct {
	Content    []posting `json:"content"`
	TotalFound int       `json:"totalFound"`
}
type posting struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Ref      string `json:"ref"`
	Location struct {
		City    string `json:"city"`
		Region  string `json:"region"`
		Country string `json:"country"`
	} `json:"location"`
	ReleasedDate string `json:"releasedDate"`
	Company      struct {
		Name string `json:"name"`
	} `json:"company"`
	JobAd struct {
		Sections struct {
			JobDescription struct {
				Text string `json:"text"`
			} `json:"jobDescription"`
		} `json:"sections"`
	} `json:"jobAd"`
	ApplyURL string `json:"applyUrl"`
}

func normalize(p *posting) (domain.Job, error) {
	if p.ID == "" || p.Name == "" {
		return domain.Job{}, schema("SmartRecruiters posting is missing id or name")
	}
	j := domain.NewJob(domain.Source("smartrecruiters"), p.ID)
	j.Title, j.Employer, j.PostedDate, j.SourceURL, j.ApplicationURL = p.Name, p.Company.Name, p.ReleasedDate, p.Ref, p.ApplyURL
	j.Location = strings.Trim(strings.Join([]string{p.Location.City, p.Location.Region, p.Location.Country}, ", "), ", ")
	j.Description = p.JobAd.Sections.JobDescription.Text
	return j, nil
}

func (s *Source) get(ctx context.Context, raw string, out any) error {
	r, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, http.NoBody)
	if err != nil {
		return err
	}
	r.Header.Set("Accept", "application/json")
	c := s.client
	if c == nil {
		c = http.DefaultClient
	}
	resp, err := c.Do(r)
	if err != nil {
		return errors.New(errors.NetworkUnreachable, "SmartRecruiters request failed", errors.CatNetwork, true, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return errors.New(errors.APIError, fmt.Sprintf("SmartRecruiters returned HTTP %d", resp.StatusCode), errors.CatAPI, resp.StatusCode >= 500, nil)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return schema("invalid SmartRecruiters response")
	}
	return nil
}

func schema(m string) error { return errors.New(errors.APISchemaChanged, m, errors.CatAPI, false, nil) }
