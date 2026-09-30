package boards

import (
	"context"
	"net/url"
	"regexp"
	"strings"

	"github.com/thedavidweng/jobs-cli/v2/internal/domain"
	"golang.org/x/net/html"
)

const (
	transSearch     = "https://getaway.translink.ca/api/jobpostings"
	transDetailBase = "https://careersconnect.translink.bc.ca/psc/EXT/EMPLOYEE/HRMS/c/HRS_HRAM_FL.HRS_CG_SEARCH_FL.GBL"
)

var (
	companies   = map[string]string{"001": "TransLink", "BCT": "Coast Mountain Bus Company", "BCR": "BC Rapid Transit Company", "WCE": "West Coast Express", "TSM": "Transit Police"}
	postingDate = regexp.MustCompile(`(?m)Posting Date:\s*([^\n]+)`)
	closingDate = regexp.MustCompile(`(?m)Closing Date:\s*([^\n]+)`)
)

func (s *Source) transJobs(ctx context.Context) ([]domain.Job, error) {
	b, err := s.get(ctx, transSearch)
	if err != nil {
		return nil, err
	}
	var entries []struct {
		Title   string `json:"title"`
		URL     string `json:"url"`
		Company string `json:"company"`
		Start   string `json:"startDate"`
		End     string `json:"endDate"`
	}
	if err := decodeJSON(b, &entries); err != nil {
		return nil, err
	}
	if entries == nil {
		return nil, schema("TransLink job feed must be an array")
	}
	jobs := make([]domain.Job, 0, len(entries))
	for _, e := range entries {
		u, err := url.Parse(e.URL)
		if err != nil || u.Scheme != "https" || u.Hostname() != "careersconnect.translink.bc.ca" || u.Path != "/psc/EXT/EMPLOYEE/HRMS/c/HRS_HRAM_FL.HRS_CG_SEARCH_FL.GBL" {
			return nil, schema("invalid TransLink job URL")
		}
		id := u.Query().Get("JobOpeningId")
		if !digits(id) || e.Title == "" {
			return nil, schema("TransLink job missing ID or title")
		}
		j := domain.NewJob(domain.SourceTransLink, id)
		j.Title = e.Title
		j.Employer = companies[e.Company]
		j.SourceURL = e.URL
		j.ApplicationURL = e.URL
		j.PostedDate = date(e.Start)
		j.ClosingDate = e.End
		if e.End == "N/A" {
			j.ClosingDate = "Open until filled"
		}
		jobs = append(jobs, j)
	}
	return jobs, nil
}

func (s *Source) transDetail(ctx context.Context, id string) (*domain.Job, error) {
	if !digits(id) {
		return nil, invalid("TransLink job ID must be numeric")
	}
	q := url.Values{"Page": {"HRS_APP_JBPST_FL"}, "Action": {"U"}, "FOCUS": {"Applicant"}, "SiteId": {"2"}, "JobOpeningId": {id}, "PostingSeq": {"1"}}
	return s.transDetailURL(ctx, id, transDetailBase+"?"+q.Encode())
}

func (s *Source) transDetailURL(ctx context.Context, id, rawURL string) (*domain.Job, error) {
	b, err := s.get(ctx, rawURL)
	if err != nil {
		return nil, err
	}
	doc, err := document(b)
	if err != nil {
		return nil, err
	}
	title := text(byID(doc, "HRS_SCH_WRK2_POSTING_TITLE"))
	actualID := text(byID(doc, "HRS_SCH_WRK2_HRS_JOB_OPENING_ID"))
	if actualID != id || title == "" {
		return nil, schema("PeopleSoft posting missing requested job ID or title")
	}
	j := domain.NewJob(domain.SourceTransLink, id)
	j.Title = title
	j.Employer = text(byID(doc, "HRS_SCH_WRK_COMPANY_DESCR"))
	j.Location = text(byID(doc, "HRS_SCH_WRK_HRS_DESCRLONG"))
	j.SourceURL = rawURL
	j.ApplicationURL = rawURL
	j.EmploymentType = text(byID(doc, "HRS_SCH_WRK_HRS_FULL_PART_TIME"))
	j.EmploymentLength = text(byID(doc, "HRS_SCH_WRK_HRS_REG_TEMP"))
	for _, n := range nodes(doc, func(n *html.Node) bool { return strings.HasPrefix(attr(n, "id"), "win0divHRS_SCH_WRK_DESCR100$") }) {
		content := first(n, func(n *html.Node) bool { return strings.HasPrefix(attr(n, "id"), "HRS_SCH_PSTDSC_DESCRLONG$") })
		heading := text(tag(n, "h2"))
		value := text(content)
		if j.Description != "" {
			j.Description += "\n\n"
		}
		j.Description += heading + "\n" + value
		switch heading {
		case "Rate of Pay":
			j.Compensation = &domain.Compensation{Summary: value, Currency: "CAD"}
		case "Work Designation":
			setWorkplace(&j, value)
		}
	}
	if j.Description == "" {
		return nil, schema("PeopleSoft job description missing")
	}
	if m := postingDate.FindStringSubmatch(j.Description); m != nil {
		j.PostedDate = date(m[1])
	}
	if m := closingDate.FindStringSubmatch(j.Description); m != nil {
		j.ClosingDate = date(m[1])
	}
	return &j, nil
}
