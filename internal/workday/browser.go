package workday

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
	"github.com/thedavidweng/jobs-cli/v2/internal/artifact"
	"github.com/thedavidweng/jobs-cli/v2/internal/domain"
	"github.com/thedavidweng/jobs-cli/v2/internal/errors"
)

//go:embed inspect.js
var inspectScript string

//go:embed fill.js
var fillScript string

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
	Steps               map[string]string        `json:"steps"`
	ReviewHash          string                   `json:"review_hash,omitempty"`
	StartAttempted      bool                     `json:"start_attempted,omitempty"`
	SubmissionAttempted bool                     `json:"submission_attempted,omitempty"`
}

func (p *BrowserProvider) connect(parent context.Context, t *domain.ApplicationTarget) (context.Context, func(), error) {
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
	allocator, cancelAllocator := chromedp.NewRemoteAllocator(context.WithoutCancel(parent), p.Endpoint)
	owner, cancel := chromedp.NewContext(allocator, chromedp.WithTargetID(target.ID(p.Tab)))
	ctx, cancelRun := context.WithCancel(owner)
	stop := context.AfterFunc(parent, cancelRun)
	closeConnection := func() {
		stop()
		cancelRun()
		// This is a borrowed tab: detach the CDP session without closing the user's tab.
		if c := chromedp.FromContext(owner); c.Target != nil {
			c.Target.TargetID = ""
		}
		cancel()
		cancelAllocator()
	}
	if err := chromedp.Run(ctx); err != nil {
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

func readPage(ctx context.Context) (pageSnapshot, error) {
	var page pageSnapshot
	err := chromedp.Run(ctx, chromedp.Evaluate(inspectScript, &page))
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
		Review   string
		Values   map[string]string
		Sections []domain.ApplicationSection
	}{page.Review, page.Values, page.Sections})
}

func inputHash(a *domain.ApplicationArtifact) string {
	return hash(struct {
		Candidate   domain.Candidate
		Answers     []domain.ApplicationAnswer
		Attachments []domain.Attachment
		Resume      json.RawMessage
	}{a.Candidate, a.Answers, a.Attachments, a.ResumeJSON})
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
	if p.StatePath == "" {
		return workflowState{}, validation("--state is required for browser fill and submit")
	}
	data, err := os.ReadFile(p.StatePath)
	if os.IsNotExist(err) {
		return workflowState{Target: a.Application, InputHash: inputHash(a), Steps: map[string]string{}}, nil
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
		return state, validation("reviewed candidate/answers/attachments changed; use a new state file and review again")
	}
	if state.Steps == nil {
		return state, validation("invalid browser workflow state")
	}
	return state, nil
}

func approved(a *domain.ApplicationArtifact, i *domain.ApplicationInspection) bool {
	if len(a.BrowserSteps) == 0 {
		return a.Fingerprint == i.Fingerprint
	}
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
			if err := chromedp.Run(browser, chromedp.Click(page.Controls["start"], chromedp.ByQuery), chromedp.Poll(`!document.querySelector('[data-automation-id="adventureButton"]')`, nil)); err != nil {
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
		if err := artifact.Validate(&req.Artifact, i); err != nil {
			return nil, err
		}
		payload, err := json.Marshal(req.Artifact)
		if err != nil {
			return nil, err
		}
		var pending string
		if err := chromedp.Run(browser, chromedp.Evaluate("("+fillScript+")("+string(payload)+")", &pending)); err != nil {
			return nil, err
		}
		if strings.HasPrefix(pending, "row_added:") {
			parts := strings.Split(pending, ":")
			count, err := strconv.Atoi(parts[2])
			if err != nil {
				return nil, err
			}
			section := map[string]string{"work": "workExperienceSection", "education": "educationSection", "skills": "skillsSection", "languages": "languagesSection", "certificates": "certificationsSection"}[parts[1]]
			row := map[string]string{"work": "workExperience", "education": "education", "skills": "skill", "languages": "language", "certificates": "certification"}[parts[1]]
			expression := `document.querySelectorAll('[data-automation-id="` + section + `"] [data-automation-id="` + row + `"]').length > ` + strconv.Itoa(count)
			if err := chromedp.Run(browser, chromedp.Poll(expression, nil, chromedp.WithPollingTimeout(5e9))); err != nil {
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
		for _, attachment := range req.Artifact.Attachments {
			if updated.Values["file:"+attachment.Kind] == attachment.Filename {
				continue
			}
			selector := updated.Controls["file:"+attachment.Kind]
			if selector == "" {
				return nil, validation("no upload control for " + attachment.Kind)
			}
			absolute, err := os.Stat(attachment.Path)
			if err != nil {
				return nil, err
			}
			if absolute.IsDir() {
				return nil, validation("attachment is a directory")
			}
			filePath, err := filepath.Abs(attachment.Path)
			if err != nil {
				return nil, err
			}
			if err := chromedp.Run(browser, chromedp.SetUploadFiles(selector, []string{filePath}, chromedp.ByQuery)); err != nil {
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
		if err := chromedp.Run(browser, chromedp.Click(updated.Controls["next"], chromedp.ByQuery), chromedp.Poll(`document.querySelector('[data-automation-id="applyFlowPage"]')?.textContent.trim() !== `+quote(page.Step)+` || !!document.querySelector('[data-automation-id="applicationConfirmation"]')`, nil)); err != nil {
			return nil, err
		}
	}
}
func quote(value string) string { data, _ := json.Marshal(value); return string(data) }
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
	if err := chromedp.Run(browser, chromedp.Click(page.Controls["submit"], chromedp.ByQuery), chromedp.Poll(`!!document.querySelector('[data-automation-id="applicationConfirmation"]')`, nil, chromedp.WithPollingTimeout(5e9))); err != nil {
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
