package icims

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/thedavidweng/jobs-cli/internal/domain"
	"github.com/thedavidweng/jobs-cli/internal/errors"
	"golang.org/x/net/html"
)

type Provider struct{ Client *http.Client }

func NewProvider(client *http.Client) domain.ApplyProvider { return &Provider{Client: client} }
func (p *Provider) Name() domain.ApplicationProvider       { return domain.ProviderICIMS }
func (p *Provider) Capabilities() domain.Capabilities {
	return domain.Capabilities{Inspect: true, BrowserRequired: true}
}

func (p *Provider) Submit(_ context.Context, _ *domain.SubmitRequest) (*domain.SubmissionResult, error) {
	return nil, errors.New(errors.NativeApplyUnsupported, "icims native application submission is not supported; apply in a browser", errors.CatAPI, false, nil)
}

func (p *Provider) Inspect(ctx context.Context, req *domain.InspectRequest) (*domain.ApplicationInspection, error) {
	if req == nil || req.Target.URL == "" {
		return nil, errors.New(errors.ValidationFailed, "iCIMS application URL is required", errors.CatValidation, false, nil)
	}
	target := req.Target
	target.Provider = domain.ProviderICIMS
	target.Capabilities = p.Capabilities()
	inspection := &domain.ApplicationInspection{Provider: domain.ProviderICIMS, Application: target, Capabilities: p.Capabilities()}
	client := p.Client
	if client == nil {
		client = http.DefaultClient
	}
	r, err := http.NewRequestWithContext(ctx, http.MethodGet, target.URL, http.NoBody)
	if err != nil {
		return nil, err
	}
	r.Header.Set("Accept", "text/html,application/xhtml+xml")
	resp, err := client.Do(r)
	if err != nil {
		return nil, errors.New(errors.NetworkUnreachable, "iCIMS page request failed", errors.CatNetwork, true, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, errors.New(errors.APIError, fmt.Sprintf("iCIMS returned HTTP %d", resp.StatusCode), errors.CatAPI, resp.StatusCode >= 500, nil)
	}
	posting, found, err := extract(resp.Body)
	if err != nil {
		return nil, errors.New(errors.APISchemaChanged, "invalid iCIMS JSON-LD", errors.CatAPI, false, err)
	}
	if found {
		if req.Job.Title == "" {
			req.Job.Title = stringValue(posting["title"])
		}
		if req.Job.Description == "" {
			req.Job.Description = stringValue(posting["description"])
		}
		if req.Job.Location == "" {
			req.Job.Location = location(posting["jobLocation"])
		}
		if req.Job.PostedDate == "" {
			req.Job.PostedDate = stringValue(posting["datePosted"])
		}
		inspection.Application.URL = target.URL
	}
	inspection.Fingerprint = domain.Fingerprint(&inspection.Application, inspection.Fields, inspection.Questions)
	return inspection, nil
}

func extract(r interface{ Read([]byte) (int, error) }) (posting map[string]any, ok bool, extractErr error) {
	doc, err := html.Parse(r)
	if err != nil {
		return nil, false, err
	}
	var parseErr error
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if posting != nil {
			return
		}
		if n.Type == html.ElementNode && n.Data == "script" && scriptType(n) == "application/ld+json" {
			var raw strings.Builder
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				if c.Type == html.TextNode {
					raw.WriteString(c.Data)
				}
			}
			job, ok, err := jobPosting([]byte(raw.String()))
			if err != nil {
				parseErr = err
				return
			}
			if ok {
				posting = job
				return
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return posting, posting != nil, parseErr
}

func scriptType(n *html.Node) string {
	for _, a := range n.Attr {
		if strings.EqualFold(a.Key, "type") {
			return strings.ToLower(strings.TrimSpace(a.Val))
		}
	}
	return ""
}

func jobPosting(raw []byte) (posting map[string]any, ok bool, parseErr error) {
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, false, err
	}
	job, ok := findPosting(value)
	return job, ok, nil
}

func findPosting(value any) (map[string]any, bool) {
	switch v := value.(type) {
	case map[string]any:
		if typ, ok := v["@type"]; ok && hasType(typ, "JobPosting") {
			return v, true
		}
		if graph, ok := v["@graph"]; ok {
			return findPosting(graph)
		}
	case []any:
		for _, x := range v {
			if job, ok := findPosting(x); ok {
				return job, true
			}
		}
	}
	return nil, false
}

func hasType(value any, want string) bool {
	switch t := value.(type) {
	case string:
		return t == want
	case []any:
		for _, x := range t {
			if s, ok := x.(string); ok && s == want {
				return true
			}
		}
	}
	return false
}
func stringValue(value any) string { s, _ := value.(string); return s }
func location(value any) string {
	switch v := value.(type) {
	case map[string]any:
		if a, ok := v["address"].(map[string]any); ok {
			return strings.Trim(strings.Join([]string{stringValue(a["addressLocality"]), stringValue(a["addressRegion"]), stringValue(a["addressCountry"])}, ", "), ", ")
		}
	case []any:
		for _, x := range v {
			if got := location(x); got != "" {
				return got
			}
		}
	}
	return ""
}
