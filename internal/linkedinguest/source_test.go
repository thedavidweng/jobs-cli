package linkedinguest_test

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/thedavidweng/jobs-cli/internal/domain"
	joberrors "github.com/thedavidweng/jobs-cli/internal/errors"
	"github.com/thedavidweng/jobs-cli/internal/linkedinguest"
	"github.com/thedavidweng/jobs-cli/internal/testutil"
)

func fixture(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return string(data)
}

func serveFixture(t *testing.T, name string) *http.Client {
	t.Helper()
	body := fixture(t, name)
	return testutil.NewClient(func(*http.Request) (*http.Response, error) {
		return testutil.HTMLResponse(http.StatusOK, body), nil
	})
}

func requireErrorCode(t *testing.T, err error, code joberrors.Code) *joberrors.Error {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error %s, got nil", code)
	}
	jobErr := joberrors.From(err)
	if jobErr.Code != code {
		t.Fatalf("error code = %s (%v), want %s", jobErr.Code, err, code)
	}
	return jobErr
}

func TestSearchSendsGuestRequestParameters(t *testing.T) {
	var got *http.Request
	client := testutil.NewClient(func(req *http.Request) (*http.Response, error) {
		got = req
		return testutil.HTMLResponse(http.StatusOK, fixture(t, "search_page_1.html")), nil
	})
	remote := true
	partition, err := linkedinguest.NewSource(client).Search(context.Background(), &domain.SearchRequest{
		Keywords: "golang engineer",
		Location: "Vancouver, BC",
		Radius:   50,
		Remote:   &remote,
		Limit:    25,
	})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if got == nil {
		t.Fatal("no request was made")
	}
	if got.Method != http.MethodGet {
		t.Errorf("method = %s, want GET", got.Method)
	}
	if got.URL.Scheme != "https" || got.URL.Host != "www.linkedin.com" {
		t.Errorf("target = %s, want https://www.linkedin.com", got.URL)
	}
	if got.URL.Path != "/jobs-guest/jobs/api/seeMoreJobPostings/search" {
		t.Errorf("path = %s", got.URL.Path)
	}
	query := got.URL.Query()
	for key, want := range map[string]string{
		"keywords": "golang engineer",
		"location": "Vancouver, BC",
		"distance": "50",
		"f_WT":     "2",
		"pageNum":  "0",
		"start":    "0",
	} {
		if value := query.Get(key); value != want {
			t.Errorf("param %s = %q, want %q", key, value, want)
		}
	}
	if got.Header.Get("User-Agent") == "" {
		t.Error("guest request is missing a user-agent")
	}
	if got.Header.Get("Accept") == "" {
		t.Error("guest request is missing an accept header")
	}
	if partition.Source != domain.SourceLinkedIn {
		t.Errorf("partition source = %s", partition.Source)
	}
}

func TestSearchParsesJobCards(t *testing.T) {
	partition, err := linkedinguest.NewSource(serveFixture(t, "search_page_1.html")).Search(context.Background(), &domain.SearchRequest{Keywords: "go"})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(partition.Jobs) != 3 {
		t.Fatalf("jobs = %d, want 3", len(partition.Jobs))
	}

	first := partition.Jobs[0]
	for field, got := range map[string]string{
		"ID":          first.ID,
		"SourceJobID": first.SourceJobID,
		"Title":       first.Title,
		"Employer":    first.Employer,
		"Location":    first.Location,
		"PostedDate":  first.PostedDate,
		"SourceURL":   first.SourceURL,
	} {
		want := map[string]string{
			"ID":          "linkedin:3891234567",
			"SourceJobID": "3891234567",
			"Title":       "Software Engineer",
			"Employer":    "Acme",
			"Location":    "Vancouver, British Columbia, Canada",
			"PostedDate":  "2026-05-01",
			"SourceURL":   "https://www.linkedin.com/jobs/view/3891234567",
		}[field]
		if got != want {
			t.Errorf("%s = %q, want %q", field, got, want)
		}
	}
	if first.Source != domain.SourceLinkedIn {
		t.Errorf("source = %s", first.Source)
	}
	if first.Workplace != domain.WorkplaceUnknown || first.Remote {
		t.Errorf("workplace = %s remote = %v, want unknown/onsite-neutral", first.Workplace, first.Remote)
	}

	second := partition.Jobs[1]
	if second.Title != "Senior Platform Engineer (Remote)" || second.Employer != "Globex" {
		t.Fatalf("second card = %+v", second)
	}
	if second.PostedDate != "2026-05-04" {
		t.Errorf("second posted date = %q", second.PostedDate)
	}
	if second.Workplace != domain.WorkplaceRemote || !second.Remote {
		t.Errorf("second workplace = %s remote = %v, want remote", second.Workplace, second.Remote)
	}

	third := partition.Jobs[2]
	if third.SourceJobID != "3900000001" {
		t.Errorf("third id from href = %q, want 3900000001", third.SourceJobID)
	}
	if third.SourceURL != "https://www.linkedin.com/jobs/view/3900000001" {
		t.Errorf("third source url = %q", third.SourceURL)
	}
	if third.Workplace != domain.WorkplaceRemote || !third.Remote {
		t.Errorf("third workplace = %s remote = %v, want remote from location", third.Workplace, third.Remote)
	}
}

func TestSearchPaginationRoundtrip(t *testing.T) {
	var starts []string
	client := testutil.NewClient(func(req *http.Request) (*http.Response, error) {
		start := req.URL.Query().Get("start")
		starts = append(starts, start)
		switch start {
		case "0":
			return testutil.HTMLResponse(http.StatusOK, fixture(t, "search_page_1.html")), nil
		case "3":
			return testutil.HTMLResponse(http.StatusOK, fixture(t, "search_page_2.html")), nil
		default:
			return testutil.HTMLResponse(http.StatusOK, "\n"), nil
		}
	})
	source := linkedinguest.NewSource(client)
	ctx := context.Background()

	first, err := source.Search(ctx, &domain.SearchRequest{Keywords: "go", Limit: 25})
	if err != nil {
		t.Fatalf("first page: %v", err)
	}
	if len(first.Jobs) != 3 {
		t.Fatalf("first page jobs = %d, want 3", len(first.Jobs))
	}
	if first.Pagination == nil || !first.Pagination.HasMore {
		t.Fatalf("first page pagination = %+v, want has_more", first.Pagination)
	}
	if first.Pagination.NextCursor != "3" {
		t.Fatalf("first page next cursor = %q, want 3", first.Pagination.NextCursor)
	}
	if first.Pagination.Native == nil || first.Pagination.Native.Kind != domain.PaginationStartCount || first.Pagination.Native.Start != 0 {
		t.Fatalf("first page native pagination = %+v", first.Pagination.Native)
	}
	if first.Pagination.Native.Count != 3 {
		t.Fatalf("first page native count = %d, want the 3 cards the provider returned", first.Pagination.Native.Count)
	}

	second, err := source.Search(ctx, &domain.SearchRequest{Keywords: "go", Limit: 25, Cursor: first.Pagination.NextCursor})
	if err != nil {
		t.Fatalf("second page: %v", err)
	}
	if len(second.Jobs) != 2 {
		t.Fatalf("second page jobs = %d, want 2", len(second.Jobs))
	}
	seen := map[string]bool{}
	for _, job := range append(append([]domain.Job{}, first.Jobs...), second.Jobs...) {
		if seen[job.ID] {
			t.Errorf("job %s appeared on two pages", job.ID)
		}
		seen[job.ID] = true
	}

	third, err := source.Search(ctx, &domain.SearchRequest{Keywords: "go", Offset: 5})
	if err != nil {
		t.Fatalf("third page: %v", err)
	}
	if len(third.Jobs) != 0 {
		t.Fatalf("third page jobs = %d, want 0", len(third.Jobs))
	}
	if third.Pagination == nil || third.Pagination.HasMore {
		t.Fatalf("third page pagination = %+v, want exhausted", third.Pagination)
	}

	want := []string{"0", "3", "5"}
	if len(starts) != len(want) {
		t.Fatalf("requests = %v, want %v", starts, want)
	}
	for i := range want {
		if starts[i] != want[i] {
			t.Fatalf("request starts = %v, want %v", starts, want)
		}
	}
}

func TestSearchHonorsLimitWithoutShiftingContinuation(t *testing.T) {
	partition, err := linkedinguest.NewSource(serveFixture(t, "search_page_1.html")).Search(context.Background(), &domain.SearchRequest{Keywords: "go", Limit: 2})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(partition.Jobs) != 2 {
		t.Fatalf("jobs = %d, want 2", len(partition.Jobs))
	}
	if partition.Pagination.NextCursor != "3" {
		t.Fatalf("next cursor = %q, want 3 (provider cards, not returned jobs)", partition.Pagination.NextCursor)
	}
	if partition.Pagination.Native.Count != 3 {
		t.Fatalf("native count = %d, want 3", partition.Pagination.Native.Count)
	}
}

func TestSearchEmptyBodyIsExhausted(t *testing.T) {
	client := testutil.NewClient(func(*http.Request) (*http.Response, error) {
		return testutil.HTMLResponse(http.StatusOK, "  \n"), nil
	})
	partition, err := linkedinguest.NewSource(client).Search(context.Background(), &domain.SearchRequest{Keywords: "go", Offset: 1000})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if partition.Jobs == nil || len(partition.Jobs) != 0 {
		t.Fatalf("jobs = %#v, want empty", partition.Jobs)
	}
	if partition.Pagination == nil || partition.Pagination.HasMore {
		t.Fatalf("pagination = %+v, want exhausted", partition.Pagination)
	}
}

func TestSearchSchemaChangedOnUnrecognizedHTML(t *testing.T) {
	_, err := linkedinguest.NewSource(serveFixture(t, "search_schema_changed.html")).Search(context.Background(), &domain.SearchRequest{Keywords: "go"})
	jobErr := requireErrorCode(t, err, joberrors.APISchemaChanged)
	if jobErr.ExitCode() != 6 {
		t.Errorf("exit code = %d, want 6", jobErr.ExitCode())
	}
}

func TestSearchSchemaChangedOnCardMissingTitle(t *testing.T) {
	_, err := linkedinguest.NewSource(serveFixture(t, "search_card_missing_title.html")).Search(context.Background(), &domain.SearchRequest{Keywords: "go"})
	_ = requireErrorCode(t, err, joberrors.APISchemaChanged)
}

func TestSearchHTTPFailures(t *testing.T) {
	cases := []struct {
		status    int
		code      joberrors.Code
		retryable bool
		exit      int
	}{
		{http.StatusTooManyRequests, joberrors.RateLimited, true, 5},
		{http.StatusForbidden, joberrors.APIAccessForbidden, false, 6},
		{http.StatusInternalServerError, joberrors.APIError, true, 6},
		{http.StatusNotFound, joberrors.APIError, false, 6},
	}
	for _, tc := range cases {
		client := testutil.NewClient(func(*http.Request) (*http.Response, error) {
			return testutil.HTMLResponse(tc.status, "<html></html>"), nil
		})
		_, err := linkedinguest.NewSource(client).Search(context.Background(), &domain.SearchRequest{Keywords: "go"})
		jobErr := requireErrorCode(t, err, tc.code)
		if jobErr.Retryable != tc.retryable {
			t.Errorf("status %d retryable = %v, want %v", tc.status, jobErr.Retryable, tc.retryable)
		}
		if jobErr.ExitCode() != tc.exit {
			t.Errorf("status %d exit = %d, want %d", tc.status, jobErr.ExitCode(), tc.exit)
		}
	}
}

func TestSearchValidatesContinuationInput(t *testing.T) {
	source := linkedinguest.NewSource(serveFixture(t, "search_page_1.html"))
	_, err := source.Search(context.Background(), &domain.SearchRequest{Keywords: "go", Cursor: "not-a-number"})
	_ = requireErrorCode(t, err, joberrors.ValidationFailed)
	_, err = source.Search(context.Background(), &domain.SearchRequest{Keywords: "go", Offset: -1})
	_ = requireErrorCode(t, err, joberrors.ValidationFailed)
}

func TestDetailIsNotAvailableOnGuest(t *testing.T) {
	_, err := linkedinguest.NewSource(serveFixture(t, "search_page_1.html")).Detail(context.Background(), &domain.DetailRequest{SourceJobID: "3891234567"})
	_ = requireErrorCode(t, err, joberrors.SourceUnavailable)
}
