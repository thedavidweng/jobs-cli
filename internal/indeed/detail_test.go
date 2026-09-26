package indeed_test

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/thedavidweng/jobs-cli/internal/domain"
	joberrors "github.com/thedavidweng/jobs-cli/internal/errors"
	"github.com/thedavidweng/jobs-cli/internal/testutil"
)

const detailJobKey = "517ca3fd71acddc9"

func TestDetailBatchRequestAndNormalization(t *testing.T) {
	r := &recorder{t: t, respond: func(int) *http.Response { return fixtureResponse(t, "detail.json") }}
	job, err := newSource(r).Detail(context.Background(), &domain.DetailRequest{SourceJobID: detailJobKey})
	if err != nil {
		t.Fatalf("detail: %v", err)
	}

	query := queryText(t, r.body)
	if !strings.Contains(query, `jobData(jobKeys: ["`+detailJobKey+`"])`) {
		t.Errorf("detail request is not a jobKeys batch:\n%s", query)
	}
	for _, want := range []string{"description {", "html", "employer {", "dossier {", "corporateWebsite", "recruit {", "viewJobUrl", "attributes {"} {
		if !strings.Contains(query, want) {
			t.Errorf("detail query missing %q:\n%s", want, query)
		}
	}

	if job.ID != "indeed:"+detailJobKey || job.SourceJobID != detailJobKey || job.Source != domain.SourceIndeed {
		t.Fatalf("job identity = %+v", job)
	}
	if job.Title != "Senior Backend Engineer" {
		t.Errorf("title = %q", job.Title)
	}
	if !strings.Contains(job.Description, "<p>Own the payments platform end to end.</p>") {
		t.Errorf("description = %q, want the description html", job.Description)
	}
	if job.Employer != "Serve Robotics" {
		t.Errorf("employer = %q", job.Employer)
	}
	if job.Location != "Vancouver, BC" {
		t.Errorf("location = %q", job.Location)
	}
	if job.PostedDate != time.UnixMilli(1790435045000).UTC().Format(time.RFC3339) {
		t.Errorf("posted date = %q", job.PostedDate)
	}
	if job.Compensation == nil || job.Compensation.Currency != "CAD" || job.Compensation.Interval != domain.IntervalYear {
		t.Fatalf("compensation = %+v", job.Compensation)
	}
	if len(job.Compensation.Amounts) != 1 || *job.Compensation.Amounts[0].Min != 180000 || *job.Compensation.Amounts[0].Max != 220000 {
		t.Errorf("compensation amounts = %+v", job.Compensation.Amounts)
	}
	if job.ApplicationURL != "https://jobs.ashbyhq.com/serverobotics/517ca3fd71acddc9" {
		t.Errorf("application url = %q", job.ApplicationURL)
	}
	if job.SourceURL != "https://www.indeed.com/viewjob?jk="+detailJobKey {
		t.Errorf("source url = %q", job.SourceURL)
	}

	if job.Diagnostics == nil || len(job.Diagnostics.SourcePayload) == 0 {
		t.Fatal("detail job has no source payload diagnostics")
	}
	payload := string(job.Diagnostics.SourcePayload)
	for _, want := range []string{`"relativeCompanyPageUrl":"/cmp/Serve-Robotics"`, `"corporateWebsite":"https://www.serverobotics.com"`, `"label":"Full-time"`} {
		if !strings.Contains(payload, want) {
			t.Errorf("detail diagnostics lost %q:\n%s", want, payload)
		}
	}
}

func TestDetailAcceptsCompoundJobID(t *testing.T) {
	r := &recorder{t: t, respond: func(int) *http.Response { return fixtureResponse(t, "detail.json") }}
	if _, err := newSource(r).Detail(context.Background(), &domain.DetailRequest{SourceJobID: "indeed:" + detailJobKey}); err != nil {
		t.Fatalf("detail: %v", err)
	}
	if !strings.Contains(queryText(t, r.body), `jobData(jobKeys: ["`+detailJobKey+`"])`) {
		t.Errorf("compound job id was not stripped into the jobKeys batch:\n%s", queryText(t, r.body))
	}
}

func TestDetailJobNotFound(t *testing.T) {
	r := &recorder{t: t, respond: func(int) *http.Response { return fixtureResponse(t, "detail_empty.json") }}
	_, err := newSource(r).Detail(context.Background(), &domain.DetailRequest{SourceJobID: detailJobKey})
	e := requireErrorCode(t, err, joberrors.ResourceNotFound)
	if e.Retryable {
		t.Errorf("resource not found must not be retryable")
	}
}

func TestDetailSchemaDrift(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{name: "missing jobData connection", body: `{"data":{}}`},
		{name: "missing results array", body: `{"data":{"jobData":{}}}`},
		{name: "null results array", body: `{"data":{"jobData":{"results":null}}}`},
		{name: "result without a job node", body: `{"data":{"jobData":{"results":[{"trackingKey":"t1"}]}}}`},
		{name: "requested job without a title", body: `{"data":{"jobData":{"results":[{"job":{"key":"` + detailJobKey + `"}}]}}}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := &recorder{t: t, respond: func(int) *http.Response { return testutil.JSONResponse(http.StatusOK, tc.body) }}
			_, err := newSource(r).Detail(context.Background(), &domain.DetailRequest{SourceJobID: detailJobKey})
			e := requireErrorCode(t, err, joberrors.APISchemaChanged)
			if e.Category != joberrors.CatAPI {
				t.Errorf("category = %s, want api", e.Category)
			}
		})
	}
}

func TestDetailIgnoresNonMatchingBatchResults(t *testing.T) {
	body := `{"data":{"jobData":{"results":[` +
		`{"job":{"title":"Keyless Decoy"}},` +
		`{"job":{"key":"ffffffffffffffff","title":"Other Job"}},` +
		`{"job":{"key":"` + detailJobKey + `","title":"Senior Backend Engineer","employer":{"name":"Serve Robotics"}}}` +
		`]}}}`
	r := &recorder{t: t, respond: func(int) *http.Response { return testutil.JSONResponse(http.StatusOK, body) }}
	job, err := newSource(r).Detail(context.Background(), &domain.DetailRequest{SourceJobID: detailJobKey})
	if err != nil {
		t.Fatalf("detail: %v", err)
	}
	if job.SourceJobID != detailJobKey || job.Title != "Senior Backend Engineer" || job.Employer != "Serve Robotics" {
		t.Fatalf("batch detail returned the wrong job: %+v", job)
	}
}

func TestDetailRequiresAJobID(t *testing.T) {
	r := &recorder{t: t, respond: func(int) *http.Response { return fixtureResponse(t, "detail.json") }}
	_, err := newSource(r).Detail(context.Background(), &domain.DetailRequest{})
	e := requireErrorCode(t, err, joberrors.InvalidArguments)
	if e.Category != joberrors.CatValidation {
		t.Errorf("category = %s, want validation", e.Category)
	}
	if r.calls != 0 {
		t.Fatalf("an invalid detail request still made %d HTTP calls", r.calls)
	}
}
