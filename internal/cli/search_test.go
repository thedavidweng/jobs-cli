package cli_test

import (
	"strings"
	"testing"

	"github.com/thedavidweng/jobs-cli/internal/domain"
	"github.com/thedavidweng/jobs-cli/internal/errors"
	"github.com/thedavidweng/jobs-cli/internal/testutil"
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

	out, _, code := h.run("--json", "search", "-q", "go")
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
	out, _, code := h.run("--json", "search", "-q", "go")
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

	out, _, code = h.run("--json", "search", "-q", "go", "--source", "indeed", "--cursor", "abc")
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

	_, _, code := h.run("--json", "search", "-q", "go")
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
	if _, _, code := h.run("--json", "search", "-q", "go"); code != 0 {
		t.Fatalf("exit = %d", code)
	}
	if len(indeed.Requests) != 1 || len(linkedin.Requests) != 1 {
		t.Fatalf("default fan-out = indeed:%d linkedin:%d, want 1 each", len(indeed.Requests), len(linkedin.Requests))
	}
}
