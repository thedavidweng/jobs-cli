package voyager

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/thedavidweng/jobs-cli/internal/config"
	"github.com/thedavidweng/jobs-cli/internal/domain"
	joberrors "github.com/thedavidweng/jobs-cli/internal/errors"
)

const (
	graphQLEndpoint = "https://www.linkedin.com/voyager/api/graphql"
	searchQueryName = "JobCardsByJobSearch"
	searchQueryID   = "voyagerJobsDashJobCards.c7c69fb8e8f054fed088918d714be58a"
	detailQueryName = "JobPostingDetailSectionsByCardSectionTypesV2"
	detailQueryID   = "voyagerJobsDashJobPostingDetailSections.8195171dc4c610f8c1551eaef6546bd8"
	applyQueryName  = "JobsOnsiteApplyApplicationByJobPosting"
	applyQueryID    = "voyagerJobsDashOnsiteApplyApplication.34ac512c4fd87baec02c710aef4f563b"
)

type Source struct {
	Client   *http.Client
	Sessions *config.SessionStore
}

func NewSource(client *http.Client, sessions *config.SessionStore) domain.SourceAdapter {
	return &Source{Client: client, Sessions: sessions}
}

func (s *Source) Name() domain.Source { return domain.SourceLinkedIn }

func (s *Source) requireSession() (*config.LinkedInSession, *joberrors.Error) {
	if s.Sessions == nil {
		return nil, sessionRequired("no LinkedIn session store configured")
	}
	session, err := s.Sessions.Load()
	if err != nil {
		if errors.Is(err, config.ErrSessionNotFound) {
			return nil, sessionRequired("LinkedIn session required for authenticated Voyager access; run `jobs-cli auth linkedin login`")
		}
		return nil, joberrors.New(joberrors.InternalError, "read LinkedIn session: "+err.Error(), joberrors.CatInternal, false, err)
	}
	if !session.Complete() {
		return nil, sessionRequired("stored LinkedIn session is incomplete (needs li_at and JSESSIONID); run `jobs-cli auth linkedin login`")
	}
	return session, nil
}

func sessionRequired(message string) *joberrors.Error {
	return joberrors.New(joberrors.LinkedInSessionRequired, message, joberrors.CatAuth, false, nil)
}

func (s *Source) Search(ctx context.Context, req *domain.SearchRequest) (*domain.SearchPartition, error) {
	session, err := s.requireSession()
	if err != nil {
		return nil, err
	}
	limit := req.Limit
	if limit <= 0 {
		limit = 25
	}
	query := map[string]any{
		"keywords":               req.Keywords,
		"origin":                 "JOB_SEARCH_PAGE_SEARCH_BUTTON",
		"spellCorrectionEnabled": true,
	}
	if req.Location != "" {
		geoUrn, lerr := resolveLocation(req.Location)
		if lerr != nil {
			return nil, lerr
		}
		query["locationUnion"] = map[string]any{"geoUrn": geoUrn}
	}
	variables := map[string]any{"query": query, "includeJobState": true, "count": limit, "start": req.Offset}
	if req.Remote != nil && *req.Remote {
		variables["workplaceType"] = []string{"2"}
	}
	if req.Sort != "" {
		variables["sortBy"] = []string{req.Sort}
	}
	raw, callErr := queryGraphQL(ctx, s.httpClient(), session, searchQueryName, searchQueryID, variables)
	if callErr != nil {
		return nil, callErr
	}
	jobs, total, parseErr := parseSearch(raw)
	if parseErr != nil {
		return nil, parseErr
	}
	hasMore := total > req.Offset+len(jobs)
	return &domain.SearchPartition{
		Source: domain.SourceLinkedIn,
		Jobs:   jobs,
		Pagination: &domain.Pagination{
			Limit: limit, Offset: req.Offset, Total: total, HasMore: hasMore,
			Native: &domain.NativePagination{Kind: domain.PaginationOffset, Offset: req.Offset + len(jobs), Count: limit},
		},
	}, nil
}

func (s *Source) Detail(ctx context.Context, req *domain.DetailRequest) (*domain.Job, error) {
	session, err := s.requireSession()
	if err != nil {
		return nil, err
	}
	id := jobID(req.SourceJobID)
	variables := map[string]any{
		"cardSectionTypes": []string{"TOP_CARD_V2", "JOB_DESCRIPTION_CARD"},
		"jobPostingUrn":    "urn:li:fsd_jobPosting:" + id,
		"includeJobState":  true,
	}
	raw, callErr := queryGraphQL(ctx, s.httpClient(), session, detailQueryName, detailQueryID, variables)
	if callErr != nil {
		return nil, callErr
	}
	return parseDetail(raw, id)
}

func (s *Source) httpClient() *http.Client {
	if s.Client != nil {
		return s.Client
	}
	return http.DefaultClient
}

type EasyApply struct {
	Available          bool
	Fields             []domain.ApplicationField
	Questions          []domain.ApplicationQuestion
	AcceptsResume      bool
	AcceptsCoverLetter bool
}

func InspectEasyApply(ctx context.Context, client *http.Client, sessions *config.SessionStore, sourceJobID string) (*EasyApply, error) {
	source := &Source{Client: client, Sessions: sessions}
	session, err := source.requireSession()
	if err != nil {
		return nil, err
	}
	raw, callErr := queryGraphQL(ctx, source.httpClient(), session, applyQueryName, applyQueryID, map[string]any{
		"jobPostingUrn": "urn:li:fsd_jobPosting:" + jobID(sourceJobID),
	})
	if callErr != nil {
		return nil, callErr
	}
	return parseEasyApply(raw)
}

func queryGraphQL(ctx context.Context, client *http.Client, session *config.LinkedInSession, name, id string, variables any) (json.RawMessage, error) {
	encoded, err := EncodeVariables(variables)
	if err != nil {
		return nil, joberrors.New(joberrors.InternalError, "encode LinkedIn Voyager variables: "+err.Error(), joberrors.CatInternal, false, err)
	}
	u, _ := url.Parse(graphQLEndpoint)
	u.RawQuery = "queryId=" + url.QueryEscape(id) + "&queryName=" + url.QueryEscape(name) + "&variables=" + encoded
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), http.NoBody)
	if err != nil {
		return nil, joberrors.New(joberrors.InternalError, "build LinkedIn Voyager request", joberrors.CatInternal, false, err)
	}
	addHeaders(req, session)
	resp, err := client.Do(req)
	if err != nil {
		return nil, joberrors.New(joberrors.NetworkUnreachable, "LinkedIn Voyager request failed", joberrors.CatNetwork, true, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, sessionRequired("LinkedIn session was rejected; run `jobs-cli auth linkedin login`")
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, joberrors.New(joberrors.RateLimited, "LinkedIn Voyager rate limited the request", joberrors.CatNetwork, true, nil)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, joberrors.New(joberrors.APIError, fmt.Sprintf("LinkedIn Voyager returned HTTP %d", resp.StatusCode), joberrors.CatAPI, true, nil)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, joberrors.New(joberrors.NetworkUnreachable, "read LinkedIn Voyager response", joberrors.CatNetwork, true, err)
	}
	return body, nil
}

func addHeaders(req *http.Request, session *config.LinkedInSession) {
	req.Header.Set("Cookie", "li_at="+session.Cookie(config.CookieLiAt)+"; JSESSIONID="+session.Cookie(config.CookieJSessionID)+"; liap=true; lang=v=2&lang=en_US")
	req.Header.Set("csrf-token", session.CSRFTokenValue())
	req.Header.Set("x-restli-protocol-version", "2.0.0")
	req.Header.Set("x-li-lang", "en_US")
	req.Header.Set("x-li-track", `{"osName":"Android OS","osVersion":"36","clientVersion":"4.1.1209","model":"samsung_SM-S901E","displayDensity":2.625,"displayWidth":1080,"displayHeight":2340,"interfaceLocale":"en_US"}`)
	req.Header.Set("User-Agent", "com.linkedin.android/211700 (Linux; U; Android 16; en_US; SM-S901E)")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
}

func EncodeVariables(v any) (string, error) { return encode(reflect.ValueOf(v)) }

func encode(v reflect.Value) (string, error) {
	for v.IsValid() && (v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface) {
		if v.IsNil() {
			return "null", nil
		}
		v = v.Elem()
	}
	if !v.IsValid() {
		return "null", nil
	}
	switch v.Kind() {
	case reflect.String:
		if v.String() == "" {
			return "''", nil
		}
		return strings.ReplaceAll(url.QueryEscape(v.String()), "+", "%20"), nil
	case reflect.Bool:
		return strconv.FormatBool(v.Bool()), nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.FormatInt(v.Int(), 10), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return strconv.FormatUint(v.Uint(), 10), nil
	case reflect.Float32, reflect.Float64:
		return strconv.FormatFloat(v.Float(), 'g', -1, 64), nil
	case reflect.Slice, reflect.Array:
		parts := make([]string, v.Len())
		for i := range parts {
			var err error
			parts[i], err = encode(v.Index(i))
			if err != nil {
				return "", err
			}
		}
		return "List(" + strings.Join(parts, ",") + ")", nil
	case reflect.Map:
		keys := v.MapKeys()
		sort.Slice(keys, func(i, j int) bool { return fmt.Sprint(keys[i]) < fmt.Sprint(keys[j]) })
		parts := make([]string, 0, len(keys))
		for _, key := range keys {
			value, err := encode(v.MapIndex(key))
			if err != nil {
				return "", err
			}
			parts = append(parts, fmt.Sprint(key.Interface())+":"+value)
		}
		return "(" + strings.Join(parts, ",") + ")", nil
	default:
		return "", fmt.Errorf("unsupported Rest.li type %s", v.Type())
	}
}

func parseSearch(raw []byte) ([]domain.Job, int, error) {
	var response struct {
		Data map[string]json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(raw, &response); err != nil {
		return nil, 0, joberrors.New(joberrors.APISchemaChanged, "invalid LinkedIn Voyager search response", joberrors.CatAPI, false, err)
	}
	var collection struct {
		Elements []json.RawMessage `json:"elements"`
		Paging   struct {
			Total int `json:"total"`
		} `json:"paging"`
	}
	found := false
	for _, value := range response.Data {
		if json.Unmarshal(value, &collection) == nil && (collection.Elements != nil || collection.Paging.Total > 0) {
			found = true
			break
		}
	}
	if !found {
		return nil, 0, joberrors.New(joberrors.APISchemaChanged, "LinkedIn Voyager search response had no job collection", joberrors.CatAPI, false, nil)
	}
	jobs := make([]domain.Job, 0, len(collection.Elements))
	for _, element := range collection.Elements {
		job, ok := parseCard(element)
		if !ok {
			return nil, 0, joberrors.New(joberrors.APISchemaChanged, "LinkedIn Voyager job card could not be parsed", joberrors.CatAPI, false, nil)
		}
		jobs = append(jobs, job)
	}
	return jobs, collection.Paging.Total, nil
}

func parseCard(raw json.RawMessage) (domain.Job, bool) {
	var wrapper map[string]json.RawMessage
	if json.Unmarshal(raw, &wrapper) != nil {
		return domain.Job{}, false
	}
	for _, key := range []string{"jobCard", "jobPostingCard"} {
		if nested := wrapper[key]; nested != nil {
			return parseCard(nested)
		}
	}
	var card struct {
		JobPostingTitle    string `json:"jobPostingTitle"`
		Title              string `json:"title"`
		EntityURN          string `json:"entityUrn"`
		FormattedLocation  string `json:"formattedLocation"`
		PrimaryDescription struct {
			Text string `json:"text"`
		} `json:"primaryDescription"`
		SecondaryDescription struct {
			Text string `json:"text"`
		} `json:"secondaryDescription"`
		JobPosting struct {
			EntityURN string `json:"entityUrn"`
			Title     string `json:"title"`
		} `json:"jobPosting"`
	}
	if json.Unmarshal(raw, &card) != nil {
		return domain.Job{}, false
	}
	if card.EntityURN == "" && card.JobPosting.EntityURN == "" && wrapper["jobPosting"] != nil {
		return parseCard(wrapper["jobPosting"])
	}
	id := jobID(first(card.JobPosting.EntityURN, card.EntityURN))
	if id == "" {
		return domain.Job{}, false
	}
	job := domain.NewJob(domain.SourceLinkedIn, id)
	job.Title = first(card.JobPostingTitle, card.Title, card.JobPosting.Title)
	job.Employer = card.PrimaryDescription.Text
	job.Location = first(card.SecondaryDescription.Text, card.FormattedLocation)
	job.Remote = strings.Contains(strings.ToLower(job.Location), "remote")
	if job.Remote {
		job.Workplace = domain.WorkplaceRemote
	}
	job.SourceURL = "https://www.linkedin.com/jobs/view/" + id
	job.ApplicationURL = job.SourceURL
	return job, true
}

func parseDetail(raw []byte, id string) (*domain.Job, error) {
	var root map[string]any
	if err := json.Unmarshal(raw, &root); err != nil {
		return nil, joberrors.New(joberrors.APISchemaChanged, "invalid LinkedIn Voyager detail response", joberrors.CatAPI, false, err)
	}
	job := domain.NewJob(domain.SourceLinkedIn, id)
	job.SourceURL, job.ApplicationURL = "https://www.linkedin.com/jobs/view/"+id, "https://www.linkedin.com/jobs/view/"+id
	sawCard, sawDescription := false, false
	walkJSON(root, func(m map[string]any) {
		if top, ok := m["topCardV2"].(map[string]any); ok {
			sawCard = true
			fillTopCard(&job, top)
		}
		if description, ok := m["jobDescription"].(map[string]any); ok {
			sawDescription = true
			fillDescription(&job, description)
		}
	})
	if !sawCard && !sawDescription {
		return nil, joberrors.New(joberrors.APISchemaChanged, "LinkedIn Voyager detail response had no recognized sections", joberrors.CatAPI, false, nil)
	}
	if job.Title == "" {
		return nil, joberrors.New(joberrors.ResourceNotFound, "LinkedIn job detail was not found", joberrors.CatAPI, false, nil)
	}
	return &job, nil
}

func fillTopCard(job *domain.Job, top map[string]any) {
	card, _ := top["jobPostingCard"].(map[string]any)
	if card == nil {
		card = top
	}
	job.Title = first(job.Title, stringAt(card, "jobPostingTitle"), stringAtPath(card, "jobPosting", "title"))
	job.Employer = first(job.Employer, stringAtPath(card, "primaryDescription", "text"))
	tertiary := stringAtPath(card, "tertiaryDescription", "text")
	if job.Location == "" && tertiary != "" {
		job.Location = strings.TrimSpace(strings.Split(tertiary, " · ")[0])
	}
}

func fillDescription(job *domain.Job, value map[string]any) {
	job.Description = first(job.Description, stringAtPath(value, "jobPosting", "description", "text"), stringAtPath(value, "description", "text"))
}

func walkJSON(value any, visit func(map[string]any)) {
	switch x := value.(type) {
	case map[string]any:
		visit(x)
		for _, child := range x {
			walkJSON(child, visit)
		}
	case []any:
		for _, child := range x {
			walkJSON(child, visit)
		}
	}
}

func stringAt(m map[string]any, key string) string {
	value, _ := m[key].(string)
	return strings.TrimSpace(value)
}

func stringAtPath(m map[string]any, keys ...string) string {
	var value any = m
	for _, key := range keys {
		next, ok := value.(map[string]any)
		if !ok {
			return ""
		}
		value = next[key]
	}
	text, _ := value.(string)
	return strings.TrimSpace(text)
}

func first(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func jobID(value string) string {
	if index := strings.LastIndex(value, ":"); index >= 0 {
		value = value[index+1:]
	}
	return strings.TrimSpace(value)
}

func parseEasyApply(raw []byte) (*EasyApply, error) {
	result := &EasyApply{}
	var root any
	if err := json.Unmarshal(raw, &root); err != nil {
		return nil, joberrors.New(joberrors.APISchemaChanged, "invalid LinkedIn Easy Apply response", joberrors.CatAPI, false, err)
	}
	recognized := false
	walkJSON(root, func(m map[string]any) {
		if available, ok := m["onsiteApply"].(bool); ok {
			recognized = true
			if available {
				result.Available = true
			}
		}
		if stringAtPath(m, "applyCtaText", "text") != "" {
			recognized = true
			result.Available = true
		}
		if stringAtPath(m, "resume", "name") != "" {
			recognized = true
			result.AcceptsResume = true
		}
	})
	if !recognized {
		return nil, joberrors.New(joberrors.APISchemaChanged, "LinkedIn Easy Apply response had no recognized application shape", joberrors.CatAPI, false, nil)
	}
	return result, nil
}
