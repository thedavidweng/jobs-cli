package artifact

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/thedavidweng/jobs-cli/internal/domain"
	joberrors "github.com/thedavidweng/jobs-cli/internal/errors"
)

type envelopeProbe struct {
	OK   *bool           `json:"ok"`
	Data json.RawMessage `json:"data"`
}

func Encode(a *domain.ApplicationArtifact) ([]byte, error) {
	return json.MarshalIndent(a, "", "  ")
}

func Decode(data []byte) (domain.ApplicationArtifact, *joberrors.Error) {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return domain.ApplicationArtifact{}, joberrors.New(joberrors.InvalidArguments, "artifact payload is empty", joberrors.CatValidation, false, nil)
	}
	body := data
	probe := envelopeProbe{}
	if err := json.Unmarshal(data, &probe); err != nil {
		return domain.ApplicationArtifact{}, joberrors.New(joberrors.InvalidArguments, "artifact payload is not valid JSON", joberrors.CatValidation, false, err)
	}
	if probe.OK != nil && len(probe.Data) > 0 {
		body = probe.Data
	}
	a := domain.ApplicationArtifact{}
	if err := json.Unmarshal(body, &a); err != nil {
		return domain.ApplicationArtifact{}, joberrors.New(joberrors.InvalidArguments, "artifact payload is not a valid Application Artifact", joberrors.CatValidation, false, err)
	}
	if a.ArtifactVersion == 0 && a.JobID == "" {
		return domain.ApplicationArtifact{}, joberrors.New(joberrors.InvalidArguments, "artifact payload is not an Application Artifact", joberrors.CatValidation, false, nil)
	}
	if a.ArtifactVersion != domain.ArtifactVersion {
		return domain.ApplicationArtifact{}, joberrors.New(joberrors.ValidationFailed,
			fmt.Sprintf("unsupported artifact_version %d (this build supports %d); run `jobs-cli apply prepare` again", a.ArtifactVersion, domain.ArtifactVersion),
			joberrors.CatValidation, false, nil)
	}
	return a, nil
}

func ReadFile(path string) (domain.ApplicationArtifact, *joberrors.Error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return domain.ApplicationArtifact{}, joberrors.New(joberrors.InvalidArguments, fmt.Sprintf("read artifact %s: %v", path, err), joberrors.CatValidation, false, err)
	}
	return Decode(data)
}

func WriteFile(path string, a *domain.ApplicationArtifact) *joberrors.Error {
	data, err := Encode(a)
	if err != nil {
		return joberrors.New(joberrors.InternalError, err.Error(), joberrors.CatInternal, false, err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return joberrors.New(joberrors.InternalError, fmt.Sprintf("write artifact %s: %v", path, err), joberrors.CatInternal, false, err)
	}
	return nil
}

func Stale(a *domain.ApplicationArtifact, inspection *domain.ApplicationInspection) *joberrors.Error {
	if inspection == nil {
		return nil
	}
	if !a.FingerprintMatches(inspection) {
		return joberrors.New(joberrors.ArtifactStale,
			"remote application requirements changed since the artifact was prepared; run `jobs-cli apply prepare` again",
			joberrors.CatValidation, false, nil)
	}
	return nil
}

func AnswerMap(a *domain.ApplicationArtifact) map[string]string {
	out := map[string]string{}
	for _, answer := range a.Answers {
		out[answer.QuestionID] = answerText(answer.Value)
	}
	return out
}

func answerText(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(v)
	case bool:
		if v {
			return "true"
		}
		return "false"
	case []any:
		parts := make([]string, 0, len(v))
		for _, item := range v {
			parts = append(parts, answerText(item))
		}
		return strings.TrimSpace(strings.Join(parts, ","))
	case float64:
		return strings.TrimSpace(fmt.Sprintf("%v", v))
	default:
		data, err := json.Marshal(v)
		if err != nil {
			return ""
		}
		return strings.TrimSpace(string(data))
	}
}

func Warnings(a *domain.ApplicationArtifact, inspection *domain.ApplicationInspection) []string {
	if inspection == nil {
		return nil
	}
	known := map[string]bool{}
	for _, q := range inspection.Questions {
		known[q.ID] = true
	}
	var warnings []string
	for _, answer := range a.Answers {
		if !known[answer.QuestionID] {
			warnings = append(warnings, fmt.Sprintf("answer for unknown question %q is ignored", answer.QuestionID))
		}
	}
	return warnings
}

func Validate(a *domain.ApplicationArtifact, inspection *domain.ApplicationInspection) *joberrors.Error {
	if inspection == nil {
		return nil
	}
	var problems []string
	attachmentKinds := attachmentKinds(a)
	for _, field := range inspection.Fields {
		if IsFileField(field) {
			if field.Required {
				if _, ok := attachmentKinds[strings.ToLower(field.Name)]; !ok {
					problems = append(problems, fieldLabel(field))
				}
			}
			continue
		}
		if field.Required && strings.TrimSpace(a.Candidate.Value(field.Name)) == "" {
			problems = append(problems, fieldLabel(field))
		}
	}
	answers := AnswerMap(a)
	for i := range inspection.Questions {
		question := &inspection.Questions[i]
		value := strings.TrimSpace(answers[question.ID])
		if value == "" {
			if question.Required {
				problems = append(problems, questionLabel(question))
			}
			continue
		}
		if problem := validateAnswer(question, value); problem != "" {
			problems = append(problems, questionLabel(question)+": "+problem)
		}
	}
	problems = append(problems, validateAttachments(a, inspection)...)
	if len(problems) > 0 {
		return joberrors.New(joberrors.ApplicationIncomplete,
			"application is incomplete; fix: "+strings.Join(problems, ", "),
			joberrors.CatValidation, false, nil)
	}
	return nil
}

var supportedQuestionTypes = map[string]bool{
	"input_text":                true,
	"textarea":                  true,
	"long_text":                 true,
	"input_file":                true,
	"multi_value_single_select": true,
	"multi_value_multi_select":  true,
	"boolean":                   true,
	"number":                    true,
	"email":                     true,
	"phone":                     true,
	"url":                       true,
}

func validateAnswer(question *domain.ApplicationQuestion, value string) string {
	questionType := strings.ToLower(strings.TrimSpace(question.Type))
	if questionType == "input_file" {
		if _, err := os.Stat(value); err != nil {
			return "file " + value + " is not readable"
		}
		return ""
	}
	if questionType != "" && !supportedQuestionTypes[questionType] {
		return fmt.Sprintf("unsupported question type %q", question.Type)
	}
	if len(question.Options) == 0 {
		return ""
	}
	allowed := make(map[string]bool, len(question.Options)*2)
	for _, option := range question.Options {
		if value := strings.ToLower(strings.TrimSpace(option.Value)); value != "" {
			allowed[value] = true
		}
		if label := strings.ToLower(strings.TrimSpace(option.Label)); label != "" {
			allowed[label] = true
		}
	}
	values := []string{value}
	if questionType == "multi_value_multi_select" {
		values = strings.Split(value, ",")
	}
	for _, candidate := range values {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if !allowed[strings.ToLower(candidate)] {
			return fmt.Sprintf("value %q is not one of the allowed options", candidate)
		}
	}
	return ""
}

func validateAttachments(a *domain.ApplicationArtifact, inspection *domain.ApplicationInspection) []string {
	var problems []string
	for _, attachment := range a.Attachments {
		kind := strings.TrimSpace(attachment.Kind)
		if kind == "" {
			problems = append(problems, "attachment with unspecified kind")
			continue
		}
		if strings.TrimSpace(attachment.Path) == "" {
			problems = append(problems, kind+" path")
			continue
		}
		if _, err := os.Stat(attachment.Path); err != nil {
			problems = append(problems, fmt.Sprintf("%s file %s", kind, attachment.Path))
			continue
		}
		switch strings.ToLower(kind) {
		case "resume":
			if !inspection.AcceptsResume {
				problems = append(problems, "this application does not accept a resume")
			}
		case "cover_letter":
			if !inspection.AcceptsCoverLetter {
				problems = append(problems, "this application does not accept a cover letter")
			}
		}
	}
	return problems
}

func attachmentKinds(a *domain.ApplicationArtifact) map[string]bool {
	kinds := make(map[string]bool, len(a.Attachments))
	for _, attachment := range a.Attachments {
		if kind := strings.ToLower(strings.TrimSpace(attachment.Kind)); kind != "" {
			kinds[kind] = true
		}
	}
	return kinds
}

func IsFileField(field domain.ApplicationField) bool {
	if strings.EqualFold(strings.TrimSpace(field.Type), "file") {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(field.Name)) {
	case "resume", "cover_letter":
		return true
	default:
		return false
	}
}

func ContentTypeFor(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".pdf":
		return "application/pdf"
	case ".doc":
		return "application/msword"
	case ".docx":
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case ".txt":
		return "text/plain"
	case ".md":
		return "text/markdown"
	case ".rtf":
		return "application/rtf"
	default:
		return "application/octet-stream"
	}
}

func fieldLabel(field domain.ApplicationField) string {
	if field.Label != "" {
		return field.Label
	}
	return field.Name
}

func questionLabel(question *domain.ApplicationQuestion) string {
	if question.Label != "" {
		return fmt.Sprintf("%s (%s)", question.Label, question.ID)
	}
	return question.ID
}
