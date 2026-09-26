package domain

type ApplicationProvider string

const (
	ProviderGreenhouse      ApplicationProvider = "greenhouse"
	ProviderLinkedIn        ApplicationProvider = "linkedin"
	ProviderIndeed          ApplicationProvider = "indeed"
	ProviderLever           ApplicationProvider = "lever"
	ProviderAshby           ApplicationProvider = "ashby"
	ProviderWorkday         ApplicationProvider = "workday"
	ProviderSmartRecruiters ApplicationProvider = "smartrecruiters"
	ProviderICIMS           ApplicationProvider = "icims"
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
	Name     string `json:"name"`
	Label    string `json:"label"`
	Type     string `json:"type"`
	Required bool   `json:"required"`
}

type ApplicationInspection struct {
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
