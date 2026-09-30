package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

const ArtifactSchemaVersion = "2026-09-30"

const ArtifactVersion = 2

type ApplicationArtifact struct {
	PendingAction         string                     `json:"pending_action,omitempty"`
	RequirementsValidated bool                       `json:"requirements_validated"`
	SchemaVersion         string                     `json:"schema_version"`
	ArtifactVersion       int                        `json:"artifact_version"`
	GeneratedAt           string                     `json:"generated_at"`
	JobID                 string                     `json:"job_id"`
	JobTitle              string                     `json:"job_title,omitempty"`
	Employer              string                     `json:"employer,omitempty"`
	Provider              ApplicationProvider        `json:"provider"`
	Application           ApplicationTarget          `json:"application"`
	Fingerprint           string                     `json:"fingerprint"`
	BrowserSteps          []ApplicationInspection    `json:"browser_steps,omitempty"`
	ResumeJSON            json.RawMessage            `json:"resume_json,omitempty"`
	CandidateOverrides    map[string]json.RawMessage `json:"candidate_overrides,omitempty"`
	Candidate             Candidate                  `json:"candidate"`
	Answers               []ApplicationAnswer        `json:"answers,omitempty"`
	Attachments           []Attachment               `json:"attachments,omitempty"`
}

type Candidate struct {
	FullName     string            `json:"full_name,omitempty"`
	Address      map[string]string `json:"address,omitempty"`
	Work         []map[string]any  `json:"work,omitempty"`
	Education    []map[string]any  `json:"education,omitempty"`
	Skills       []map[string]any  `json:"skills,omitempty"`
	Languages    []map[string]any  `json:"languages,omitempty"`
	Certificates []map[string]any  `json:"certificates,omitempty"`
	FirstName    string            `json:"first_name,omitempty"`
	LastName     string            `json:"last_name,omitempty"`
	Email        string            `json:"email,omitempty"`
	Phone        string            `json:"phone,omitempty"`
	Location     string            `json:"location,omitempty"`
	LinkedIn     string            `json:"linkedin,omitempty"`
	Website      string            `json:"website,omitempty"`
}

type ApplicationAnswer struct {
	QuestionID string `json:"question_id"`
	Value      any    `json:"value,omitempty"`
}

type Attachment struct {
	SHA256      string `json:"sha256,omitempty"`
	Kind        string `json:"kind"`
	Path        string `json:"path"`
	Filename    string `json:"filename,omitempty"`
	ContentType string `json:"content_type,omitempty"`
	Size        int64  `json:"size,omitempty"`
}

type fingerprintInput struct {
	URL       string                `json:"url"`
	Provider  ApplicationProvider   `json:"provider"`
	Fields    []ApplicationField    `json:"fields"`
	Questions []ApplicationQuestion `json:"questions"`
}

func Fingerprint(target *ApplicationTarget, fields []ApplicationField, questions []ApplicationQuestion) string {
	payload := fingerprintInput{
		URL:       target.URL,
		Provider:  target.Provider,
		Fields:    fields,
		Questions: questions,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func (a *ApplicationArtifact) FingerprintMatches(inspection *ApplicationInspection) bool {
	if inspection == nil {
		return false
	}
	return a.Fingerprint != "" && a.Fingerprint == inspection.Fingerprint
}

func (c *Candidate) Value(name string) string {
	switch normalizeFieldName(name) {
	case "full_name", "name":
		return c.FullName
	case "address_line", "postal_code", "country", "region":
		return c.Address[normalizeFieldName(name)]
	case "first_name", "firstname", "given_name":
		return c.FirstName
	case "last_name", "lastname", "family_name", "surname":
		return c.LastName
	case "email", "email_address":
		return c.Email
	case "phone", "phone_number", "mobile":
		return c.Phone
	case "location", "city", "address":
		return c.Location
	case "linkedin", "linkedin_url", "linkedin_profile":
		return c.LinkedIn
	case "website", "url", "portfolio", "github":
		return c.Website
	default:
		return ""
	}
}

func normalizeFieldName(name string) string {
	out := make([]rune, 0, len(name))
	for _, r := range name {
		switch {
		case r >= 'A' && r <= 'Z':
			out = append(out, r+('a'-'A'))
		case r == ' ' || r == '-' || r == '.':
			out = append(out, '_')
		default:
			out = append(out, r)
		}
	}
	return string(out)
}
