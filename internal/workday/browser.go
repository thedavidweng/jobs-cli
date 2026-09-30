package workday

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/cdp"
	"github.com/go-rod/rod/lib/proto"
	"github.com/thedavidweng/jobs-cli/v2/internal/artifact"
	"github.com/thedavidweng/jobs-cli/v2/internal/domain"
	"github.com/thedavidweng/jobs-cli/v2/internal/errors"
)

//go:embed inspect.js
var inspectScript string

//go:embed fill.js
var fillScript string

//go:embed bindings.json
var bindingsJSON string

type BrowserProvider struct{ Endpoint, Tab, StatePath string }

func NewBrowserProvider(endpoint, tab, state string) domain.ApplyProvider {
	return &BrowserProvider{endpoint, tab, state}
}

func (*BrowserProvider) Capabilities() domain.Capabilities {
	return domain.CapabilitiesFor(domain.ProviderWorkday)
}

type pageSnapshot struct {
	URL           string                       `json:"url"`
	Step          string                       `json:"step"`
	Pending       string                       `json:"pending"`
	Fields        []domain.ApplicationField    `json:"fields"`
	Questions     []domain.ApplicationQuestion `json:"questions"`
	Sections      []domain.ApplicationSection  `json:"sections"`
	Controls      map[string]string            `json:"controls"`
	Values        map[string]string            `json:"values"`
	Review        string                       `json:"review"`
	Receipt       string                       `json:"receipt"`
	ApplicationID string                       `json:"application_id"`
	Tasks         []string                     `json:"tasks"`
}
type workflowState struct {
	Target              domain.ApplicationTarget `json:"target"`
	InputHash           string                   `json:"input_hash"`
	AnswerHashes        map[string]string        `json:"answer_hashes"`
	Uploads             map[string]string        `json:"uploads"`
	Steps               map[string]string        `json:"steps"`
	ReviewHash          string                   `json:"review_hash,omitempty"`
	StartAttempted      bool                     `json:"start_attempted,omitempty"`
	SubmissionAttempted bool                     `json:"submission_attempted,omitempty"`
}

func (p *BrowserProvider) connect(parent context.Context, t *domain.ApplicationTarget) (*rod.Page, func(), error) {
	endpoint, err := url.Parse(p.Endpoint)
	if err != nil || (endpoint.Scheme != "http" && endpoint.Scheme != "ws") || endpoint.User != nil {
		return nil, nil, validation("--browser-endpoint must be a local http or ws CDP endpoint")
	}
	ip := net.ParseIP(endpoint.Hostname())
	if endpoint.Hostname() != "localhost" && (ip == nil || !ip.IsLoopback()) {
		return nil, nil, validation("CDP endpoint must use a loopback host")
	}
	if p.Tab == "" {
		return nil, nil, validation("--browser-tab must identify an existing Job tab")
	}
	wsURL := p.Endpoint
	if endpoint.Scheme == "http" {
		request, err := http.NewRequestWithContext(parent, http.MethodGet, strings.TrimRight(p.Endpoint, "/")+"/json/version", http.NoBody)
		if err != nil {
			return nil, nil, err
		}
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			return nil, nil, err
		}
		var version struct {
			URL string `json:"webSocketDebuggerUrl"`
		}
		decodeErr := json.NewDecoder(response.Body).Decode(&version)
		_ = response.Body.Close()
		if decodeErr != nil {
			return nil, nil, decodeErr
		}
		wsURL = version.URL
	}
	wsEndpoint, err := url.Parse(wsURL)
	if err != nil || wsEndpoint.Scheme != "ws" || wsEndpoint.Host != endpoint.Host {
		return nil, nil, validation("browser returned a different CDP origin")
	}
	socket := &cdp.WebSocket{}
	if err := socket.Connect(parent, wsURL, nil); err != nil {
		return nil, nil, err
	}
	stop := context.AfterFunc(parent, func() { _ = socket.Close() })
	closeConnection := func() { stop(); _ = socket.Close() }
	connection := rod.New().Context(parent).Client(cdp.New().Start(socket))
	if err := connection.Connect(); err != nil {
		closeConnection()
		return nil, nil, err
	}
	ctx, err := connection.PageFromTarget(proto.TargetTargetID(p.Tab))
	if err != nil {
		closeConnection()
		return nil, nil, err
	}
	snapshot, err := readPage(ctx)
	if err != nil {
		closeConnection()
		return nil, nil, err
	}
	expected, err := url.Parse(t.URL)
	if err != nil {
		closeConnection()
		return nil, nil, validation("invalid Application Target URL")
	}
	current, err := url.Parse(snapshot.URL)
	if err != nil || current.Scheme != expected.Scheme || current.Host != expected.Host || current.Path != expected.Path && !strings.HasPrefix(current.Path, strings.TrimRight(expected.Path, "/")+"/") {
		closeConnection()
		return nil, nil, validation("browser tab does not match this employer/site/Job; open the intended Job in that tab")
	}
	return ctx, closeConnection, nil
}

func readPage(ctx *rod.Page) (pageSnapshot, error) {
	var page pageSnapshot
	err := evaluate(ctx, inspectScript, &page, json.RawMessage(bindingsJSON))
	return page, err
}

func inspection(t *domain.ApplicationTarget, page *pageSnapshot) *domain.ApplicationInspection {
	result := &domain.ApplicationInspection{Provider: domain.ProviderWorkday, Application: *t, Step: page.Step, PendingAction: page.Pending, Fields: page.Fields, Questions: page.Questions, Sections: page.Sections, Capabilities: domain.CapabilitiesFor(domain.ProviderWorkday)}
	for _, field := range page.Fields {
		if field.Name == "resume" {
			result.AcceptsResume = true
		}
		if field.Name == "cover_letter" {
			result.AcceptsCoverLetter = true
		}
	}
	// Existing values do not change requirements; section field definitions do.
	requirements := struct {
		Step        string
		Fingerprint string
		Sections    []domain.ApplicationSection
	}{page.Step, domain.Fingerprint(t, page.Fields, page.Questions), nil}
	for _, section := range page.Sections {
		requirements.Sections = append(requirements.Sections, domain.ApplicationSection{Name: section.Name, Fields: section.Fields})
	}
	result.Fingerprint = hash(requirements)
	return result
}

func (p *BrowserProvider) Inspect(ctx context.Context, req *domain.InspectRequest) (*domain.ApplicationInspection, error) {
	browser, closeConnection, err := p.connect(ctx, &req.Target)
	if err != nil {
		return nil, err
	}
	defer closeConnection()
	page, err := readPage(browser)
	if err != nil {
		return nil, err
	}
	return inspection(&req.Target, &page), nil
}

func hash(value any) string {
	data, _ := json.Marshal(value)
	return fmt.Sprintf("sha256:%x", sha256.Sum256(data))
}

func reviewHash(page *pageSnapshot) string {
	return hash(struct {
		Review    string
		Values    map[string]string
		Sections  []domain.ApplicationSection
		Fields    []domain.ApplicationField
		Questions []domain.ApplicationQuestion
	}{page.Review, page.Values, page.Sections, page.Fields, page.Questions})
}

func inputHash(a *domain.ApplicationArtifact) string {
	return hash(struct {
		Candidate   domain.Candidate
		Attachments []domain.Attachment
		Resume      json.RawMessage
		Overrides   map[string]json.RawMessage
	}{a.Candidate, a.Attachments, a.ResumeJSON, a.CandidateOverrides})
}

func validation(message string) error {
	return errors.New(errors.ValidationFailed, message, errors.CatValidation, false, nil)
}

func outcome(status string, page *pageSnapshot) *domain.SubmissionResult {
	return &domain.SubmissionResult{Provider: domain.ProviderWorkday, Status: status, Receipt: map[string]any{"step": page.Step, "pending_action": page.Pending, "tasks": page.Tasks}}
}

func (p *BrowserProvider) save(state *workflowState) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p.StatePath, data, 0o600)
}

func (p *BrowserProvider) load(a *domain.ApplicationArtifact) (workflowState, error) {
	if a.ArtifactVersion != domain.ArtifactVersion || len(a.BrowserSteps) == 0 || a.Provider != domain.ProviderWorkday || a.Application.Provider != domain.ProviderWorkday {
		return workflowState{}, validation("browser execution requires a prepared v2 Workday artifact")
	}
	if a.Application.Tenant == "" || a.Application.Site == "" || a.Application.ProviderJobID == "" {
		return workflowState{}, validation("Workday requires tenant, site and Job identifiers")
	}
	if p.StatePath == "" {
		return workflowState{}, validation("--state is required for browser fill and submit")
	}
	data, err := os.ReadFile(p.StatePath)
	if os.IsNotExist(err) {
		return workflowState{Target: a.Application, InputHash: inputHash(a), AnswerHashes: answerHashes(a), Steps: map[string]string{}, Uploads: map[string]string{}}, nil
	}
	if err != nil {
		return workflowState{}, err
	}
	var state workflowState
	if err := json.Unmarshal(data, &state); err != nil {
		return state, err
	}
	if state.Target.URL != a.Application.URL || state.Target.Tenant != a.Application.Tenant || state.Target.Site != a.Application.Site || state.Target.ProviderJobID != a.Application.ProviderJobID {
		return state, validation("workflow state belongs to a different Application Target")
	}
	if state.InputHash != inputHash(a) {
		return state, validation("reviewed candidate/attachments changed; use a new state file and review again")
	}
	if state.Steps == nil || state.Uploads == nil || state.AnswerHashes == nil {
		return state, validation("invalid browser workflow state")
	}
	currentAnswers := answerHashes(a)
	for id, digest := range state.AnswerHashes {
		if currentAnswers[id] != digest {
			return state, validation("previously reviewed answers changed; review the affected steps again")
		}
	}
	for id := range currentAnswers {
		if _, known := state.AnswerHashes[id]; known {
			continue
		}
		pendingQuestion := false
		for index := range a.BrowserSteps {
			step := &a.BrowserSteps[index]
			for _, question := range step.Questions {
				if question.ID != id {
					continue
				}
				if _, completed := state.Steps[step.Step]; completed {
					return state, validation("answer added to an already completed step; review the affected step again")
				}
				pendingQuestion = true
			}
		}
		if !pendingQuestion {
			return state, validation("new answers must belong to an inspected, uncompleted step")
		}
		state.ReviewHash = ""
	}
	state.AnswerHashes = currentAnswers
	return state, nil
}

func answerHashes(a *domain.ApplicationArtifact) map[string]string {
	result := make(map[string]string, len(a.Answers))
	for _, answer := range a.Answers {
		result[answer.QuestionID] = hash(answer.Value)
	}
	return result
}

func approved(a *domain.ApplicationArtifact, i *domain.ApplicationInspection) bool {
	for index := range a.BrowserSteps {
		step := &a.BrowserSteps[index]
		if step.Step == i.Step && step.Fingerprint == i.Fingerprint {
			return true
		}
	}
	return false
}

func (p *BrowserProvider) Fill(ctx context.Context, req *domain.SubmitRequest) (*domain.SubmissionResult, error) {
	state, err := p.load(&req.Artifact)
	if err != nil {
		return nil, err
	}
	if err := artifact.Validate(&req.Artifact, &domain.ApplicationInspection{AcceptsResume: true, AcceptsCoverLetter: true}); err != nil {
		return nil, err
	}
	if state.SubmissionAttempted {
		return nil, validation("submission was already attempted; inspect the browser receipt, do not retry automatically")
	}
	browser, closeConnection, err := p.connect(ctx, &req.Target)
	if err != nil {
		return nil, err
	}
	defer closeConnection()
	for {
		page, err := readPage(browser)
		if err != nil {
			return nil, err
		}
		if page.Pending == "start_application_required" && req.Artifact.PendingAction == "start_application_required" {
			if state.StartAttempted {
				return outcome("start_uncertain", &page), nil
			}
			state.StartAttempted = true
			if err := p.save(&state); err != nil {
				return nil, err
			}
			if err := clickAndWait(browser, page.Controls["start"], `!document.querySelector('[data-automation-id="adventureButton"]')`); err != nil {
				return outcome("start_uncertain", &page), nil
			}
			continue
		}
		if page.Pending != "" {
			return outcome(page.Pending, &page), nil
		}
		if page.Receipt != "" {
			return outcome("already_submitted", &page), nil
		}
		if page.Controls["submit"] != "" {
			for _, attachment := range req.Artifact.Attachments {
				if state.Uploads[attachment.Kind] != attachment.SHA256 {
					result := outcome("missing_answers", &page)
					result.Receipt["detail"] = "document not uploaded: " + attachment.Kind
					return result, nil
				}
			}
			if len(state.Steps) == 0 {
				return outcome("review_required", &page), nil
			}
			if state.ReviewHash != "" && state.ReviewHash != reviewHash(&page) {
				return nil, validation("review values changed; return to the affected step, prepare and fill again")
			}

			// Only values actually rendered at review can be approved.
			state.ReviewHash = reviewHash(&page)
			if err := p.save(&state); err != nil {
				return nil, err
			}
			return outcome("review_ready", &page), nil
		}
		i := inspection(&req.Target, &page)
		if !approved(&req.Artifact, i) {
			result := outcome("requirements_changed", &page)
			result.Receipt["inspection"] = i
			return result, nil
		}
		validationInspection := *i
		validationInspection.AcceptsResume = true
		validationInspection.AcceptsCoverLetter = true
		if err := artifact.Validate(&req.Artifact, &validationInspection); err != nil {
			return nil, err
		}
		if err := p.save(&state); err != nil {
			return nil, err
		}
		var pending string
		if err := evaluate(browser, fillScript, &pending, req.Artifact, json.RawMessage(bindingsJSON)); err != nil {
			return nil, err
		}
		if strings.HasPrefix(pending, "row_added:") {
			var added struct {
				Selector string `json:"selector"`
				Count    int    `json:"count"`
			}
			if err := json.Unmarshal([]byte(strings.TrimPrefix(pending, "row_added:")), &added); err != nil {
				return nil, err
			}
			if err := browser.Timeout(5 * time.Second).Wait(rod.Eval(`(selector, count) => document.querySelectorAll(selector).length > count`, added.Selector, added.Count)); err != nil {
				return nil, err
			}
			continue
		}
		if pending != "" {
			result := outcome("missing_answers", &page)
			result.Receipt["detail"] = pending
			return result, nil
		}
		// Conditional controls may appear after filling; inspect before saving/advancing.
		updated, err := readPage(browser)
		if err != nil {
			return nil, err
		}
		if updated.Pending != "" {
			return outcome(updated.Pending, &updated), nil
		}
		if !approved(&req.Artifact, inspection(&req.Target, &updated)) {
			result := outcome("requirements_changed", &updated)
			result.Receipt["inspection"] = inspection(&req.Target, &updated)
			return result, nil
		}
		if err := evaluate(browser, fillScript, &pending, req.Artifact, json.RawMessage(bindingsJSON), true); err != nil {
			return nil, err
		}
		if pending != "" {
			result := outcome("missing_answers", &updated)
			result.Receipt["detail"] = pending
			return result, nil
		}
		for _, attachment := range req.Artifact.Attachments {
			selector := updated.Controls["file:"+attachment.Kind]
			if selector == "" {
				// The document may be requested on a later wizard step.
				continue
			}
			fileInfo, err := os.Stat(attachment.Path)
			if err != nil {
				return nil, err
			}
			if fileInfo.IsDir() {
				return nil, validation("attachment is a directory")
			}
			filePath, err := filepath.Abs(attachment.Path)
			if err != nil {
				return nil, err
			}
			digest, err := fileDigest(browser, selector)
			if err != nil {
				return nil, err
			}
			if digest != attachment.SHA256 {
				if err := upload(browser, selector, filePath); err != nil {
					return nil, err
				}
				digest, err = fileDigest(browser, selector)
				if err != nil {
					return nil, err
				}
				if digest != attachment.SHA256 {
					return nil, validation("browser did not retain the reviewed " + attachment.Kind + " document")
				}
			}
			state.Uploads[attachment.Kind] = digest
			if err := p.save(&state); err != nil {
				return nil, err
			}
		}
		if updated.Controls["next"] == "" {
			return outcome("unsupported_step", &updated), nil
		}
		state.Steps[page.Step] = i.Fingerprint
		if err := p.save(&state); err != nil {
			return nil, err
		}
		if err := clickAndWait(browser, updated.Controls["next"], `document.querySelector('[data-automation-id="applyFlowPage"]')?.textContent.trim() !== args[0] || !!document.querySelector('[data-automation-id="applicationConfirmation"]')`, page.Step); err != nil {
			return nil, err
		}
	}
}

func (p *BrowserProvider) Submit(ctx context.Context, req *domain.SubmitRequest) (*domain.SubmissionResult, error) {
	state, err := p.load(&req.Artifact)
	if err != nil {
		return nil, err
	}
	if err := artifact.Validate(&req.Artifact, &domain.ApplicationInspection{AcceptsResume: true, AcceptsCoverLetter: true}); err != nil {
		return nil, err
	}
	browser, closeConnection, err := p.connect(ctx, &req.Target)
	if err != nil {
		return nil, err
	}
	defer closeConnection()
	page, err := readPage(browser)
	if err != nil {
		return nil, err
	}
	if page.Receipt != "" {
		return receipt(&page), nil
	}
	if page.Pending != "" {
		return outcome(page.Pending, &page), nil
	}
	if state.SubmissionAttempted {
		return outcome("submission_uncertain", &page), nil
	}
	if state.ReviewHash == "" || page.Controls["submit"] == "" {
		return outcome("review_required", &page), nil
	}
	if state.ReviewHash != reviewHash(&page) {
		return nil, validation("browser values changed after fill review; fill and review again")
	}
	for index := range req.Artifact.BrowserSteps {
		step := &req.Artifact.BrowserSteps[index]
		if previous, ok := state.Steps[step.Step]; ok && previous != step.Fingerprint {
			return nil, validation("prepared requirements changed since fill")
		}
	}
	state.SubmissionAttempted = true
	if err := p.save(&state); err != nil {
		return nil, err
	}
	// A submission click is never retried, even if the browser connection is lost.
	if err := clickAndWait(browser, page.Controls["submit"], `!!document.querySelector('[data-automation-id="applicationConfirmation"],[data-automation-id="thankYouMessage"]')`); err != nil {
		return outcome("submission_uncertain", &page), nil
	}
	page, err = readPage(browser)
	if err != nil {
		return outcome("submission_uncertain", &page), nil
	}
	if page.Receipt == "" {
		return outcome("submission_uncertain", &page), nil
	}
	return receipt(&page), nil
}

func receipt(page *pageSnapshot) *domain.SubmissionResult {
	result := outcome("submitted", page)
	result.Submitted = true
	result.ApplicationID = page.ApplicationID
	result.ConfirmationURL = page.URL
	result.Receipt["confirmation"] = page.Receipt
	return result
}

func evaluate(page *rod.Page, function string, out any, args ...any) error {
	result, err := page.Eval(function, args...)
	if err != nil {
		return err
	}
	if out == nil {
		return nil
	}
	return result.Value.Unmarshal(out)
}

func clickAndWait(page *rod.Page, selector, expression string, args ...any) error {
	element, err := page.Element(selector)
	if err != nil {
		return err
	}
	if err := element.Click(proto.InputMouseButtonLeft, 1); err != nil {
		return err
	}
	return page.Timeout(5 * time.Second).Wait(rod.Eval("(...args) => ("+expression+")", args...))
}

func upload(page *rod.Page, selector, path string) error {
	element, err := page.Element(selector)
	if err != nil {
		return err
	}
	return element.SetFiles([]string{path})
}

func fileDigest(page *rod.Page, selector string) (string, error) {
	result, err := page.Eval(`async (selector) => {const file=document.querySelector(selector).files[0];if(!file)return '';const digest=await crypto.subtle.digest('SHA-256',await file.arrayBuffer());return Array.from(new Uint8Array(digest),b=>b.toString(16).padStart(2,'0')).join('');}`, selector)
	if err != nil {
		return "", err
	}
	return result.Value.Str(), nil
}
