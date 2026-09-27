package cli_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/thedavidweng/jobs-cli/v2/internal/domain"
	"github.com/thedavidweng/jobs-cli/v2/internal/errors"
	"github.com/thedavidweng/jobs-cli/v2/internal/testutil"
)

func TestSearchReturnsPartitionsAndPartialSuccess(t *testing.T) {
	indeed := &testutil.FakeSource{
		SourceName: domain.SourceIndeed,
		Partition: &domain.SearchPartition{
			Source: domain.SourceIndeed,
			Jobs:   []domain.Job{*fakeJob(domain.SourceIndeed, "a"), *fakeJob(domain.SourceIndeed, "b")},
			Pagination: &domain.Pagination{
				Limit:      25,
				HasMore:    true,
				NextCursor: "cursor-2",
				Native:     &domain.NativePagination{Kind: domain.PaginationCursor, Cursor: "cursor-2"},
			},
		},
	}
	linkedin := &testutil.FakeSource{
		SourceName: domain.SourceLinkedIn,
		SearchErr:  errors.New(errors.SourceUnavailable, "LinkedIn Guest is unavailable", errors.CatNetwork, true, nil),
	}
	reg := testutil.NewRegistry(
		map[domain.Source]domain.SourceAdapter{domain.SourceIndeed: indeed, domain.SourceLinkedIn: linkedin},
		&testutil.FakeResolver{},
		map[domain.ApplicationProvider]domain.ApplyProvider{},
	)
	h := newHarness(t).useRegistry(reg)
	t.Setenv("JOBS_LOCALE", "")

	out, _, code := h.run("--json", "search", "-q", "go", "--country", "US")
	if code != 0 {
		t.Fatalf("partial success must exit 0, got %d", code)
	}
	doc := decodeEnvelope(t, out)
	partitions, ok := doc.Data["partitions"].([]any)
	if !ok || len(partitions) != 2 {
		t.Fatalf("data.partitions = %#v, want 2 partitions", doc.Data["partitions"])
	}
	first, fok := partitions[0].(map[string]any)
	if !fok {
		t.Fatalf("partitions[0] = %#v", partitions[0])
	}
	if first["source"] != "indeed" {
		t.Fatalf("first partition source = %v", first["source"])
	}
	if market, ok := first["market"].(map[string]any); !ok || market["country"] != "US" || market["locale"] != "en-US" || market["origin"] != "flag" {
		t.Fatalf("indeed partition market = %#v, want US/en-US from the flag", first["market"])
	}
	jobs, ok := first["jobs"].([]any)
	if !ok || len(jobs) != 2 {
		t.Fatalf("indeed partition jobs = %#v", first["jobs"])
	}
	if _, hasError := first["error"]; hasError {
		t.Fatal("successful partition carries an error")
	}
	second, sok := partitions[1].(map[string]any)
	if !sok {
		t.Fatalf("partitions[1] = %#v", partitions[1])
	}
	partitionError, ok := second["error"].(map[string]any)
	if !ok || partitionError["code"] != "SOURCE_UNAVAILABLE" {
		t.Fatalf("failed partition error = %#v", second["error"])
	}
	if _, hasMarket := second["market"]; hasMarket {
		t.Fatalf("linkedin partition carries a market: %#v", second["market"])
	}

	if len(doc.Meta.Partitions) != 2 {
		t.Fatalf("meta.partitions = %#v", doc.Meta.Partitions)
	}
	if !doc.Meta.Partitions[0].OK || doc.Meta.Partitions[1].OK {
		t.Fatalf("meta.partitions ok flags = %#v", doc.Meta.Partitions)
	}
	if doc.Meta.Partitions[0].Pagination["has_more"] != true {
		t.Fatalf("per-partition pagination missing: %#v", doc.Meta.Partitions[0].Pagination)
	}
	if len(doc.Meta.Warnings) == 0 {
		t.Fatal("meta.warnings empty for partial success")
	}
}

func TestSearchFailsOnlyWhenEverySourceFails(t *testing.T) {
	reg := testutil.NewRegistry(
		map[domain.Source]domain.SourceAdapter{
			domain.SourceIndeed:   &testutil.FakeSource{SourceName: domain.SourceIndeed, SearchErr: errors.New(errors.SourceUnavailable, "indeed down", errors.CatNetwork, true, nil)},
			domain.SourceLinkedIn: &testutil.FakeSource{SourceName: domain.SourceLinkedIn, SearchErr: errors.New(errors.SourceUnavailable, "linkedin down", errors.CatNetwork, true, nil)},
		},
		&testutil.FakeResolver{},
		map[domain.ApplicationProvider]domain.ApplyProvider{},
	)
	h := newHarness(t).useRegistry(reg)
	out, _, code := h.run("--json", "search", "-q", "go", "--country", "US")
	doc := decodeEnvelope(t, out)
	requireCode(t, &doc, "SOURCE_UNAVAILABLE", 5, code)
}

func TestSearchContinuationRules(t *testing.T) {
	indeed := &testutil.FakeSource{SourceName: domain.SourceIndeed, Partition: &domain.SearchPartition{Source: domain.SourceIndeed, Jobs: []domain.Job{}}}
	linkedin := &testutil.FakeSource{SourceName: domain.SourceLinkedIn, Partition: &domain.SearchPartition{Source: domain.SourceLinkedIn, Jobs: []domain.Job{}}}
	reg := testutil.NewRegistry(
		map[domain.Source]domain.SourceAdapter{domain.SourceIndeed: indeed, domain.SourceLinkedIn: linkedin},
		&testutil.FakeResolver{},
		map[domain.ApplicationProvider]domain.ApplyProvider{},
	)
	h := newHarness(t).useRegistry(reg)

	out, _, code := h.run("--json", "search", "-q", "go", "--offset", "10")
	doc := decodeEnvelope(t, out)
	requireCode(t, &doc, "INVALID_ARGUMENTS", 2, code)
	if !strings.Contains(doc.Error.Message, "exactly one --source") {
		t.Fatalf("message = %q", doc.Error.Message)
	}

	out, _, code = h.run("--json", "search", "-q", "go", "--source", "indeed", "--source", "linkedin", "--cursor", "abc")
	doc = decodeEnvelope(t, out)
	requireCode(t, &doc, "INVALID_ARGUMENTS", 2, code)

	out, _, code = h.run("--json", "search", "-q", "go", "--source", "indeed", "--cursor", "abc", "--country", "US")
	if code != 0 {
		t.Fatalf("single-source continuation exit = %d (%s)", code, out)
	}
	if len(indeed.Requests) != 1 {
		t.Fatalf("indeed search calls = %d, want 1", len(indeed.Requests))
	}
	if indeed.Requests[0].Cursor != "abc" {
		t.Fatalf("cursor propagated = %q, want abc", indeed.Requests[0].Cursor)
	}
	if len(linkedin.Requests) != 0 {
		t.Fatalf("linkedin was called during a single-source continuation")
	}
}

func TestSearchUsesProfileSources(t *testing.T) {
	indeed := &testutil.FakeSource{SourceName: domain.SourceIndeed, Partition: &domain.SearchPartition{Source: domain.SourceIndeed, Jobs: []domain.Job{}}}
	linkedin := &testutil.FakeSource{SourceName: domain.SourceLinkedIn, Partition: &domain.SearchPartition{Source: domain.SourceLinkedIn, Jobs: []domain.Job{}}}
	h := newHarness(t).useRegistry(testutil.NewRegistry(
		map[domain.Source]domain.SourceAdapter{domain.SourceIndeed: indeed, domain.SourceLinkedIn: linkedin},
		&testutil.FakeResolver{},
		map[domain.ApplicationProvider]domain.ApplyProvider{},
	))
	h.writeConfig("default_profile: work\nprofiles:\n  work:\n    sources:\n      - indeed\n")

	_, _, code := h.run("--json", "search", "-q", "go", "--country", "US")
	if code != 0 {
		t.Fatalf("exit = %d", code)
	}
	if len(indeed.Requests) != 1 {
		t.Fatalf("indeed search calls = %d, want 1", len(indeed.Requests))
	}
	if len(linkedin.Requests) != 0 {
		t.Fatalf("profile sources were ignored: linkedin was called")
	}
}

func TestSearchDefaultSourcesAreIndeedAndLinkedIn(t *testing.T) {
	indeed := &testutil.FakeSource{SourceName: domain.SourceIndeed, Partition: &domain.SearchPartition{Source: domain.SourceIndeed, Jobs: []domain.Job{}}}
	linkedin := &testutil.FakeSource{SourceName: domain.SourceLinkedIn, Partition: &domain.SearchPartition{Source: domain.SourceLinkedIn, Jobs: []domain.Job{}}}
	h := newHarness(t).useRegistry(testutil.NewRegistry(
		map[domain.Source]domain.SourceAdapter{domain.SourceIndeed: indeed, domain.SourceLinkedIn: linkedin},
		&testutil.FakeResolver{},
		map[domain.ApplicationProvider]domain.ApplyProvider{},
	))
	if _, _, code := h.run("--json", "search", "-q", "go", "--country", "US"); code != 0 {
		t.Fatalf("exit = %d", code)
	}
	if len(indeed.Requests) != 1 || len(linkedin.Requests) != 1 {
		t.Fatalf("default fan-out = indeed:%d linkedin:%d, want 1 each", len(indeed.Requests), len(linkedin.Requests))
	}
}

type searchPartitionDoc struct {
	Source string         `json:"source"`
	Market *domain.Market `json:"market"`
	Error  *errorDoc      `json:"error"`
}

func decodePartitions(t *testing.T, out string) []searchPartitionDoc {
	t.Helper()
	decodeEnvelope(t, out)
	var doc struct {
		Data struct {
			Partitions []searchPartitionDoc `json:"partitions"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("decode partitions: %v\n%s", err, out)
	}
	return doc.Data.Partitions
}

// marketHarness clears the market environment so only each test's own inputs
// decide the market.
func marketHarness(t *testing.T) (h *harness, indeed, linkedin *testutil.FakeSource) {
	t.Helper()
	t.Setenv("JOBS_COUNTRY", "")
	t.Setenv("JOBS_LOCALE", "")
	indeed = &testutil.FakeSource{SourceName: domain.SourceIndeed}
	linkedin = &testutil.FakeSource{SourceName: domain.SourceLinkedIn}
	h = newHarness(t).useRegistry(testutil.NewRegistry(
		map[domain.Source]domain.SourceAdapter{domain.SourceIndeed: indeed, domain.SourceLinkedIn: linkedin},
		&testutil.FakeResolver{},
		map[domain.ApplicationProvider]domain.ApplyProvider{},
	))
	return h, indeed, linkedin
}

func TestSearchWithoutAMarketFailsOnlyTheIndeedPartition(t *testing.T) {
	h, indeed, linkedin := marketHarness(t)
	out, errOut, code := h.run("--json", "search", "-q", "go", "-l", "London")
	if code != 0 {
		t.Fatalf("exit = %d, want 0 because linkedin still ran (stderr: %s)", code, errOut)
	}
	if len(indeed.Requests) != 0 {
		t.Fatalf("indeed was searched without a market: %+v", indeed.Requests)
	}
	if len(linkedin.Requests) != 1 || linkedin.Requests[0].Market != nil {
		t.Fatalf("linkedin requests = %+v, want one request without a market", linkedin.Requests)
	}
	partitions := decodePartitions(t, out)
	if len(partitions) != 2 {
		t.Fatalf("partitions = %+v", partitions)
	}
	failed := partitions[0]
	if failed.Source != "indeed" || failed.Error == nil || failed.Error.Code != "MARKET_REQUIRED" || failed.Error.Category != "validation" {
		t.Fatalf("indeed partition = %+v, want a MARKET_REQUIRED validation error", failed)
	}
	if !strings.Contains(failed.Error.Message, `"London"`) || !strings.Contains(failed.Error.Message, "--country") {
		t.Fatalf("message = %q, want the location and the --country remedy", failed.Error.Message)
	}
	if failed.Market != nil || partitions[1].Error != nil {
		t.Fatalf("partitions = %+v", partitions)
	}
	if doc := decodeEnvelope(t, out); len(doc.Meta.Warnings) != 1 || !strings.Contains(doc.Meta.Warnings[0], "MARKET_REQUIRED") {
		t.Fatalf("meta.warnings = %#v, want the indeed MARKET_REQUIRED warning", doc.Meta.Warnings)
	}
}

func TestSearchIndeedAloneWithoutAMarketExitsWithMarketRequired(t *testing.T) {
	h, indeed, _ := marketHarness(t)
	out, _, code := h.run("--json", "search", "-q", "go", "--source", "indeed")
	doc := decodeEnvelope(t, out)
	requireCode(t, &doc, "MARKET_REQUIRED", 2, code)
	if doc.Error.Category != "validation" || doc.Error.Retryable {
		t.Fatalf("error = %+v, want a non-retryable validation error", doc.Error)
	}
	if len(indeed.Requests) != 0 {
		t.Fatalf("indeed was searched without a market")
	}
}

func TestSearchResolvesTheIndeedMarket(t *testing.T) {
	cases := []struct {
		name    string
		env     map[string]string
		profile string
		args    []string
		want    domain.Market
	}{
		{
			name: "location suffix",
			args: []string{"-l", "Vancouver, BC"},
			want: domain.Market{Country: "CA", Locale: "en-CA", Origin: domain.MarketFromLocation},
		},
		{
			name: "flag beats the location",
			args: []string{"-l", "Vancouver, BC", "--country", "us"},
			want: domain.Market{Country: "US", Locale: "en-US", Origin: domain.MarketFromFlag},
		},
		{
			name:    "location beats the environment and profile",
			env:     map[string]string{"JOBS_COUNTRY": "GB"},
			profile: "CA",
			args:    []string{"-l", "Seattle, WA"},
			want:    domain.Market{Country: "US", Locale: "en-US", Origin: domain.MarketFromLocation},
		},
		{
			name:    "environment beats the profile",
			env:     map[string]string{"JOBS_COUNTRY": "GB", "JOBS_LOCALE": "en-GB"},
			profile: "CA",
			args:    []string{"-l", "London"},
			want:    domain.Market{Country: "GB", Locale: "en-GB", Origin: domain.MarketFromEnv},
		},
		{
			name:    "profile is the last resort",
			profile: "CA",
			want:    domain.Market{Country: "CA", Locale: "fr-CA", Origin: domain.MarketFromProfile},
		},
		{
			name: "configured locale outside the market is ignored",
			env:  map[string]string{"JOBS_LOCALE": "fr-CA"},
			args: []string{"--country", "US"},
			want: domain.Market{Country: "US", Locale: "en-US", Origin: domain.MarketFromFlag},
		},
		{
			name: "locale flag always wins",
			env:  map[string]string{"JOBS_LOCALE": "en-CA"},
			args: []string{"--country", "CA", "--locale", "fr-CA"},
			want: domain.Market{Country: "CA", Locale: "fr-CA", Origin: domain.MarketFromFlag},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h, indeed, linkedin := marketHarness(t)
			for key, value := range tc.env {
				t.Setenv(key, value)
			}
			if tc.profile != "" {
				h.writeConfig("profiles:\n  default:\n    country: " + tc.profile + "\n    locale: fr-CA\n")
			}
			out, errOut, code := h.run(append([]string{"--json", "search", "-q", "go"}, tc.args...)...)
			if code != 0 {
				t.Fatalf("exit = %d (stdout: %s, stderr: %s)", code, out, errOut)
			}
			if len(indeed.Requests) != 1 || indeed.Requests[0].Market == nil || *indeed.Requests[0].Market != tc.want {
				t.Fatalf("indeed requests = %+v, want market %+v", indeed.Requests, tc.want)
			}
			if len(linkedin.Requests) != 1 || linkedin.Requests[0].Market != nil {
				t.Fatalf("linkedin requests = %+v, want one request without a market", linkedin.Requests)
			}
			partitions := decodePartitions(t, out)
			if len(partitions) != 2 || partitions[0].Market == nil || *partitions[0].Market != tc.want || partitions[1].Market != nil {
				t.Fatalf("partitions = %+v, want the indeed partition to echo %+v", partitions, tc.want)
			}
		})
	}
}

func TestSearchRejectsACountryOutsideTheIndeedMarkets(t *testing.T) {
	h, indeed, _ := marketHarness(t)
	out, _, code := h.run("--json", "search", "-q", "go", "--country", "XX")
	if code != 0 {
		t.Fatalf("exit = %d, want 0 because linkedin still ran", code)
	}
	partitions := decodePartitions(t, out)
	if len(partitions) != 2 || partitions[0].Error == nil || partitions[0].Error.Code != "INVALID_ARGUMENTS" || partitions[1].Error != nil {
		t.Fatalf("partitions = %+v, want only the indeed partition to fail", partitions)
	}
	if !strings.Contains(partitions[0].Error.Message, `--country "XX"`) {
		t.Fatalf("message = %q", partitions[0].Error.Message)
	}

	out, _, code = h.run("--json", "search", "-q", "go", "--country", "XX", "--source", "indeed")
	doc := decodeEnvelope(t, out)
	requireCode(t, &doc, "INVALID_ARGUMENTS", 2, code)
	if len(indeed.Requests) != 0 {
		t.Fatalf("indeed was searched with an unsupported market")
	}
}

func TestSearchHumanOutputNamesTheMarket(t *testing.T) {
	h, _, _ := marketHarness(t)
	out, _, code := h.run("search", "-q", "go", "-l", "Vancouver, BC")
	if code != 0 {
		t.Fatalf("exit = %d", code)
	}
	for _, want := range []string{"== indeed (0 jobs, market CA from --location) ==", "== linkedin (0 jobs) =="} {
		if !strings.Contains(out, want) {
			t.Errorf("human output missing %q:\n%s", want, out)
		}
	}
}
