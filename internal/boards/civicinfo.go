package boards

import (
	"context"
	"net/url"
	"strconv"
	"strings"

	"github.com/thedavidweng/jobs-cli/v2/internal/domain"
	"golang.org/x/net/html"
)

const civicURL = "https://www.civicinfo.bc.ca"

func (s *Source) searchCivic(ctx context.Context, req *domain.SearchRequest, offset int) (*domain.SearchPartition, error) {
	switch req.Sort {
	case "", "newest", "oldest", "soonest", "last":
	default:
		return nil, invalid("CivicInfo sort orders: newest, oldest, soonest, last")
	}
	result := []domain.Job{}
	next := offset
	total := 0
	for {
		q := url.Values{"type": {"ss"}, "stext": {req.Keywords}, "ntd": {"25"}, "pn": {strconv.Itoa(next/25 + 1)}}
		if req.Sort != "" {
			q.Set("ob", req.Sort)
		}
		b, err := s.get(ctx, civicURL+"/careers?"+q.Encode())
		if err != nil {
			return nil, err
		}
		doc, err := document(b)
		if err != nil {
			return nil, err
		}
		summary := byClass(doc, "showing")
		count := text(tag(summary, "span"))
		total, err = strconv.Atoi(count)
		if err != nil {
			return nil, schema("CivicInfo result count missing")
		}
		cards := nodes(doc, func(n *html.Node) bool { return n.Data == "li" && strings.Contains(attr(n, "class"), "jobbox-") })
		if len(cards) == 0 && next < total {
			return nil, schema("CivicInfo job cards missing")
		}
		start := next % 25

		for i := min(start, len(cards)); i < len(cards) && len(result) < req.Limit; {
			end := min(i+min(4, req.Limit-len(result)), len(cards))
			jobs := make([]domain.Job, 0, end-i)
			for _, card := range cards[i:end] {
				link := tag(byClass(card, "title"), "a")
				u, err := url.Parse(attr(link, "href"))
				if err != nil {
					return nil, schema("invalid CivicInfo job link")
				}
				id := u.Query().Get("jobid")
				if !digits(id) || text(link) == "" {
					return nil, schema("CivicInfo card missing job ID or title")
				}
				jobs = append(jobs, domain.NewJob(s.name, id))
			}
			details, err := s.hydrate(ctx, jobs)
			if err != nil {
				return nil, err
			}
			for k := range details {
				j := &details[k]
				next++
				if !contains(j.Location, req.Location) || (req.Remote != nil && *req.Remote && !j.Remote) {
					continue
				}
				result = append(result, *j)
			}
			i = end
		}
		if len(result) >= req.Limit || next >= total {
			break
		}
		if len(cards) < 25 {
			return nil, schema("CivicInfo page shorter than result count")
		}
	}
	return partition(s.name, result, offset, next, total, req.Limit), nil
}

func (s *Source) civicDetail(ctx context.Context, id string) (*domain.Job, error) {
	if !digits(id) {
		return nil, invalid("CivicInfo job ID must be numeric")
	}
	rawURL := civicURL + "/careers?jobid=" + id
	b, err := s.get(ctx, rawURL)
	if err != nil {
		return nil, err
	}
	doc, err := document(b)
	if err != nil {
		return nil, err
	}
	overview := byClass(doc, "job-overview")
	company := byClass(doc, "company-profile")
	description := byClass(doc, "job-description")
	title := field(overview, "Job Title")
	if title == "" || description == nil {
		return nil, schema("CivicInfo posting missing title or description")
	}
	j := domain.NewJob(domain.SourceCivicInfo, id)
	j.Title = title
	j.Employer = text(tag(company, "h2"))
	j.Location = field(company, "Location")
	j.Description = text(description)
	j.SourceURL = rawURL
	j.PostedDate = date(field(overview, "Date Posted"))
	j.ClosingDate = date(field(overview, "Expires"))
	j.EmploymentType = field(overview, "Employment Type")
	j.EmploymentLength = field(overview, "Employment Length")
	setWorkplace(&j, field(overview, "Workplace Information"))
	if rate := field(overview, "Rate"); rate != "" {
		j.Compensation = &domain.Compensation{Summary: rate}
	}
	for _, a := range nodes(description, func(n *html.Node) bool { return n.Data == "a" }) {
		href := attr(a, "href")
		u, err := url.Parse(href)
		if err != nil {
			continue
		}
		if (u.Scheme == "http" || u.Scheme == "https") && (contains(href, "job") || contains(href, "career") || contains(href, "apply") || contains(href, "posting") || contains(href, "position") || contains(text(a), "apply")) {
			j.ApplicationURL = href
			break
		}
	}
	return &j, nil
}
