package greenhouse

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/thedavidweng/jobs-cli/v2/internal/artifact"
	"github.com/thedavidweng/jobs-cli/v2/internal/domain"
	"github.com/thedavidweng/jobs-cli/v2/internal/errors"
	"github.com/thedavidweng/jobs-cli/v2/internal/httpclient"
)

const (
	baseURL      = "https://boards-api.greenhouse.io"
	userAgent    = "jobs-cli"
	fileField    = "file"
	errorSnippet = 300
)

type Provider struct {
	Client *http.Client
	Board  string
	APIKey string
}

func NewProvider(client *http.Client) domain.ApplyProvider {
	return &Provider{Client: client}
}

func NewAuthorizedProvider(client *http.Client, board, key string) domain.ApplyProvider {
	return &Provider{Client: client, Board: board, APIKey: key}
}

type BoardJob struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Location  string `json:"location,omitempty"`
	UpdatedAt string `json:"updated_at,omitempty"`
	URL       string `json:"url,omitempty"`
}

func (p *Provider) Capabilities() domain.Capabilities {
	return domain.Capabilities{Inspect: true, Prepare: true, NativeSubmit: p.APIKey != "" && p.Board != "", AuthRequired: true}
}

func (p *Provider) ListBoardJobs(ctx context.Context, token string) ([]BoardJob, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, errors.New(errors.ATSResolutionFailed, "greenhouse board token is required", errors.CatAPI, false, nil)
	}
	var payload boardJobsResponse
	if err := p.getJSON(ctx, baseURL+"/v1/boards/"+url.PathEscape(token)+"/jobs", &payload); err != nil {
		return nil, err
	}
	jobs := make([]BoardJob, 0, len(payload.Jobs))
	for _, item := range payload.Jobs {
		jobs = append(jobs, BoardJob{
			ID:        identifier(item.ID),
			Title:     item.Title,
			Location:  item.Location.Name,
			UpdatedAt: item.UpdatedAt,
			URL:       item.AbsoluteURL,
		})
	}
	return jobs, nil
}

func (p *Provider) Inspect(ctx context.Context, req *domain.InspectRequest) (*domain.ApplicationInspection, error) {
	if req == nil {
		return nil, errors.New(errors.InternalError, "greenhouse inspect requires a request", errors.CatInternal, false, nil)
	}
	token, jobID, err := boardIdentifiers(&req.Target)
	if err != nil {
		return nil, err
	}
	detail, err := p.fetchJob(ctx, token, jobID)
	if err != nil {
		return nil, err
	}
	target := req.Target
	if target.BoardToken == "" {
		target.BoardToken = token
	}
	if target.ProviderJobID == "" {
		target.ProviderJobID = jobID
	}
	if target.URL == "" {
		target.URL = detail.AbsoluteURL
	}
	fields, questions, acceptsResume, acceptsCover := normalizeQuestions(detail.Questions)
	inspection := &domain.ApplicationInspection{
		Provider:           domain.ProviderGreenhouse,
		Application:        target,
		Fields:             fields,
		Questions:          questions,
		AcceptsResume:      acceptsResume,
		AcceptsCoverLetter: acceptsCover,
		Capabilities:       p.Capabilities(),
	}
	inspection.Fingerprint = domain.Fingerprint(&target, fields, questions)
	return inspection, nil
}

func (p *Provider) Submit(ctx context.Context, req *domain.SubmitRequest) (*domain.SubmissionResult, error) {
	if req == nil {
		return nil, errors.New(errors.InternalError, "greenhouse submit requires a request", errors.CatInternal, false, nil)
	}
	token, jobID, err := boardIdentifiers(&req.Target)
	if p.APIKey == "" || p.Board != token {
		return nil, errors.New(errors.AuthRequired, "Greenhouse submission requires an employer Job Board API key scoped to board "+token, errors.CatAuth, false, nil)
	}
	if err != nil {
		return nil, err
	}
	detail, err := p.fetchJob(ctx, token, jobID)
	if err != nil {
		return nil, err
	}
	fields, questions, acceptsResume, acceptsCover := normalizeQuestions(detail.Questions)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writeApplication(writer, &req.Artifact, fields, questions, acceptsResume, acceptsCover); err != nil {
		_ = writer.Close()
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, errors.New(errors.InternalError, "encode greenhouse application: "+err.Error(), errors.CatInternal, false, err)
	}

	endpoint := baseURL + "/v1/boards/" + url.PathEscape(token) + "/jobs/" + url.PathEscape(jobID)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, &body)
	if err != nil {
		return nil, errors.New(errors.InternalError, "build greenhouse submission request: "+err.Error(), errors.CatInternal, false, err)
	}
	httpReq.SetBasicAuth(p.APIKey, "")
	httpReq.Header.Set("Content-Type", writer.FormDataContentType())
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("User-Agent", userAgent)
	httpReq.ContentLength = int64(body.Len())

	resp, err := p.client().Do(httpReq)
	if err != nil {
		return nil, errors.New(errors.NetworkUnreachable,
			"greenhouse submission failed in transit and was not retried; the application may or may not have been received",
			errors.CatNetwork, false, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, responseError(resp, "greenhouse submission")
	}
	return parseReceipt(resp)
}

func (p *Provider) fetchJob(ctx context.Context, token, jobID string) (jobDetail, error) {
	query := url.Values{}
	query.Set("questions", "true")
	endpoint := baseURL + "/v1/boards/" + url.PathEscape(token) + "/jobs/" + url.PathEscape(jobID) + "?" + query.Encode()
	var detail jobDetail
	if err := p.getJSON(ctx, endpoint, &detail); err != nil {
		return jobDetail{}, err
	}
	if identifier(detail.ID) == "" {
		return jobDetail{}, schemaError("greenhouse job payload is missing an id")
	}
	return detail, nil
}

func (p *Provider) getJSON(ctx context.Context, endpoint string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, http.NoBody)
	if err != nil {
		return errors.New(errors.InternalError, "build greenhouse request: "+err.Error(), errors.CatInternal, false, err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", userAgent)
	resp, err := p.client().Do(req)
	if err != nil {
		return errors.New(errors.NetworkUnreachable, "greenhouse read request failed", errors.CatNetwork, true, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return responseError(resp, "greenhouse read")
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return schemaError("greenhouse response is not valid JSON")
	}
	return nil
}

func (p *Provider) client() *http.Client {
	if p.Client != nil {
		return p.Client
	}
	return http.DefaultClient
}

func writeApplication(writer *multipart.Writer, app *domain.ApplicationArtifact, fields []domain.ApplicationField, questions []domain.ApplicationQuestion, acceptsResume, acceptsCover bool) error {
	for _, field := range fields {
		if artifact.IsFileField(&field) {
			continue
		}
		value := strings.TrimSpace(app.Candidate.Value(field.Name))
		if value == "" {
			continue
		}
		if err := writer.WriteField(field.Name, value); err != nil {
			return errors.New(errors.InternalError, "encode greenhouse field "+field.Name+": "+err.Error(), errors.CatInternal, false, err)
		}
	}

	resume, hasResume := attachmentFor(app, "resume")
	if hasResume {
		if !acceptsResume {
			return errors.New(errors.ApplicationIncomplete, "this application does not accept a resume", errors.CatValidation, false, nil)
		}
		if err := writeFilePart(writer, "resume", &resume); err != nil {
			return err
		}
	}
	cover, hasCover := attachmentFor(app, "cover_letter")
	if hasCover {
		if !acceptsCover {
			return errors.New(errors.ApplicationIncomplete, "this application does not accept a cover letter", errors.CatValidation, false, nil)
		}
		if err := writeFilePart(writer, "cover_letter", &cover); err != nil {
			return err
		}
	}

	answers := artifact.AnswerMap(app)
	for i := range questions {
		question := &questions[i]
		value := strings.TrimSpace(answers[question.ID])
		if value == "" {
			continue
		}
		if strings.EqualFold(question.Type, "input_file") {
			file := domain.Attachment{Kind: question.ID, Path: value, Filename: filepath.Base(value)}
			if err := writeFilePart(writer, question.ID, &file); err != nil {
				return err
			}
			continue
		}
		if err := writer.WriteField(question.ID, value); err != nil {
			return errors.New(errors.InternalError, "encode greenhouse question "+question.ID+": "+err.Error(), errors.CatInternal, false, err)
		}
	}
	return nil
}

func writeFilePart(writer *multipart.Writer, name string, attachment *domain.Attachment) error {
	data, err := os.ReadFile(attachment.Path)
	if err != nil {
		return errors.New(errors.ApplicationIncomplete, fmt.Sprintf("%s file %s is not readable", name, attachment.Path), errors.CatValidation, false, err)
	}
	filename := strings.TrimSpace(attachment.Filename)
	if filename == "" {
		filename = filepath.Base(attachment.Path)
	}
	contentType := strings.TrimSpace(attachment.ContentType)
	if contentType == "" {
		contentType = artifact.ContentTypeFor(filename)
	}
	header := textproto.MIMEHeader{}
	header.Set("Content-Disposition", fmt.Sprintf("form-data; name=%q; filename=%q", sanitizePart(name), sanitizePart(filename)))
	header.Set("Content-Type", contentType)
	part, err := writer.CreatePart(header)
	if err != nil {
		return errors.New(errors.InternalError, "encode greenhouse file "+name+": "+err.Error(), errors.CatInternal, false, err)
	}
	if _, err := part.Write(data); err != nil {
		return errors.New(errors.InternalError, "encode greenhouse file "+name+": "+err.Error(), errors.CatInternal, false, err)
	}
	return nil
}

func parseReceipt(resp *http.Response) (*domain.SubmissionResult, error) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.New(errors.APIError, "read greenhouse submission response: "+err.Error(), errors.CatAPI, true, err)
	}
	result := &domain.SubmissionResult{
		Provider:  domain.ProviderGreenhouse,
		Submitted: true,
		Status:    "submitted",
		Receipt:   map[string]any{},
	}
	if len(bytes.TrimSpace(body)) == 0 {
		return result, nil
	}
	var payload submitResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, schemaError("greenhouse submission response is not valid JSON")
	}
	if id := identifier(payload.ID); id != "" {
		result.ApplicationID = id
		result.Receipt["id"] = id
	}
	if payload.Success != nil {
		result.Receipt["success"] = *payload.Success
		if !*payload.Success {
			return nil, errors.New(errors.APIError, "greenhouse did not confirm the submission", errors.CatAPI, false, nil)
		}
	}
	if payload.Status != "" {
		result.Status = payload.Status
		result.Receipt["status"] = payload.Status
	}
	if payload.ConfirmationURL != "" {
		result.ConfirmationURL = payload.ConfirmationURL
	}
	return result, nil
}

func responseError(resp *http.Response, operation string) error {
	message := operation + " returned HTTP " + strconv.Itoa(resp.StatusCode)
	if detail := errorDetail(resp.Body); detail != "" {
		message += ": " + detail
	}
	switch {
	case resp.StatusCode == http.StatusUnauthorized:
		return errors.New(errors.AuthRequired, message, errors.CatAuth, false, nil)
	case resp.StatusCode == http.StatusForbidden:
		return errors.New(errors.APIAccessForbidden, message, errors.CatAPI, false, nil)
	case resp.StatusCode == http.StatusNotFound:
		return errors.New(errors.ResourceNotFound, message, errors.CatAPI, false, nil)
	case resp.StatusCode == http.StatusTooManyRequests:
		return errors.NewWithRetryAfter(errors.RateLimited, message, errors.CatNetwork, true, httpclient.RetryAfter(resp, 0), nil)
	case resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusUnprocessableEntity:
		return errors.New(errors.ApplicationIncomplete, message, errors.CatValidation, false, nil)
	case resp.StatusCode >= 500:
		return errors.New(errors.APIError, message, errors.CatAPI, true, nil)
	default:
		return errors.New(errors.APIError, message, errors.CatAPI, false, nil)
	}
}

func errorDetail(body io.Reader) string {
	data, err := io.ReadAll(io.LimitReader(body, 8<<10))
	if err != nil {
		return ""
	}
	if trimmed := strings.TrimSpace(string(data)); trimmed != "" {
		var payload struct {
			Error   string `json:"error"`
			Message string `json:"message"`
			Errors  []struct {
				Field   string `json:"field"`
				Message string `json:"message"`
			} `json:"errors"`
		}
		if json.Unmarshal(data, &payload) == nil {
			parts := make([]string, 0, len(payload.Errors)+2)
			if payload.Error != "" {
				parts = append(parts, payload.Error)
			}
			if payload.Message != "" {
				parts = append(parts, payload.Message)
			}
			for _, item := range payload.Errors {
				if item.Message != "" {
					parts = append(parts, item.Message)
				}
			}
			if len(parts) > 0 {
				return truncate(collapse(strings.Join(parts, "; ")), errorSnippet)
			}
		}
		return truncate(collapse(trimmed), errorSnippet)
	}
	return ""
}

func boardIdentifiers(target *domain.ApplicationTarget) (token, jobID string, err error) {
	if target == nil {
		return "", "", errors.New(errors.InternalError, "greenhouse requires an application target", errors.CatInternal, false, nil)
	}
	token = strings.TrimSpace(target.BoardToken)
	jobID = strings.TrimSpace(target.ProviderJobID)
	urlToken, urlJobID := parseBoardURL(target.URL)
	if token == "" {
		token = urlToken
	}
	if jobID == "" {
		jobID = urlJobID
	}
	if token == "" || jobID == "" {
		return "", "", errors.New(errors.ATSResolutionFailed, "greenhouse application target is missing the board token or job id", errors.CatAPI, false, nil)
	}
	return token, jobID, nil
}

func parseBoardURL(raw string) (token, jobID string) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", ""
	}
	segments := make([]string, 0, 6)
	for _, segment := range strings.Split(strings.Trim(parsed.Path, "/"), "/") {
		if segment != "" {
			segments = append(segments, segment)
		}
	}
	for i, segment := range segments {
		if segment == "jobs" && i >= 1 && i+1 < len(segments) {
			token = segments[i-1]
			jobID = segments[i+1]
			break
		}
	}
	query := parsed.Query()
	if jobID == "" {
		jobID = query.Get("gh_jid")
	}
	if token == "" {
		token = query.Get("for")
	}
	return token, jobID
}

func normalizeQuestions(groups []questionGroup) ([]domain.ApplicationField, []domain.ApplicationQuestion, bool, bool) {
	fields := make([]domain.ApplicationField, 0, len(groups))
	questions := make([]domain.ApplicationQuestion, 0, len(groups))
	acceptsResume, acceptsCover := false, false
	for _, group := range groups {
		for _, field := range group.Fields {
			name := strings.TrimSpace(field.Name)
			if name == "" {
				continue
			}
			label := strings.TrimSpace(group.Label)
			if label == "" {
				label = name
			}
			switch strings.ToLower(name) {
			case "resume", "resume_text":
				acceptsResume = true
				if strings.EqualFold(name, "resume") {
					fields = append(fields, domain.ApplicationField{Name: name, Label: label, Type: fileField, Required: group.Required})
				}
				continue
			case "cover_letter", "cover_letter_text":
				acceptsCover = true
				if strings.EqualFold(name, "cover_letter") {
					fields = append(fields, domain.ApplicationField{Name: name, Label: label, Type: fileField, Required: group.Required})
				}
				continue
			}
			if standardFields[strings.ToLower(name)] {
				fields = append(fields, domain.ApplicationField{Name: name, Label: label, Type: strings.TrimSpace(field.Type), Required: group.Required})
				continue
			}
			questions = append(questions, domain.ApplicationQuestion{
				ID:       name,
				Label:    label,
				Type:     strings.TrimSpace(field.Type),
				Required: group.Required,
				Options:  normalizeOptions(field.Values),
			})
		}
	}
	return fields, questions, acceptsResume, acceptsCover
}

var standardFields = map[string]bool{
	"first_name":       true,
	"last_name":        true,
	"email":            true,
	"phone":            true,
	"location":         true,
	"website":          true,
	"linkedin":         true,
	"linkedin_profile": true,
}

func normalizeOptions(values []fieldValue) []domain.QuestionOption {
	if len(values) == 0 {
		return nil
	}
	options := make([]domain.QuestionOption, 0, len(values))
	for _, value := range values {
		option := strings.TrimSpace(value.Value)
		label := strings.TrimSpace(value.Label)
		if option == "" {
			option = label
		}
		if option == "" {
			continue
		}
		options = append(options, domain.QuestionOption{Value: option, Label: label})
	}
	return options
}

func attachmentFor(app *domain.ApplicationArtifact, kind string) (domain.Attachment, bool) {
	for _, attachment := range app.Attachments {
		if attachment.Kind == kind {
			return attachment, true
		}
	}
	return domain.Attachment{}, false
}

func sanitizePart(value string) string {
	value = strings.ReplaceAll(value, "\\", "_")
	value = strings.ReplaceAll(value, `"`, "'")
	value = strings.ReplaceAll(value, "\r", "")
	value = strings.ReplaceAll(value, "\n", "")
	return value
}

func identifier(raw json.RawMessage) string {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return ""
	}
	if strings.HasPrefix(trimmed, `"`) {
		var value string
		if json.Unmarshal(raw, &value) == nil {
			return strings.TrimSpace(value)
		}
		return ""
	}
	var number json.Number
	if json.Unmarshal(raw, &number) == nil {
		return number.String()
	}
	return ""
}

func collapse(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

func truncate(value string, limit int) string {
	if limit <= 0 || len(value) <= limit {
		return value
	}
	return value[:limit] + "..."
}

func schemaError(message string) error {
	return errors.New(errors.APISchemaChanged, message, errors.CatAPI, false, nil)
}

type jobDetail struct {
	ID          json.RawMessage `json:"id"`
	AbsoluteURL string          `json:"absolute_url"`
	Questions   []questionGroup `json:"questions"`
}

type locationJSON struct {
	Name string `json:"name"`
}

type questionGroup struct {
	Label    string      `json:"label"`
	Required bool        `json:"required"`
	Fields   []fieldJSON `json:"fields"`
}

type fieldJSON struct {
	Name   string       `json:"name"`
	Type   string       `json:"type"`
	Values []fieldValue `json:"values"`
}

type fieldValue struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

type boardJobsResponse struct {
	Jobs []boardJobJSON `json:"jobs"`
}

type boardJobJSON struct {
	ID          json.RawMessage `json:"id"`
	Title       string          `json:"title"`
	UpdatedAt   string          `json:"updated_at"`
	AbsoluteURL string          `json:"absolute_url"`
	Location    locationJSON    `json:"location"`
}

type submitResponse struct {
	ID              json.RawMessage `json:"id"`
	Success         *bool           `json:"success"`
	Status          string          `json:"status"`
	ConfirmationURL string          `json:"confirmation_url"`
}
