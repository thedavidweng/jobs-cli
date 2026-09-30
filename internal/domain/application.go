package domain

import "github.com/thedavidweng/jobs-cli/v2/internal/errors"

type ApplicationProvider string

func BrowserRequiredError(provider, operation, url string) *errors.Error {
	message := provider + " native application " + operation + " is not available; the candidate flow is browser-only"
	if url != "" {
		message += ": " + url
	}
	return errors.New(errors.BrowserRequired, message, errors.CatAPI, false, nil)
}

const (
	ProviderGreenhouse      ApplicationProvider = "greenhouse"
	ProviderLinkedIn        ApplicationProvider = "linkedin"
	ProviderIndeed          ApplicationProvider = "indeed"
	ProviderLever           ApplicationProvider = "lever"
	ProviderAshby           ApplicationProvider = "ashby"
	ProviderWorkday         ApplicationProvider = "workday"
	ProviderSmartRecruiters ApplicationProvider = "smartrecruiters"
	ProviderICIMS           ApplicationProvider = "icims"
	ProviderYZi             ApplicationProvider = "yzi"
	ProviderPeopleSoft      ApplicationProvider = "peoplesoft"
	ProviderExternal        ApplicationProvider = "external"
	ProviderUnknown         ApplicationProvider = "unknown"
)

type VerificationPosture string

const (
	VerifiedWorking        VerificationPosture = "VERIFIED WORKING"
	VerifiedSourceImpl     VerificationPosture = "VERIFIED SOURCE IMPLEMENTATION"
	PartiallyVerified      VerificationPosture = "PARTIALLY VERIFIED"
	BrowserRequiredPosture VerificationPosture = "BROWSER REQUIRED"
)

type Capabilities struct {
	Inspect         bool `json:"inspect"`
	Prepare         bool `json:"prepare"`
	BrowserFill     bool `json:"browser_fill"`
	BrowserSubmit   bool `json:"browser_submit"`
	NativeSubmit    bool `json:"native_submit"`
	BrowserRequired bool `json:"browser_required"`
	AuthRequired    bool `json:"auth_required"`
}

type ApplicationTarget struct {
	URL           string              `json:"url"`
	Provider      ApplicationProvider `json:"provider"`
	ProviderJobID string              `json:"provider_job_id,omitempty"`
	BoardToken    string              `json:"board_token,omitempty"`
	Tenant        string              `json:"tenant,omitempty"`
	Site          string              `json:"site,omitempty"`
	Capabilities  Capabilities        `json:"capabilities"`
	Verification  VerificationPosture `json:"verification,omitempty"`
	ResolvedFrom  string              `json:"resolved_from,omitempty"`
}

type ApplicationQuestion struct {
	ID        string           `json:"id"`
	Label     string           `json:"label"`
	Type      string           `json:"type"`
	Required  bool             `json:"required"`
	Options   []QuestionOption `json:"options,omitempty"`
	MaxLength int              `json:"max_length,omitempty"`
}

type QuestionOption struct {
	Value string `json:"value"`
	Label string `json:"label,omitempty"`
}

type ApplicationField struct {
	Options  []QuestionOption `json:"options,omitempty"`
	Name     string           `json:"name"`
	Label    string           `json:"label"`
	Type     string           `json:"type"`
	Required bool             `json:"required"`
}

type ApplicationInspection struct {
	Step               string                `json:"step,omitempty"`
	PendingAction      string                `json:"pending_action,omitempty"`
	Sections           []ApplicationSection  `json:"sections,omitempty"`
	Provider           ApplicationProvider   `json:"provider"`
	Application        ApplicationTarget     `json:"application"`
	Fingerprint        string                `json:"fingerprint"`
	Fields             []ApplicationField    `json:"fields,omitempty"`
	Questions          []ApplicationQuestion `json:"questions,omitempty"`
	AcceptsResume      bool                  `json:"accepts_resume"`
	AcceptsCoverLetter bool                  `json:"accepts_cover_letter"`
	Capabilities       Capabilities          `json:"capabilities"`
}

type SubmissionResult struct {
	Provider        ApplicationProvider `json:"provider"`
	Submitted       bool                `json:"submitted"`
	ApplicationID   string              `json:"application_id,omitempty"`
	Status          string              `json:"status,omitempty"`
	ConfirmationURL string              `json:"confirmation_url,omitempty"`
	Receipt         map[string]any      `json:"receipt,omitempty"`
}

type ApplicationSection struct {
	Name   string                `json:"name"`
	Rows   []map[string]string   `json:"rows,omitempty"`
	Fields []ApplicationQuestion `json:"fields,omitempty"`
}
