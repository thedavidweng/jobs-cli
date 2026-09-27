package e2e

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thedavidweng/jobs-cli/v2/internal/cli"
	"github.com/thedavidweng/jobs-cli/v2/internal/testutil"
)

const indeedNextCursor = "eyJvZmZzZXQiOjI1fQ"

type indeedTransport struct {
	t       *testing.T
	calls   int
	method  string
	url     string
	key     string
	app     string
	co      string
	locale  string
	body    string
	respond func(call int) *http.Response
}

func (tr *indeedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	tr.calls++
	tr.method = req.Method
	tr.url = req.URL.String()
	tr.key = req.Header.Get("indeed-api-key")
	tr.app = req.Header.Get("indeed-app-info")
	tr.co = req.Header.Get("indeed-co")
	tr.locale = req.Header.Get("indeed-locale")
	raw, err := io.ReadAll(req.Body)
	if err != nil {
		tr.t.Fatalf("read request body: %v", err)
	}
	tr.body = string(raw)
	return tr.respond(tr.calls), nil
}

func indeedQuery(t *testing.T, body string) string {
	t.Helper()
	var payload struct {
		Query string `json:"query"`
	}
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		t.Fatalf("outgoing body is not the GraphQL envelope: %v\n%s", err, body)
	}
	if payload.Query == "" {
		t.Fatalf("outgoing body carries no GraphQL query:\n%s", body)
	}
	return payload.Query
}

func indeedFixture(t *testing.T, name string) *http.Response {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "internal", "indeed", "testdata", name))
	if err != nil {
		t.Fatalf("read indeed fixture %s: %v", name, err)
	}
	return testutil.JSONResponse(http.StatusOK, string(data))
}

func runIndeedSearch(t *testing.T, transport http.RoundTripper, args ...string) indeedSearchDoc {
	t.Helper()
	return runIndeedSearchDir(t, t.TempDir(), transport, args...)
}

func runIndeedSearchDir(t *testing.T, configDir string, transport http.RoundTripper, args ...string) indeedSearchDoc {
	t.Helper()
	var stdout, stderr bytes.Buffer
	app := cli.New(&cli.Options{
		Stdout:        &stdout,
		Stderr:        &stderr,
		ConfigDir:     configDir,
		BaseTransport: transport,
	})
	code := app.Run(args)
	if code != 0 {
		t.Fatalf("search exit = %d (stdout: %s, stderr: %s)", code, stdout.String(), stderr.String())
	}
	if lines := strings.Split(strings.TrimSpace(stdout.String()), "\n"); len(lines) != 1 {
		t.Fatalf("stdout must be exactly one JSON document, got %d lines:\n%s", len(lines), stdout.String())
	}
	var doc indeedSearchDoc
	if err := json.Unmarshal(stdout.Bytes(), &doc); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\n%s", err, stdout.String())
	}
	return doc
}

type indeedSearchDoc struct {
	OK   bool             `json:"ok"`
	Data indeedSearchData `json:"data"`
	Meta indeedMeta       `json:"meta"`
}

type indeedSearchData struct {
	Partitions []indeedPartition `json:"partitions"`
}

type indeedPartition struct {
	Source     string              `json:"source"`
	Market     *indeedMarket       `json:"market"`
	Jobs       []indeedJob         `json:"jobs"`
	Pagination *indeedPagination   `json:"pagination"`
	Error      *indeedPartitionErr `json:"error"`
}

type indeedMarket struct {
	Country string `json:"country"`
	Locale  string `json:"locale"`
	Origin  string `json:"origin"`
}

type indeedPartitionErr struct {
	Code string `json:"code"`
}

type indeedJob struct {
	ID             string              `json:"id"`
	Source         string              `json:"source"`
	SourceJobID    string              `json:"source_job_id"`
	Title          string              `json:"title"`
	Employer       string              `json:"employer"`
	Location       string              `json:"location"`
	Workplace      string              `json:"workplace"`
	Remote         bool                `json:"remote"`
	Description    string              `json:"description"`
	PostedDate     string              `json:"posted_date"`
	Compensation   *indeedCompensation `json:"compensation"`
	SourceURL      string              `json:"source_url"`
	ApplicationURL string              `json:"application_url"`
	Diagnostics    *indeedDiagnostics  `json:"diagnostics"`
}

type indeedCompensation struct {
	Amounts  []indeedAmount `json:"amounts"`
	Currency string         `json:"currency"`
	Interval string         `json:"interval"`
	Summary  string         `json:"summary"`
}

type indeedAmount struct {
	Kind string   `json:"kind"`
	Min  *float64 `json:"min"`
	Max  *float64 `json:"max"`
}

type indeedDiagnostics struct {
	SourcePayload json.RawMessage `json:"source_payload"`
}

type indeedPagination struct {
	Limit      int                     `json:"limit"`
	HasMore    bool                    `json:"has_more"`
	NextCursor string                  `json:"next_cursor"`
	Native     *indeedNativePagination `json:"native"`
}

type indeedNativePagination struct {
	Kind   string `json:"kind"`
	Cursor string `json:"cursor"`
}

type indeedMeta struct {
	Partitions []indeedPartitionMeta `json:"partitions"`
}

type indeedPartitionMeta struct {
	Source     string            `json:"source"`
	OK         bool              `json:"ok"`
	Pagination *indeedPagination `json:"pagination"`
}

func TestSearchIndeedCommandReturnsNormalizedPartition(t *testing.T) {
	transport := &indeedTransport{
		t:       t,
		respond: func(int) *http.Response { return indeedFixture(t, "search.json") },
	}
	doc := runIndeedSearch(t, transport,
		"--json", "--full", "search", "--query", "software engineer", "--location", "Vancouver, BC", "--source", "indeed")

	if !doc.OK {
		t.Fatalf("envelope ok = false")
	}
	if transport.calls != 1 {
		t.Fatalf("indeed HTTP calls = %d, want a single source request", transport.calls)
	}
	if transport.method != http.MethodPost || transport.url != "https://apis.indeed.com/graphql" {
		t.Fatalf("outgoing request = %s %s, want POST https://apis.indeed.com/graphql", transport.method, transport.url)
	}
	if transport.key == "" {
		t.Fatal("outgoing request is missing the indeed-api-key identity header")
	}
	if transport.app != "appv=193.1; appid=com.indeed.jobsearch; osv=16.6.1; os=ios; dtype=phone" {
		t.Fatalf("indeed-app-info = %q", transport.app)
	}
	if transport.co != "CA" || transport.locale != "en-CA" {
		t.Fatalf("market headers = %q/%q, want CA/en-CA inferred from --location", transport.co, transport.locale)
	}
	for _, want := range []string{`what: "software engineer"`, `location: {where: "Vancouver, BC"`, "jobSearch("} {
		if !strings.Contains(indeedQuery(t, transport.body), want) {
			t.Errorf("outgoing GraphQL query missing %q:\n%s", want, indeedQuery(t, transport.body))
		}
	}

	if len(doc.Data.Partitions) != 1 {
		t.Fatalf("partitions = %d, want 1", len(doc.Data.Partitions))
	}
	partition := doc.Data.Partitions[0]
	if partition.Source != "indeed" || partition.Error != nil {
		t.Fatalf("partition = %+v, want a successful indeed partition", partition)
	}
	if partition.Market == nil || *partition.Market != (indeedMarket{Country: "CA", Locale: "en-CA", Origin: "location"}) {
		t.Fatalf("partition market = %+v, want CA/en-CA from the location", partition.Market)
	}
	if len(partition.Jobs) != 3 {
		t.Fatalf("indeed jobs = %d, want 3", len(partition.Jobs))
	}

	first := partition.Jobs[0]
	if first.ID != "indeed:b8ef297959c26ac5" || first.Source != "indeed" || first.SourceJobID != "b8ef297959c26ac5" {
		t.Fatalf("first job identity = %+v", first)
	}
	if first.Title != "Sr. Embedded Software Engineer, Robotics Platform" {
		t.Errorf("first title = %q", first.Title)
	}
	if first.Employer != "Serve Robotics" || first.Location != "Vancouver, BC" {
		t.Errorf("first employer/location = %q/%q", first.Employer, first.Location)
	}
	if first.Workplace != "remote" || !first.Remote {
		t.Errorf("first workplace = %q remote=%v, want remote", first.Workplace, first.Remote)
	}
	if first.SourceURL != "https://ca.indeed.com/viewjob?jk=b8ef297959c26ac5" {
		t.Errorf("first source url = %q", first.SourceURL)
	}
	if first.ApplicationURL != "https://jobs.ashbyhq.com/serverobotics/0b9f3986-9ae6-4361-85b5-2e1edc6e10e2?utm_source=j20ZWL4oeG" {
		t.Errorf("first application url = %q, want recruit.viewJobUrl", first.ApplicationURL)
	}
	if first.Compensation == nil {
		t.Fatal("first job has no compensation")
	}
	if first.Compensation.Currency != "CAD" || first.Compensation.Interval != "year" || first.Compensation.Summary != "$167,800 - $204,000 a year" {
		t.Errorf("first compensation = %+v", first.Compensation)
	}
	if len(first.Compensation.Amounts) != 1 || first.Compensation.Amounts[0].Kind != "range" ||
		*first.Compensation.Amounts[0].Min != 167800 || *first.Compensation.Amounts[0].Max != 204000 {
		t.Errorf("first compensation amounts = %+v", first.Compensation.Amounts)
	}
	if first.Diagnostics == nil || len(first.Diagnostics.SourcePayload) == 0 {
		t.Error("first job lost its diagnostics source payload")
	}

	if partition.Pagination == nil {
		t.Fatal("indeed partition has no pagination")
	}
	if partition.Pagination.Limit != 25 || !partition.Pagination.HasMore || partition.Pagination.NextCursor != indeedNextCursor {
		t.Fatalf("partition pagination = %+v", partition.Pagination)
	}
	if partition.Pagination.Native == nil || partition.Pagination.Native.Kind != "cursor" || partition.Pagination.Native.Cursor != indeedNextCursor {
		t.Fatalf("partition native pagination = %+v", partition.Pagination.Native)
	}

	if len(doc.Meta.Partitions) != 1 {
		t.Fatalf("meta partitions = %+v", doc.Meta.Partitions)
	}
	meta := doc.Meta.Partitions[0]
	if meta.Source != "indeed" || !meta.OK {
		t.Fatalf("meta partition = %+v", meta)
	}
	if meta.Pagination == nil || !meta.Pagination.HasMore || meta.Pagination.NextCursor != indeedNextCursor {
		t.Fatalf("meta partition pagination = %+v", meta.Pagination)
	}
}

func TestSearchIndeedCommandContinuesWithCursor(t *testing.T) {
	transport := &indeedTransport{
		t:       t,
		respond: func(int) *http.Response { return indeedFixture(t, "search_last_page.json") },
	}
	doc := runIndeedSearch(t, transport,
		"--json", "search", "--query", "software engineer", "--source", "indeed", "--cursor", indeedNextCursor, "--country", "CA")

	if !strings.Contains(indeedQuery(t, transport.body), `cursor: "`+indeedNextCursor+`"`) {
		t.Errorf("continuation query did not carry the cursor:\n%s", indeedQuery(t, transport.body))
	}
	if len(doc.Data.Partitions) != 1 || doc.Data.Partitions[0].Source != "indeed" {
		t.Fatalf("partitions = %+v", doc.Data.Partitions)
	}
	partition := doc.Data.Partitions[0]
	if partition.Pagination == nil || partition.Pagination.HasMore || partition.Pagination.NextCursor != "" {
		t.Fatalf("last page pagination = %+v", partition.Pagination)
	}
	if len(partition.Jobs) != 1 || partition.Jobs[0].ID != "indeed:eeee1111f222f333" {
		t.Fatalf("last page jobs = %+v", partition.Jobs)
	}
}

func TestSearchIndeedMarketComesFromFlagsLocationEnvAndProfile(t *testing.T) {
	t.Setenv("JOBS_COUNTRY", "")
	t.Setenv("JOBS_LOCALE", "")
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("profiles:\n  default:\n    country: CA\n    locale: en-CA\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	search := func(t *testing.T, args ...string) (*indeedTransport, indeedPartition) {
		t.Helper()
		transport := &indeedTransport{
			t:       t,
			respond: func(int) *http.Response { return indeedFixture(t, "search.json") },
		}
		doc := runIndeedSearchDir(t, dir, transport, append([]string{"--json", "search", "-q", "software engineer", "--source", "indeed"}, args...)...)
		if len(doc.Data.Partitions) != 1 || len(doc.Data.Partitions[0].Jobs) != 3 || doc.Data.Partitions[0].Market == nil {
			t.Fatalf("partitions = %+v, want one indeed partition that echoes its market", doc.Data.Partitions)
		}
		return transport, doc.Data.Partitions[0]
	}

	transport, partition := search(t)
	if transport.co != "CA" || transport.locale != "en-CA" || partition.Market.Origin != "profile" {
		t.Fatalf("market = %q/%q echoed as %+v, want CA/en-CA from the profile", transport.co, transport.locale, partition.Market)
	}
	if got := partition.Jobs[0].SourceURL; got != "https://ca.indeed.com/viewjob?jk=b8ef297959c26ac5" {
		t.Fatalf("job source url = %q, want the ca.indeed.com host of the job's country", got)
	}

	t.Run("flag overrides location, env, and profile", func(t *testing.T) {
		t.Setenv("JOBS_COUNTRY", "GB")
		transport, partition := search(t, "-l", "Vancouver, BC", "--country", "US")
		if transport.co != "US" || transport.locale != "en-US" || partition.Market.Origin != "flag" {
			t.Fatalf("market = %q/%q from %s, want the flag-provided US", transport.co, transport.locale, partition.Market.Origin)
		}
	})

	t.Run("location overrides env and profile", func(t *testing.T) {
		t.Setenv("JOBS_COUNTRY", "GB")
		transport, partition := search(t, "-l", "Seattle, WA")
		if transport.co != "US" || transport.locale != "en-US" || partition.Market.Origin != "location" {
			t.Fatalf("market = %q/%q from %s, want US inferred from the location", transport.co, transport.locale, partition.Market.Origin)
		}
	})

	t.Run("env overrides profile", func(t *testing.T) {
		t.Setenv("JOBS_COUNTRY", "GB")
		t.Setenv("JOBS_LOCALE", "en-GB")
		transport, partition := search(t, "-l", "London")
		if transport.co != "GB" || transport.locale != "en-GB" || partition.Market.Origin != "env" {
			t.Fatalf("market = %q/%q from %s, want GB/en-GB from the environment", transport.co, transport.locale, partition.Market.Origin)
		}
	})
}

func TestSearchIndeedWithoutAMarketSendsNoRequest(t *testing.T) {
	t.Setenv("JOBS_COUNTRY", "")
	transport := &indeedTransport{
		t:       t,
		respond: func(int) *http.Response { return indeedFixture(t, "search.json") },
	}
	var stdout, stderr bytes.Buffer
	app := cli.New(&cli.Options{Stdout: &stdout, Stderr: &stderr, ConfigDir: t.TempDir(), BaseTransport: transport})
	code := app.Run([]string{"--json", "search", "-q", "software engineer", "-l", "Vancouver", "--source", "indeed"})
	if code != 2 {
		t.Fatalf("exit = %d, want 2 (stdout: %s, stderr: %s)", code, stdout.String(), stderr.String())
	}
	var doc struct {
		OK    bool                `json:"ok"`
		Error *indeedPartitionErr `json:"error"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &doc); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\n%s", err, stdout.String())
	}
	if doc.OK || doc.Error == nil || doc.Error.Code != "MARKET_REQUIRED" {
		t.Fatalf("envelope = %+v, want MARKET_REQUIRED", doc)
	}
	if transport.calls != 0 {
		t.Fatalf("indeed HTTP calls = %d, want none without a market", transport.calls)
	}
}
