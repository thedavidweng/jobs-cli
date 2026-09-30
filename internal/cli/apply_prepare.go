package cli

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/thedavidweng/jobs-cli/v2/internal/artifact"
	"github.com/thedavidweng/jobs-cli/v2/internal/domain"
	joberrors "github.com/thedavidweng/jobs-cli/v2/internal/errors"
	"github.com/thedavidweng/jobs-cli/v2/internal/resumejson"
)

type applyPrepareFlags struct {
	authenticated    bool
	answersFile      string
	manifestFile     string
	resumeJSON       string
	resume           string
	coverLetter      string
	previousArtifact string
	out              string
}

type prepareInput struct {
	candidateKeys map[string]json.RawMessage
	ResumeJSON    json.RawMessage            `json:"resume_json,omitempty"`
	Candidate     domain.Candidate           `json:"candidate"`
	Answers       []domain.ApplicationAnswer `json:"answers"`
	Attachments   []domain.Attachment        `json:"attachments"`
	Resume        string                     `json:"resume"`
	CoverLetter   string                     `json:"cover_letter"`
}

func applyPrepareCmd(a *App) *cobra.Command {
	f := &applyPrepareFlags{}
	cmd := &cobra.Command{
		Use:   "prepare <job-id>",
		Short: "Validate candidate input and produce a versioned Application Artifact",
		Long: `prepare never submits anything. It validates required fields, supported
types, selected options, and referenced files, then produces an Application Artifact.

In --json mode the artifact is emitted on stdout. In human-readable mode --out <path>
is required: the CLI never chooses a default artifact path.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.runApplyPrepare(cmd, f, args[0])
		},
	}
	cmd.Flags().BoolVar(&f.authenticated, "authenticated", false, "use authenticated LinkedIn (Voyager) instead of Guest")
	cmd.Flags().StringVar(&f.answersFile, "answers", "", "answers JSON file (- for stdin)")
	cmd.Flags().StringVar(&f.manifestFile, "manifest", "", "manifest JSON file embedding candidate, answers, and attachment paths (- for stdin)")
	cmd.Flags().StringVar(&f.resumeJSON, "resume-json", "", "JSON Resume v1.0.0 input file")
	cmd.Flags().StringVar(&f.resume, "resume", "", "resume file path")
	cmd.Flags().StringVar(&f.coverLetter, "cover-letter", "", "cover letter file path")
	cmd.Flags().StringVar(&f.previousArtifact, "previous-artifact", "", "previous Workday artifact to retain reviewed steps when preparing the next step")
	cmd.Flags().StringVar(&f.out, "out", "", "write the Application Artifact to this path")
	return cmd
}

func (a *App) runApplyPrepare(cmd *cobra.Command, f *applyPrepareFlags, jobID string) error {
	ctx := cmd.Context()
	job, target, err := a.loadTarget(ctx, jobID, f.authenticated)
	if err != nil {
		return err
	}
	provider, perr := a.applyProvider(target.Provider)
	if perr != nil {
		return perr
	}
	inspection, ierr := provider.Inspect(ctx, &domain.InspectRequest{Job: job, Target: target})
	if ierr != nil {
		return joberrors.From(ierr)
	}

	input, inputErr := a.prepareInputs(f)
	if inputErr != nil {
		return inputErr
	}
	attachments, attachmentErr := collectAttachments(&input, f)
	if attachmentErr != nil {
		return attachmentErr
	}

	prepared := domain.ApplicationArtifact{
		PendingAction: inspection.PendingAction, RequirementsValidated: inspection.PendingAction == "",
		SchemaVersion:   domain.ArtifactSchemaVersion,
		ArtifactVersion: domain.ArtifactVersion,
		GeneratedAt:     time.Now().UTC().Format(time.RFC3339),
		JobID:           job.ID,
		JobTitle:        job.Title,
		Employer:        job.Employer,
		Provider:        target.Provider,
		Application:     target,
		Fingerprint:     inspection.Fingerprint,
		ResumeJSON:      input.ResumeJSON,
		Candidate:       input.Candidate,
		Answers:         input.Answers,
		Attachments:     attachments,
	}

	if target.Provider == domain.ProviderWorkday {
		if f.previousArtifact != "" {
			previous, err := a.loadArtifact(f.previousArtifact)
			if err != nil {
				return err
			}
			if previous.Application.URL != target.URL {
				return invalid("previous artifact belongs to another Job")
			}
			prepared.BrowserSteps = previous.BrowserSteps
		}
		var steps []domain.ApplicationInspection
		for index := range prepared.BrowserSteps {
			step := &prepared.BrowserSteps[index]
			if step.Step != inspection.Step {
				steps = append(steps, *step)
			}
		}
		steps = append(steps, *inspection)
		prepared.BrowserSteps = steps
	}
	if inspection.PendingAction == "" {
		if verr := artifact.Validate(&prepared, inspection); verr != nil {
			return verr
		}
	}
	warnings := artifact.Warnings(&prepared, inspection)
	if inspection.PendingAction != "" {
		warnings = append(warnings, "application requirements are unvalidated: "+inspection.PendingAction)
	}

	if f.out != "" {
		if werr := artifact.WriteFile(f.out, &prepared); werr != nil {
			return werr
		}
		if !a.jsonMode {
			fmt.Fprintf(a.out, "wrote Application Artifact for %s (%s) to %s\n", job.ID, target.Provider, f.out)
		}
		return a.emit(result{
			Data:     map[string]any{"path": f.out, "artifact": prepared},
			Warnings: warnings,
		})
	}
	if !a.jsonMode {
		return invalid("human-readable mode requires --out <path> (there is no default artifact path)")
	}
	return a.emit(result{Data: prepared, Warnings: warnings})
}

func (a *App) prepareInputs(f *applyPrepareFlags) (prepareInput, *joberrors.Error) {
	input := prepareInput{}
	if f.resumeJSON != "" {
		data, err := a.readInput(f.resumeJSON)
		if err != nil {
			return input, err
		}
		candidate, err := resumejson.Parse(data)
		if err != nil {
			return input, err
		}
		input.Candidate, input.ResumeJSON = candidate, data
	}
	if f.answersFile != "" {
		data, rerr := a.readInput(f.answersFile)
		if rerr != nil {
			return prepareInput{}, rerr
		}
		parsed, perr := parsePrepareInput(data, "--answers")
		if perr != nil {
			return prepareInput{}, perr
		}
		mergePrepareInput(&input, &parsed)
	}
	if f.manifestFile != "" {
		data, rerr := a.readInput(f.manifestFile)
		if rerr != nil {
			return prepareInput{}, rerr
		}
		parsed, perr := parsePrepareInput(data, "--manifest")
		if perr != nil {
			return prepareInput{}, perr
		}
		mergePrepareInput(&input, &parsed)
	}
	if f.resumeJSON == "" && f.answersFile == "" && f.manifestFile == "" && f.resume == "" && f.coverLetter == "" {
		return prepareInput{}, invalid("provide candidate input via --manifest, --answers, --resume, or --cover-letter")
	}
	return input, nil
}

func parsePrepareInput(data []byte, flag string) (prepareInput, *joberrors.Error) {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return prepareInput{}, invalid(flag + " payload is empty")
	}
	if strings.HasPrefix(trimmed, "[") {
		answers := []domain.ApplicationAnswer{}
		if err := json.Unmarshal(data, &answers); err != nil {
			return prepareInput{}, invalid(flag + " is not a valid answer array")
		}
		return prepareInput{Answers: answers}, nil
	}
	input := prepareInput{}
	if err := json.Unmarshal(data, &input); err != nil {
		return prepareInput{}, invalid(flag + " is not valid JSON: " + err.Error())
	}
	var raw struct {
		Candidate map[string]json.RawMessage `json:"candidate"`
	}
	_ = json.Unmarshal(data, &raw)
	input.candidateKeys = raw.Candidate
	if len(input.Answers) == 0 {
		asMap := map[string]any{}
		if err := json.Unmarshal(data, &asMap); err == nil {
			if _, hasAnswers := asMap["answers"]; !hasAnswers {
				if _, hasCandidate := asMap["candidate"]; !hasCandidate && !hasAttachmentKeys(asMap) {
					for key, value := range asMap {
						input.Answers = append(input.Answers, domain.ApplicationAnswer{QuestionID: key, Value: value})
					}
				}
			}
		}
	}
	return input, nil
}

func hasAttachmentKeys(raw map[string]any) bool {
	for _, key := range []string{"attachments", "resume", "cover_letter"} {
		if _, ok := raw[key]; ok {
			return true
		}
	}
	return false
}

func mergePrepareInput(base, override *prepareInput) {
	// Merge explicit candidate keys, including intentionally empty values.
	baseData, _ := json.Marshal(base.Candidate)
	overrideData, _ := json.Marshal(override.Candidate)
	values := map[string]json.RawMessage{}
	_ = json.Unmarshal(baseData, &values)
	overrides := override.candidateKeys
	if overrides == nil {
		_ = json.Unmarshal(overrideData, &overrides)
	}
	for key, value := range overrides {
		values[key] = value
	}
	merged, _ := json.Marshal(values)
	_ = json.Unmarshal(merged, &base.Candidate)
	if len(override.Answers) > 0 {
		base.Answers = override.Answers
	}
	if len(override.Attachments) > 0 {
		base.Attachments = override.Attachments
	}
	if override.Resume != "" {
		base.Resume = override.Resume
	}
	if override.CoverLetter != "" {
		base.CoverLetter = override.CoverLetter
	}
}

func collectAttachments(input *prepareInput, f *applyPrepareFlags) ([]domain.Attachment, *joberrors.Error) {
	attachments := make([]domain.Attachment, 0, len(input.Attachments))
	for _, item := range input.Attachments {
		normalized, err := attachmentFromFile(item.Kind, item.Path)
		if err != nil {
			return nil, err
		}
		attachments = append(attachments, normalized)
	}
	sources := []struct {
		kind string
		path string
	}{
		{"resume", input.Resume},
		{"cover_letter", input.CoverLetter},
		{"resume", f.resume},
		{"cover_letter", f.coverLetter},
	}
	for _, source := range sources {
		if source.path == "" {
			continue
		}
		attachment, err := attachmentFromFile(source.kind, source.path)
		if err != nil {
			return nil, err
		}
		attachments = append(attachments, attachment)
	}
	return dedupeAttachments(attachments), nil
}

func attachmentFromFile(kind, path string) (domain.Attachment, *joberrors.Error) {
	info, err := os.Stat(path)
	if err != nil {
		return domain.Attachment{}, joberrors.New(joberrors.ApplicationIncomplete, fmt.Sprintf("%s file %s is not readable", kind, path), joberrors.CatValidation, false, err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return domain.Attachment{}, invalid("read attachment: " + err.Error())
	}
	return domain.Attachment{
		SHA256:      fmt.Sprintf("%x", sha256.Sum256(data)),
		Kind:        kind,
		Path:        path,
		Filename:    filepath.Base(path),
		ContentType: artifact.ContentTypeFor(path),
		Size:        info.Size(),
	}, nil
}

func dedupeAttachments(attachments []domain.Attachment) []domain.Attachment {
	seen := map[string]bool{}
	out := make([]domain.Attachment, 0, len(attachments))
	for i := len(attachments) - 1; i >= 0; i-- {
		attachment := attachments[i]
		key := attachment.Kind
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, attachment)
	}
	slices.Reverse(out)
	return out
}
