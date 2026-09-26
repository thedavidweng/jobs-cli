package artifact

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
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
	var missing []string
	for _, field := range inspection.Fields {
		if field.Required && a.Candidate.Value(field.Name) == "" {
			missing = append(missing, fieldLabel(field))
		}
	}
	answers := AnswerMap(a)
	for i := range inspection.Questions {
		question := &inspection.Questions[i]
		if question.Required && answers[question.ID] == "" {
			missing = append(missing, questionLabel(question))
		}
	}
	for _, attachment := range a.Attachments {
		if attachment.Kind == "" {
			missing = append(missing, "attachment with unspecified kind")
			continue
		}
		if attachment.Path == "" {
			missing = append(missing, attachment.Kind+" path")
			continue
		}
		if _, err := os.Stat(attachment.Path); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				missing = append(missing, fmt.Sprintf("%s file %s", attachment.Kind, attachment.Path))
				continue
			}
			return joberrors.New(joberrors.ApplicationIncomplete, fmt.Sprintf("cannot read %s file %s: %v", attachment.Kind, attachment.Path, err), joberrors.CatValidation, false, err)
		}
	}
	if len(missing) > 0 {
		return joberrors.New(joberrors.ApplicationIncomplete,
			"application is incomplete; missing required input: "+strings.Join(missing, ", "),
			joberrors.CatValidation, false, nil)
	}
	return nil
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
