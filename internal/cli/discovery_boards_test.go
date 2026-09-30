package cli_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/thedavidweng/jobs-cli/v2/internal/domain"
)

type boardTransport func(*http.Request) (*http.Response, error)

func (f boardTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestLocalBoardsDiscoveryAndBrowserRouting(t *testing.T) {
	const uuid = "41f81a01-ed89-4983-b433-22d62121a5f4"
	const psURL = "https://careersconnect.translink.bc.ca/psc/EXT/EMPLOYEE/HRMS/c/HRS_HRAM_FL.HRS_CG_SEARCH_FL.GBL?Page=HRS_APP_JBPST_FL&Action=U&FOCUS=Applicant&SiteId=2&JobOpeningId=20251035&PostingSeq=1"
	rt := boardTransport(func(r *http.Request) (*http.Response, error) {
		var body string
		switch r.URL.Hostname() {
		case "talent.yzilabs.com":
			if r.URL.Path == "/" {
				body = `<script>self.__next_f.push([1,"0:{\"initialJobs\":[{\"id\":\"` + uuid + `\",\"role\":\"Backend Engineer\",\"company\":\"SmartX\",\"location\":\"Remote\",\"created_at\":\"2026-09-29\",\"tags\":[\"AI\"],\"level\":\"Mid\",\"type\":\"Full-time\"}]}"])</script>`
			} else {
				body = `<main><h1>Backend Engineer</h1><p>SmartX · Remote · Mid</p><section><h2>About the role</h2><p>Build trading systems.</p></section><section><h2>Requirements</h2><p>Go experience.</p></section><section><h2>Apply for this role</h2><form></form></section></main>`
			}
		case "www.civicinfo.bc.ca":
			if r.URL.Query().Get("jobid") != "" {
				body = `<div class="job-description"><h2>City of Vancouver</h2><h2>Backend Engineer</h2><p>Build public services. <a href="https://careers.example.com/jobs/42">Apply online</a></p></div><div class="job-overview"><h6>Job Title</h6><p>Backend Engineer</p><h6>Date Posted</h6><p>September 29, 2026, 3:35 pm</p><h6>Expires</h6><p>November 2, 2026, 4:30 pm</p><h6>Employment Type</h6><p>Full Time</p><h6>Employment Length</h6><p>Permanent</p></div><div class="company-profile"><h2>City of Vancouver</h2><h6>Location</h6><p>Vancouver, BC</p></div>`
			} else {
				body = `<form id="search-form1"><p class="showing"><span>1</span> records; Displaying 1 to 1</p><li class="jobbox-42"><div class="post-info"><div class="title"><a href="careers?jobid=42">Backend Engineer</a></div><p><a>City of Vancouver</a></p><p><span class="time">Vancouver, BC <i></i>1 hr ago</span></p></div></li></form>`
			}
		case "getaway.translink.ca":
			body = `[{"title":"Backend Engineer","url":"` + psURL + `","company":"001","startDate":"2026-09-29","endDate":"N/A"}]`
		case "careersconnect.translink.bc.ca":
			body = `<span id="HRS_SCH_WRK2_HRS_JOB_OPENING_ID">20251035</span><span id="HRS_SCH_WRK2_POSTING_TITLE">Backend Engineer</span><span id="HRS_SCH_WRK_COMPANY_DESCR">TransLink</span><span id="HRS_SCH_WRK_HRS_DESCRLONG">Greater Vancouver</span><span id="HRS_SCH_WRK_HRS_FULL_PART_TIME">Full-Time</span><span id="HRS_SCH_WRK_HRS_REG_TEMP">Regular</span><div id="win0divHRS_SCH_WRK_DESCR100$0"><h2>Responsibilities</h2><span id="HRS_SCH_PSTDSC_DESCRLONG$0">Build public services.</span></div><div id="win0divHRS_SCH_WRK_DESCR100$1"><h2>Rate of Pay</h2><span id="HRS_SCH_PSTDSC_DESCRLONG$1">$109,920 - $164,880 per annum</span></div><div id="win0divHRS_SCH_WRK_DESCR100$2"><h2>How to Apply</h2><span id="HRS_SCH_PSTDSC_DESCRLONG$2"><p>Posting Date: September 29, 2026<br>Closing Date: Open until filled</p></span></div>`
		default:
			return nil, fmt.Errorf("unexpected request: %s", r.URL)
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
	})
	for _, tc := range []struct{ source, id, provider, closing string }{
		{"yzi", uuid, "yzi", ""}, {"civicinfo", "42", "external", "2026-11-02"}, {"translink", "20251035", "peoplesoft", "Open until filled"},
	} {
		t.Run(tc.source, func(t *testing.T) {
			h := newHarness(t).withTransport(rt).useRealRegistry()
			out, _, code := h.run("--json", "search", "--source", tc.source, "--query", "Engineer", "--limit", "1")
			if code != 0 {
				t.Fatalf("search: %s", out)
			}

			var search struct{ Data domain.SearchResult }
			if err := json.Unmarshal([]byte(out), &search); err != nil {
				t.Fatal(err)
			}
			jobs := search.Data.Partitions[0].Jobs
			if len(jobs) != 1 || jobs[0].ID != tc.source+":"+tc.id {
				t.Fatalf("jobs: %#v", jobs)
			}
			out, _, code = h.run("--json", "show", tc.source+":"+tc.id, "--resolve")
			if code != 0 {
				t.Fatalf("show: %s", out)
			}

			var shown struct{ Data domain.Job }
			if err := json.Unmarshal([]byte(out), &shown); err != nil {
				t.Fatal(err)
			}
			job := shown.Data
			if !strings.Contains(job.Description, "Build") {
				t.Fatalf("description: %#v", job)
			}
			if tc.closing != "" && job.ClosingDate != tc.closing {
				t.Errorf("closing: %#v", job)
			}
			target := job.Application
			if target == nil || string(target.Provider) != tc.provider || !target.Capabilities.BrowserRequired {
				t.Fatalf("target: %#v", target)
			}
			if tc.source == "yzi" {
				out, _, code = h.run("--json", "search", "--source", "yzi", "--limit", "1")
				if code != 0 {
					t.Fatalf("browse board: %s", out)
				}
			}
			if tc.provider != "external" {
				out, _, code = h.run("--json", "apply", "inspect", tc.source+":"+tc.id)
				if code == 0 || decodeEnvelope(t, out).Error.Code != "BROWSER_REQUIRED" {
					t.Fatalf("apply inspect: %s", out)
				}
			}
		})
	}
}
