package boards

import (
	"context"
	"encoding/json"
	"regexp"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/thedavidweng/jobs-cli/v2/internal/domain"
	"golang.org/x/net/html"
)

const yziURL = "https://talent.yzilabs.com"

var flightChunk = regexp.MustCompile(`self\.__next_f\.push\(\[1,("(?:[^"\\]|\\.)*")\]\)`)

type yziJob struct {
	ID       string   `json:"id"`
	Role     string   `json:"role"`
	Company  string   `json:"company"`
	Location string   `json:"location"`
	Created  string   `json:"created_at"`
	Level    string   `json:"level"`
	Type     string   `json:"type"`
	Tags     []string `json:"tags"`
}

func (s *Source) yziJobs(ctx context.Context) ([]domain.Job, error) {
	b, err := s.get(ctx, yziURL+"/")
	if err != nil {
		return nil, err
	}
	var entries []yziJob
	found := false
	for _, match := range flightChunk.FindAllSubmatch(b, -1) {
		var chunk string
		if err := json.Unmarshal(match[1], &chunk); err != nil {
			return nil, schema("invalid YZi page data")
		}
		_, raw, ok := strings.Cut(chunk, `"initialJobs":`)
		if !ok {
			continue
		}
		if err := json.NewDecoder(strings.NewReader(raw)).Decode(&entries); err != nil {
			return nil, schema("invalid YZi initialJobs")
		}
		found = true
		break
	}
	if !found {
		return nil, schema("YZi initialJobs missing from public page")
	}
	jobs := make([]domain.Job, 0, len(entries))
	for i := range entries {
		e := &entries[i]
		if _, err := uuid.Parse(e.ID); err != nil || e.Role == "" {
			return nil, schema("YZi job missing valid ID or title")
		}
		j := domain.NewJob(domain.SourceYZi, e.ID)
		j.Title = e.Role
		j.Employer = e.Company
		j.Location = e.Location
		j.PostedDate = date(e.Created)
		j.Level = e.Level
		j.Tags = e.Tags
		j.EmploymentType = e.Type
		j.SourceURL = yziURL + "/jobs/" + e.ID
		j.ApplicationURL = j.SourceURL
		setWorkplace(&j, j.Location)
		jobs = append(jobs, j)
	}
	return jobs, nil
}

func (s *Source) yziDetail(ctx context.Context, id string) (*domain.Job, error) {
	if _, err := uuid.Parse(id); err != nil {
		return nil, invalid("YZi job ID must be a UUID")
	}
	b, err := s.get(ctx, yziURL+"/jobs/"+id)
	if err != nil {
		return nil, err
	}
	doc, err := document(b)
	if err != nil {
		return nil, err
	}
	main := tag(doc, "main")
	title := tag(main, "h1")
	if title == nil {
		return nil, schema("YZi job title missing")
	}
	j := domain.NewJob(domain.SourceYZi, id)
	j.Title = text(title)
	j.SourceURL = yziURL + "/jobs/" + id
	j.ApplicationURL = j.SourceURL
	meta := strings.Split(text(tag(title.Parent, "p")), "·")
	j.Employer = strings.TrimSpace(meta[0])
	if len(meta) > 1 {
		j.Location = strings.TrimSpace(meta[1])
	}
	if len(meta) > 2 {
		j.Level = strings.TrimSpace(meta[2])
	}
	for _, section := range nodes(main, func(n *html.Node) bool { return n.Data == "section" }) {
		if tag(section, "form") != nil {
			continue
		}
		if j.Description != "" {
			j.Description += "\n\n"
		}
		j.Description += text(section)
	}
	for _, span := range nodes(title.Parent, func(n *html.Node) bool { return n.Data == "span" }) {
		if value := text(span); value != "" {
			j.Tags = append(j.Tags, value)
		}
	}
	setWorkplace(&j, j.Location)
	if j.Description == "" {
		return nil, schema("YZi description missing")
	}
	return &j, nil
}

func (s *Source) searchJobs(ctx context.Context, req *domain.SearchRequest, jobs []domain.Job, offset int) (*domain.SearchPartition, error) {
	sort.SliceStable(jobs, func(i, j int) bool {
		if req.Sort == "oldest" {
			return jobs[i].PostedDate < jobs[j].PostedDate
		}
		return jobs[i].PostedDate > jobs[j].PostedDate
	})
	candidates := make([]domain.Job, 0, len(jobs))
	for i := range jobs {
		j := &jobs[i]
		if contains(j.Title+" "+j.Employer+" "+strings.Join(j.Tags, " "), req.Keywords) {
			candidates = append(candidates, *j)
		}
	}
	// ponytail: scan a board snapshot; use server filtering if these small boards grow substantially.
	result := []domain.Job{}
	next := offset

	for next < len(candidates) && len(result) < req.Limit {
		end := min(next+min(4, req.Limit-len(result)), len(candidates))
		batch := candidates[next:end]
		if s.name == domain.SourceTransLink {
			var err error
			batch, err = s.hydrate(ctx, batch)
			if err != nil {
				return nil, err
			}
		}
		for i := range batch {
			j := &batch[i]
			next++
			if !contains(j.Location, req.Location) || (req.Remote != nil && *req.Remote && !j.Remote) {
				continue
			}
			result = append(result, *j)
		}
	}
	return partition(s.name, result, offset, next, len(candidates), req.Limit), nil
}
