package artifact_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thedavidweng/jobs-cli/internal/artifact"
	"github.com/thedavidweng/jobs-cli/internal/domain"
	joberrors "github.com/thedavidweng/jobs-cli/internal/errors"
)

func requireCode(t *testing.T, err *joberrors.Error, code string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error code %s, got nil", code)
	}
	if err.Code != joberrors.Code(code) {
		t.Fatalf("error code = %s, want %s (%v)", err.Code, code, err)
	}
}

func baseInspection() *domain.ApplicationInspection {
	target := domain.ApplicationTarget{
		URL:      "https://boards.greenhouse.io/acme/jobs/1",
		Provider: domain.ProviderGreenhouse,
	}
	fields := []domain.ApplicationField{
		{Name: "first_name", Label: "First Name", Type: "input_text", Required: true},
		{Name: "email", Label: "Email", Type: "input_text", Required: true},
		{Name: "resume", Label: "Resume", Type: "file", Required: true},
	}
	questions := []domain.ApplicationQuestion{
		{ID: "q_text", Label: "Why this role?", Type: "textarea", Required: true},
		{ID: "q_select", Label: "How did you hear?", Type: "multi_value_single_select", Options: []domain.QuestionOption{{Value: "job_board", Label: "Job board"}, {Value: "referral"}}},
		{ID: "q_multi", Label: "Skills", Type: "multi_value_multi_select", Options: []domain.QuestionOption{{Value: "go"}, {Value: "rust"}}},
		{ID: "q_file", Label: "Portfolio", Type: "input_file"},
	}
	inspection := &domain.ApplicationInspection{
		Provider:           domain.ProviderGreenhouse,
		Application:        target,
		Fields:             fields,
		Questions:          questions,
		AcceptsResume:      true,
		AcceptsCoverLetter: false,
	}
	inspection.Fingerprint = domain.Fingerprint(&target, fields, questions)
	return inspection
}

func writeTemp(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

func validArtifact(t *testing.T) domain.ApplicationArtifact {
	t.Helper()
	resume := writeTemp(t, "resume.pdf", "resume-bytes")
	return domain.ApplicationArtifact{
		SchemaVersion:   domain.ArtifactSchemaVersion,
		ArtifactVersion: domain.ArtifactVersion,
		JobID:           "indeed:1",
		Provider:        domain.ProviderGreenhouse,
		Application:     baseInspection().Application,
		Fingerprint:     baseInspection().Fingerprint,
		Candidate:       domain.Candidate{FirstName: "Ada", LastName: "Lovelace", Email: "ada@example.com"},
		Answers: []domain.ApplicationAnswer{
			{QuestionID: "q_text", Value: "Because"},
			{QuestionID: "q_select", Value: "job_board"},
			{QuestionID: "q_multi", Value: "go,rust"},
		},
		Attachments: []domain.Attachment{{Kind: "resume", Path: resume, Filename: "resume.pdf"}},
	}
}

func TestEncodeDecodeRoundTrip(t *testing.T) {
	original := validArtifact(t)
	data, err := artifact.Encode(&original)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	decoded, derr := artifact.Decode(data)
	if derr != nil {
		t.Fatalf("decode: %v", derr)
	}
	if decoded.ArtifactVersion != domain.ArtifactVersion || decoded.JobID != original.JobID || len(decoded.Answers) != len(original.Answers) {
		t.Fatalf("decoded = %+v", decoded)
	}
}

func TestDecodeAcceptsEnvelopeWrappedArtifact(t *testing.T) {
	original := validArtifact(t)
	data, err := artifact.Encode(&original)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	wrapped, err := json.Marshal(map[string]any{"ok": true, "data": json.RawMessage(data)})
	if err != nil {
		t.Fatalf("wrap: %v", err)
	}
	decoded, derr := artifact.Decode(wrapped)
	if derr != nil {
		t.Fatalf("decode envelope: %v", derr)
	}
	if decoded.JobID != original.JobID {
		t.Fatalf("job id = %s", decoded.JobID)
	}
}

func TestDecodeRejectsUnsupportedVersion(t *testing.T) {
	original := validArtifact(t)
	original.ArtifactVersion = domain.ArtifactVersion + 1
	data, err := artifact.Encode(&original)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	_, derr := artifact.Decode(data)
	requireCode(t, derr, "VALIDATION_FAILED")
}

func TestDecodeRejectsEmptyPayload(t *testing.T) {
	if _, derr := artifact.Decode([]byte("   ")); derr == nil {
		t.Fatal("expected an error for an empty payload")
	} else {
		requireCode(t, derr, "INVALID_ARGUMENTS")
	}
}

func TestValidateAcceptsCompleteArtifact(t *testing.T) {
	prepared := validArtifact(t)
	if err := artifact.Validate(&prepared, baseInspection()); err != nil {
		t.Fatalf("validate: %v", err)
	}
}

func TestValidateRequiresCandidateFields(t *testing.T) {
	prepared := validArtifact(t)
	prepared.Candidate.FirstName = ""
	err := artifact.Validate(&prepared, baseInspection())
	requireCode(t, err, "APPLICATION_INCOMPLETE")
	if !contains(err.Message, "First Name") {
		t.Fatalf("message = %q", err.Message)
	}
}

func TestValidateRequiresAnswers(t *testing.T) {
	prepared := validArtifact(t)
	prepared.Answers = prepared.Answers[1:]
	err := artifact.Validate(&prepared, baseInspection())
	requireCode(t, err, "APPLICATION_INCOMPLETE")
	if !contains(err.Message, "Why this role?") {
		t.Fatalf("message = %q", err.Message)
	}
}

func TestValidateRejectsUnsupportedQuestionType(t *testing.T) {
	prepared := validArtifact(t)
	inspection := baseInspection()
	inspection.Questions[0].Type = "color_picker"
	inspection.Fingerprint = domain.Fingerprint(&inspection.Application, inspection.Fields, inspection.Questions)
	err := artifact.Validate(&prepared, inspection)
	requireCode(t, err, "APPLICATION_INCOMPLETE")
	if !contains(err.Message, "unsupported question type") {
		t.Fatalf("message = %q", err.Message)
	}
}

func TestValidateRejectsUnknownOptionValue(t *testing.T) {
	prepared := validArtifact(t)
	prepared.Answers[1].Value = "billboard"
	err := artifact.Validate(&prepared, baseInspection())
	requireCode(t, err, "APPLICATION_INCOMPLETE")
	if !contains(err.Message, "allowed options") {
		t.Fatalf("message = %q", err.Message)
	}
}

func TestValidateAcceptsLabelForOption(t *testing.T) {
	prepared := validArtifact(t)
	prepared.Answers[1].Value = "Job board"
	if err := artifact.Validate(&prepared, baseInspection()); err != nil {
		t.Fatalf("validate by label: %v", err)
	}
}

func TestValidateMultiSelectSplitsValues(t *testing.T) {
	prepared := validArtifact(t)
	prepared.Answers[2].Value = "go,cobol"
	err := artifact.Validate(&prepared, baseInspection())
	requireCode(t, err, "APPLICATION_INCOMPLETE")
	if !contains(err.Message, "cobol") {
		t.Fatalf("message = %q", err.Message)
	}
}

func TestValidateRequiresInputFileAnswerToExist(t *testing.T) {
	prepared := validArtifact(t)
	prepared.Answers = append(prepared.Answers, domain.ApplicationAnswer{QuestionID: "q_file", Value: "/nonexistent/portfolio.pdf"})
	err := artifact.Validate(&prepared, baseInspection())
	requireCode(t, err, "APPLICATION_INCOMPLETE")
	if !contains(err.Message, "not readable") {
		t.Fatalf("message = %q", err.Message)
	}
}

func TestValidateAcceptsExistingInputFileAnswer(t *testing.T) {
	prepared := validArtifact(t)
	prepared.Answers = append(prepared.Answers, domain.ApplicationAnswer{QuestionID: "q_file", Value: writeTemp(t, "portfolio.pdf", "bytes")})
	if err := artifact.Validate(&prepared, baseInspection()); err != nil {
		t.Fatalf("validate input file: %v", err)
	}
}

func TestValidateRequiresFileFieldAttachment(t *testing.T) {
	prepared := validArtifact(t)
	prepared.Attachments = nil
	err := artifact.Validate(&prepared, baseInspection())
	requireCode(t, err, "APPLICATION_INCOMPLETE")
	if !contains(err.Message, "Resume") {
		t.Fatalf("message = %q", err.Message)
	}
}

func TestValidateRejectsMissingAttachmentFile(t *testing.T) {
	prepared := validArtifact(t)
	prepared.Attachments[0].Path = "/nonexistent/resume.pdf"
	err := artifact.Validate(&prepared, baseInspection())
	requireCode(t, err, "APPLICATION_INCOMPLETE")
	if !contains(err.Message, "resume file") {
		t.Fatalf("message = %q", err.Message)
	}
}

func TestValidateRejectsUnacceptedAttachments(t *testing.T) {
	prepared := validArtifact(t)
	inspection := baseInspection()
	inspection.AcceptsResume = false
	err := artifact.Validate(&prepared, inspection)
	requireCode(t, err, "APPLICATION_INCOMPLETE")
	if !contains(err.Message, "does not accept a resume") {
		t.Fatalf("message = %q", err.Message)
	}

	prepared = validArtifact(t)
	prepared.Attachments = append(prepared.Attachments, domain.Attachment{Kind: "cover_letter", Path: writeTemp(t, "cover.txt", "bytes")})
	err = artifact.Validate(&prepared, baseInspection())
	requireCode(t, err, "APPLICATION_INCOMPLETE")
	if !contains(err.Message, "does not accept a cover letter") {
		t.Fatalf("message = %q", err.Message)
	}
}

func TestValidateIgnoresInspection(t *testing.T) {
	if err := artifact.Validate(&domain.ApplicationArtifact{}, nil); err != nil {
		t.Fatalf("nil inspection should be a no-op: %v", err)
	}
}

func TestStaleDetectsFingerprintDrift(t *testing.T) {
	prepared := validArtifact(t)
	inspection := baseInspection()
	if err := artifact.Stale(&prepared, inspection); err != nil {
		t.Fatalf("matching fingerprint should not be stale: %v", err)
	}
	prepared.Fingerprint = "sha256:drifted"
	requireCode(t, artifact.Stale(&prepared, inspection), "ARTIFACT_STALE")
}

func TestWarningsFlagUnknownAnswers(t *testing.T) {
	prepared := validArtifact(t)
	prepared.Answers = append(prepared.Answers, domain.ApplicationAnswer{QuestionID: "q_unknown", Value: "x"})
	warnings := artifact.Warnings(&prepared, baseInspection())
	if len(warnings) != 1 || !contains(warnings[0], "q_unknown") {
		t.Fatalf("warnings = %v", warnings)
	}
}

func TestAnswerMapFlattensValues(t *testing.T) {
	prepared := validArtifact(t)
	prepared.Answers = append(prepared.Answers,
		domain.ApplicationAnswer{QuestionID: "q_bool", Value: true},
		domain.ApplicationAnswer{QuestionID: "q_list", Value: []any{"go", "rust"}},
	)
	answers := artifact.AnswerMap(&prepared)
	if answers["q_bool"] != "true" {
		t.Fatalf("bool answer = %q", answers["q_bool"])
	}
	if answers["q_list"] != "go,rust" {
		t.Fatalf("list answer = %q", answers["q_list"])
	}
}

func contains(haystack, needle string) bool {
	return strings.Contains(haystack, needle)
}
