package e2e

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/thedavidweng/jobs-cli/v2/internal/config"
)

const voyagerSearchFixture = `{"data":{"jobsDashJobCardsByJobSearch":{"paging":{"total":1},"elements":[{"jobCard":{"jobPostingCard":{"jobPostingTitle":"Swift Developer","primaryDescription":{"text":"Acme"},"secondaryDescription":{"text":"Vancouver, BC"},"jobPosting":{"entityUrn":"urn:li:fsd_jobPosting:42"}}}}]}}}`

type authenticatedFanoutTransport struct {
	t             *testing.T
	indeedCalls   int
	voyagerCalls  int
	indeedBody    string
	voyagerRawURL string
	voyagerCookie string
}

func (tr *authenticatedFanoutTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	tr.t.Helper()
	switch req.URL.Host {
	case "apis.indeed.com":
		tr.indeedCalls++
		raw, err := io.ReadAll(req.Body)
		if err != nil {
			tr.t.Fatalf("read indeed body: %v", err)
		}
		tr.indeedBody = string(raw)
		return indeedFixture(tr.t, "search.json"), nil
	case "www.linkedin.com":
		tr.voyagerCalls++
		tr.voyagerRawURL = req.URL.String()
		tr.voyagerCookie = req.Header.Get("Cookie")
		return voyagerJSONResponse(tr.t, voyagerSearchFixture), nil
	default:
		tr.t.Fatalf("unexpected host %s", req.URL.Host)
		return nil, nil
	}
}

func voyagerJSONResponse(t *testing.T, body string) *http.Response {
	t.Helper()
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}
}

func writeLinkedInSession(t *testing.T, dir string) {
	t.Helper()
	store := config.NewSessionStore(dir)
	if err := store.Save(&config.LinkedInSession{Cookies: map[string]string{
		config.CookieLiAt: "secret", config.CookieJSessionID: `"ajax:123"`,
	}}); err != nil {
		t.Fatal(err)
	}
}

func TestSearchAuthenticatedDefaultSourcesKeepIndeedAndUseVoyagerForLinkedIn(t *testing.T) {
	dir := t.TempDir()
	writeLinkedInSession(t, dir)
	transport := &authenticatedFanoutTransport{t: t}
	doc := runIndeedSearchDir(t, dir, transport,
		"--json", "--full", "search", "--query", "swift", "--location", "Vancouver, BC", "--authenticated")

	if transport.indeedCalls != 1 || transport.voyagerCalls != 1 {
		t.Fatalf("outgoing calls = indeed:%d voyager:%d, want one request per default source", transport.indeedCalls, transport.voyagerCalls)
	}
	if !strings.Contains(indeedQuery(t, transport.indeedBody), `what: "swift"`) {
		t.Errorf("indeed request did not carry the keyword:\n%s", transport.indeedBody)
	}
	if !strings.Contains(transport.voyagerRawURL, "queryId=voyagerJobsDashJobCards.c7c69fb8e8f054fed088918d714be58a") {
		t.Errorf("linkedin request did not use the Voyager persisted job search query:\n%s", transport.voyagerRawURL)
	}
	if !strings.Contains(transport.voyagerRawURL, "locationUnion:(geoUrn:urn%3Ali%3Afsd_geo%3A103366113)") {
		t.Errorf("voyager variables did not encode the Vancouver geo URN as locationUnion.geoUrn:\n%s", transport.voyagerRawURL)
	}
	if !strings.Contains(transport.voyagerCookie, "li_at=secret") {
		t.Error("voyager request did not send the stored session cookie")
	}

	if len(doc.Data.Partitions) != 2 {
		t.Fatalf("partitions = %d, want the default indeed + linkedin fan-out", len(doc.Data.Partitions))
	}
	indeed, linkedin := doc.Data.Partitions[0], doc.Data.Partitions[1]
	if indeed.Source != "indeed" || indeed.Error != nil || len(indeed.Jobs) != 3 {
		t.Fatalf("indeed partition = %+v, want a successful indeed search", indeed)
	}
	if linkedin.Source != "linkedin" || linkedin.Error != nil || len(linkedin.Jobs) != 1 {
		t.Fatalf("linkedin partition = %+v, want a successful Voyager search", linkedin)
	}
	if linkedin.Jobs[0].ID != "linkedin:42" || linkedin.Jobs[0].Title != "Swift Developer" {
		t.Fatalf("voyager job = %+v", linkedin.Jobs[0])
	}
	if len(doc.Meta.Partitions) != 2 || !doc.Meta.Partitions[0].OK || !doc.Meta.Partitions[1].OK {
		t.Fatalf("meta partitions = %+v, want both partitions ok", doc.Meta.Partitions)
	}
}

func TestSearchAuthenticatedWithoutSessionKeepsIndeedPartition(t *testing.T) {
	transport := &indeedTransport{
		t:       t,
		respond: func(int) *http.Response { return indeedFixture(t, "search.json") },
	}
	doc := runIndeedSearch(t, transport, "--json", "search", "-q", "swift", "--authenticated", "--country", "US")

	if transport.calls != 1 {
		t.Fatalf("indeed HTTP calls = %d, want the indeed partition to run without a session", transport.calls)
	}
	if len(doc.Data.Partitions) != 2 {
		t.Fatalf("partitions = %d, want the default indeed + linkedin fan-out", len(doc.Data.Partitions))
	}
	indeed, linkedin := doc.Data.Partitions[0], doc.Data.Partitions[1]
	if indeed.Source != "indeed" || indeed.Error != nil || len(indeed.Jobs) != 3 {
		t.Fatalf("indeed partition = %+v, want a successful indeed search", indeed)
	}
	if linkedin.Error == nil || linkedin.Error.Code != "LINKEDIN_SESSION_REQUIRED" {
		t.Fatalf("linkedin partition error = %+v, want LINKEDIN_SESSION_REQUIRED (not SOURCE_UNAVAILABLE)", linkedin.Error)
	}
	if len(doc.Meta.Partitions) != 2 || !doc.Meta.Partitions[0].OK || doc.Meta.Partitions[1].OK {
		t.Fatalf("meta partitions = %+v", doc.Meta.Partitions)
	}
}
