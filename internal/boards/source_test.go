package boards

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/thedavidweng/jobs-cli/v2/internal/domain"
	joberrors "github.com/thedavidweng/jobs-cli/v2/internal/errors"
)

type transport func(*http.Request) (*http.Response, error)

func (f transport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func reply(r *http.Request, status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: r}
}

func TestYZiFilteringAndContinuation(t *testing.T) {
	entries := []yziJob{
		{ID: "41f81a01-ed89-4983-b433-22d62121a5f4", Role: "Engineer", Location: "Remote Canada", Created: "2026-09-29", Tags: []string{"AI"}},
		{ID: "0f21c40b-29c3-4649-b53f-9172e441c511", Role: "Engineer", Location: "Vancouver Canada", Created: "2026-09-28", Tags: []string{"AI"}},
		{ID: "24cfb2fe-ef36-465a-9609-e3a2d567e50e", Role: "Engineer", Location: "Remote Canada", Created: "2026-09-27", Tags: []string{"AI"}},
	}
	raw, _ := json.Marshal(entries)
	chunk, _ := json.Marshal(`0:{"initialJobs":` + string(raw) + `}`)
	s := New(domain.SourceYZi, &http.Client{Transport: transport(func(r *http.Request) (*http.Response, error) {
		return reply(r, 200, `<script>self.__next_f.push([1,`+string(chunk)+`])</script>`), nil
	})})
	remote := true
	req := &domain.SearchRequest{Keywords: "AI", Location: "Canada", Remote: &remote, Limit: 1}
	first, err := s.Search(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Jobs) != 1 || first.Jobs[0].SourceJobID != "41f81a01-ed89-4983-b433-22d62121a5f4" || first.Pagination.NextCursor != "1" {
		t.Fatalf("first: %+v", first)
	}
	req.Cursor = first.Pagination.NextCursor
	second, err := s.Search(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Jobs) != 1 || second.Jobs[0].SourceJobID != "24cfb2fe-ef36-465a-9609-e3a2d567e50e" || second.Pagination.HasMore {
		t.Fatalf("second: %+v", second)
	}
	req.Cursor = "99"
	empty, err := s.Search(context.Background(), req)
	if err != nil || len(empty.Jobs) != 0 || empty.Pagination.HasMore {
		t.Fatalf("past end: %+v, %v", empty, err)
	}
}

func TestCivicContinuationRequestsCorrectPage(t *testing.T) {
	s := New(domain.SourceCivicInfo, &http.Client{Transport: transport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Query().Get("jobid") != "" {
			return reply(r, 200, `<div class="job-description">Description</div><div class="job-overview"><h6>Job Title</h6><p>Engineer</p><h6>Date Posted</h6><p>September 29, 2026, 3:35 pm</p><h6>Expires</h6><p>November 2, 2026, 4:30 pm</p></div>`), nil
		}
		if r.URL.Query().Get("pn") != "2" || r.URL.Query().Get("stext") != "Engineer" || r.URL.Query().Get("ntd") != "25" {
			return nil, fmt.Errorf("wrong continuation URL: %s", r.URL)
		}
		return reply(r, 200, `<p class="showing"><span>26</span></p><li class="jobbox-42"><div class="title"><a href="careers?jobid=42">Engineer</a></div></li>`), nil
	})})
	p, err := s.Search(context.Background(), &domain.SearchRequest{Keywords: "Engineer", Limit: 1, Cursor: "25"})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Jobs) != 1 || p.Jobs[0].ID != "civicinfo:42" || p.Pagination.HasMore {
		t.Fatalf("page: %+v", p)
	}
}

func TestBoardsRejectChallengesAndChangedSchemas(t *testing.T) {
	for _, tc := range []struct {
		status int
		body   string
		code   joberrors.Code
	}{{403, "<title>Just a moment...</title>", joberrors.APIAccessForbidden}, {429, "rate limited", joberrors.RateLimited}, {200, "<html>new layout</html>", joberrors.APISchemaChanged}} {
		t.Run(string(tc.code), func(t *testing.T) {
			s := New(domain.SourceYZi, &http.Client{Transport: transport(func(r *http.Request) (*http.Response, error) { return reply(r, tc.status, tc.body), nil })})
			_, err := s.Search(context.Background(), &domain.SearchRequest{Limit: 1})
			if err == nil || joberrors.From(err).Code != tc.code {
				t.Fatalf("error: %v", err)
			}
		})
	}
}

func TestTransLinkOrdinalDates(t *testing.T) {
	if got := date("September 25th 2026"); got != "2026-09-25" {
		t.Fatalf("date = %q", got)
	}
}

func TestCivicSearchLimitsDetailConcurrency(t *testing.T) {
	var active, peak atomic.Int32
	s := New(domain.SourceCivicInfo, &http.Client{Transport: transport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Query().Get("jobid") == "" {
			body := `<p class="showing"><span>12</span></p>`
			for id := range 12 {
				body += fmt.Sprintf(`<li class="jobbox-%d"><div class="title"><a href="careers?jobid=%d">Engineer</a></div></li>`, id+1, id+1)
			}
			return reply(r, 200, body), nil
		}
		now := active.Add(1)
		defer active.Add(-1)
		for old := peak.Load(); now > old; old = peak.Load() {
			if peak.CompareAndSwap(old, now) {
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
		return reply(r, 200, `<div class="job-description">Description</div><div class="job-overview"><h6>Job Title</h6><p>Engineer</p></div>`), nil
	})})
	result, err := s.Search(context.Background(), &domain.SearchRequest{Limit: 12})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Jobs) != 12 || peak.Load() > 4 {
		t.Fatalf("jobs=%d, concurrent detail requests=%d", len(result.Jobs), peak.Load())
	}
}

func TestCivicSearchStopsAfterDetailFailure(t *testing.T) {
	var requests atomic.Int32
	s := New(domain.SourceCivicInfo, &http.Client{Transport: transport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Query().Get("jobid") == "" {
			body := `<p class="showing"><span>12</span></p>`
			for id := range 12 {
				body += fmt.Sprintf(`<li class="jobbox-%d"><div class="title"><a href="careers?jobid=%d">Engineer</a></div></li>`, id+1, id+1)
			}
			return reply(r, 200, body), nil
		}
		requests.Add(1)
		return reply(r, http.StatusForbidden, "challenge"), nil
	})})
	_, err := s.Search(context.Background(), &domain.SearchRequest{Limit: 12})
	if err == nil || joberrors.From(err).Code != joberrors.APIAccessForbidden || requests.Load() > 4 {
		t.Fatalf("error=%v, detail requests after rejection=%d", err, requests.Load())
	}
}
