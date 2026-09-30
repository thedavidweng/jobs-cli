// Package boards implements the public YZi, CivicInfo BC, and TransLink discovery sources.
package boards

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/thedavidweng/jobs-cli/v2/internal/domain"
	joberrors "github.com/thedavidweng/jobs-cli/v2/internal/errors"
	"github.com/thedavidweng/jobs-cli/v2/internal/httpclient"
	"golang.org/x/net/html"
)

type Source struct {
	name   domain.Source
	client *http.Client
}

func New(name domain.Source, client *http.Client) domain.SourceAdapter {
	if client == nil {
		client = http.DefaultClient
	}
	return &Source{name: name, client: client}
}

func (s *Source) get(ctx context.Context, rawURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, http.NoBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "text/html,application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; jobs-cli)")
	client := s.client
	if s.name == domain.SourceTransLink {
		copied := *client
		copied.Jar, _ = cookiejar.New(nil)
		client = &copied
	}
	resp, err := client.Do(req)
	if err != nil {
		code := joberrors.NetworkUnreachable
		if errors.Is(err, context.DeadlineExceeded) {
			code = joberrors.NetworkTimeout
		}
		return nil, joberrors.New(code, string(s.name)+" request failed", joberrors.CatNetwork, true, err)
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == 404:
		return nil, joberrors.New(joberrors.ResourceNotFound, "job was not found on "+string(s.name), joberrors.CatAPI, false, nil)
	case resp.StatusCode == 403 || resp.StatusCode == 401:
		return nil, joberrors.New(joberrors.APIAccessForbidden, string(s.name)+" denied the public request (browser verification may be required)", joberrors.CatAPI, false, nil)
	case resp.StatusCode == 429:
		return nil, joberrors.NewWithRetryAfter(joberrors.RateLimited, string(s.name)+" rate limited the request", joberrors.CatNetwork, true, httpclient.RetryAfter(resp, 0), nil)
	case resp.StatusCode < 200 || resp.StatusCode >= 300:
		return nil, joberrors.New(joberrors.APIError, fmt.Sprintf("%s returned HTTP %d", s.name, resp.StatusCode), joberrors.CatAPI, resp.StatusCode >= 500, nil)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20+1))
	if err != nil {
		return nil, joberrors.New(joberrors.APIError, "could not read "+string(s.name)+" response", joberrors.CatAPI, false, err)
	}
	if len(b) > 10<<20 {
		return nil, joberrors.New(joberrors.APIError, "board response exceeds 10 MiB", joberrors.CatAPI, false, nil)
	}
	return b, nil
}

func (s *Source) Search(ctx context.Context, req *domain.SearchRequest) (*domain.SearchPartition, error) {
	offset, err := searchOffset(req)
	if err != nil {
		return nil, err
	}
	if req.Radius != 0 {
		return nil, invalid("these boards do not support --radius")
	}
	if s.name == domain.SourceCivicInfo {
		return s.searchCivic(ctx, req, offset)
	}
	if req.Sort != "" && req.Sort != "newest" && req.Sort != "oldest" && req.Sort != "date" {
		return nil, invalid("supported sort orders: newest, oldest, date")
	}
	var jobs []domain.Job
	switch s.name {
	case domain.SourceYZi:
		jobs, err = s.yziJobs(ctx)
	case domain.SourceTransLink:
		jobs, err = s.transJobs(ctx)
	}
	if err != nil {
		return nil, err
	}
	return s.searchJobs(ctx, req, jobs, offset)
}

func searchOffset(req *domain.SearchRequest) (int, error) {
	if req == nil || req.Limit < 1 || req.Limit > 100 {
		return 0, invalid("board search limit must be between 1 and 100")
	}
	offset := req.Offset
	if req.Cursor != "" {
		var err error
		offset, err = strconv.Atoi(req.Cursor)
		if err != nil {
			return 0, invalid("board cursor must be a non-negative integer offset")
		}
		if req.Offset != 0 && req.Offset != offset {
			return 0, invalid("cursor and offset disagree")
		}
	}
	if offset < 0 {
		return 0, invalid("offset must not be negative")
	}
	return offset, nil
}

func (s *Source) Detail(ctx context.Context, req *domain.DetailRequest) (*domain.Job, error) {
	if req == nil {
		return nil, invalid("job ID is required")
	}
	switch s.name {
	case domain.SourceYZi:
		return s.yziDetail(ctx, req.SourceJobID)
	case domain.SourceCivicInfo:
		return s.civicDetail(ctx, req.SourceJobID)
	case domain.SourceTransLink:
		return s.transDetail(ctx, req.SourceJobID)
	}
	return nil, invalid("unknown board")
}

func invalid(message string) error {
	return joberrors.New(joberrors.ValidationFailed, message, joberrors.CatValidation, false, nil)
}

func schema(message string) error {
	return joberrors.New(joberrors.APISchemaChanged, message, joberrors.CatAPI, false, nil)
}
func document(b []byte) (*html.Node, error) { return html.Parse(strings.NewReader(string(b))) }

func nodes(root *html.Node, match func(*html.Node) bool) []*html.Node {
	var found []*html.Node
	if root != nil {
		for n := range root.Descendants() {
			if n.Type == html.ElementNode && match(n) {
				found = append(found, n)
			}
		}
	}
	return found
}

func first(root *html.Node, match func(*html.Node) bool) *html.Node {
	if root != nil {
		for n := range root.Descendants() {
			if n.Type == html.ElementNode && match(n) {
				return n
			}
		}
	}
	return nil
}

func tag(root *html.Node, name string) *html.Node {
	return first(root, func(n *html.Node) bool { return n.Data == name })
}

func attr(n *html.Node, key string) string {
	if n != nil {
		for _, a := range n.Attr {
			if a.Key == key {
				return a.Val
			}
		}
	}
	return ""
}

func class(n *html.Node, name string) bool {
	for _, c := range strings.Fields(attr(n, "class")) {
		if c == name {
			return true
		}
	}
	return false
}

func byClass(root *html.Node, name string) *html.Node {
	return first(root, func(n *html.Node) bool { return class(n, name) })
}

func byID(root *html.Node, id string) *html.Node {
	return first(root, func(n *html.Node) bool { return attr(n, "id") == id })
}

func text(root *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
		}
		if n.Data == "script" || n.Data == "style" {
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
		switch n.Data {
		case "p", "div", "br", "li", "h1", "h2", "h6", "section":
			b.WriteByte('\n')
		}
	}
	if root != nil {
		walk(root)
	}
	var lines []string
	for _, line := range strings.Split(b.String(), "\n") {
		if line = strings.Join(strings.Fields(line), " "); line != "" {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "\n")
}

func field(root *html.Node, label string) string {
	for _, h := range nodes(root, func(n *html.Node) bool { return n.Data == "h6" }) {
		if text(h) == label {
			for sibling := h.NextSibling; sibling != nil; sibling = sibling.NextSibling {
				if sibling.Type == html.ElementNode {
					if sibling.Data == "p" {
						return text(sibling)
					}
					break
				}
			}
		}
	}
	return ""
}

func setWorkplace(j *domain.Job, value string) {
	value = strings.ToLower(value)
	switch {
	case strings.Contains(value, "hybrid"):
		j.Workplace = domain.WorkplaceHybrid
	case strings.Contains(value, "remote"):
		j.Workplace = domain.WorkplaceRemote
		j.Remote = true
	case strings.Contains(value, "onsite"), strings.Contains(value, "on-site"):
		j.Workplace = domain.WorkplaceOnsite
	}
}

var ordinalDay = regexp.MustCompile(`(\d+)(st|nd|rd|th)\b`)

func date(value string) string {
	value = ordinalDay.ReplaceAllString(value, "$1")
	for _, layout := range []string{"January 2, 2006, 3:04 pm", "January 02, 2006", "January 2, 2006", "January 2 2006", "2006-01-02", time.RFC3339Nano} {
		if t, err := time.Parse(layout, value); err == nil {
			return t.Format("2006-01-02")
		}
	}
	return value
}

func digits(id string) bool {
	if id == "" {
		return false
	}
	for _, r := range id {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func contains(value, query string) bool {
	return strings.Contains(strings.ToLower(value), strings.ToLower(query))
}

func partition(source domain.Source, jobs []domain.Job, offset, next, total, limit int) *domain.SearchPartition {
	p := &domain.Pagination{Limit: limit, Offset: offset, Total: total, HasMore: next < total, Native: &domain.NativePagination{Kind: domain.PaginationOffset, Offset: next}}
	if p.HasMore {
		p.NextCursor = strconv.Itoa(next)
	}
	return &domain.SearchPartition{Source: source, Jobs: jobs, Pagination: p}
}

func decodeJSON(b []byte, target any) error {
	if err := json.Unmarshal(b, target); err != nil {
		return schema("invalid board JSON: " + err.Error())
	}
	return nil
}

// hydrate reads at most four independent detail pages at a time.
func (s *Source) hydrate(ctx context.Context, jobs []domain.Job) ([]domain.Job, error) {
	details := make([]domain.Job, len(jobs))
	errs := make([]error, len(jobs))
	var wg sync.WaitGroup
	for i := range jobs {
		j := &jobs[i]
		wg.Add(1)
		go func() {
			defer wg.Done()
			var detail *domain.Job
			if s.name == domain.SourceTransLink {
				detail, errs[i] = s.transDetailURL(ctx, j.SourceJobID, j.SourceURL)
			} else {
				detail, errs[i] = s.civicDetail(ctx, j.SourceJobID)
			}
			if errs[i] == nil {
				details[i] = *detail
			}
		}()
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			return nil, err
		}
	}
	return details, nil
}
