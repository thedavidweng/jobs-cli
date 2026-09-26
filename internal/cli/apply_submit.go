package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/thedavidweng/jobs-cli/internal/artifact"
	"github.com/thedavidweng/jobs-cli/internal/domain"
	joberrors "github.com/thedavidweng/jobs-cli/internal/errors"
	"github.com/thedavidweng/jobs-cli/internal/safety"
)

type applySubmitFlags struct {
	artifactPath string
}

func applySubmitCmd(a *App) *cobra.Command {
	f := &applySubmitFlags{}
	cmd := &cobra.Command{
		Use:   "submit",
		Short: "Submit a prepared Application Artifact",
		Long: `submit consumes an Application Artifact produced by apply prepare (a file path
or - for stdin), re-inspects the remote requirements, and refuses to submit if the
artifact fingerprint is stale. Submission requires --confirm and is never retried.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.runApplySubmit(cmd, f)
		},
	}
	cmd.Flags().StringVar(&f.artifactPath, "artifact", "", "Application Artifact file (- for stdin)")
	return cmd
}

func (a *App) runApplySubmit(cmd *cobra.Command, f *applySubmitFlags) error {
	ctx := cmd.Context()
	if f.artifactPath == "" {
		return invalid("--artifact <path> is required (use - to read the artifact from stdin)")
	}
	prepared, aerr := a.loadArtifact(f.artifactPath)
	if aerr != nil {
		return aerr
	}
	provider, perr := a.registry().Provider(prepared.Provider)
	if perr != nil {
		return perr
	}

	job := domain.Job{
		ID:       prepared.JobID,
		Title:    prepared.JobTitle,
		Employer: prepared.Employer,
		Source:   sourceFromJobID(prepared.JobID),
	}

	if a.readOnly {
		return safety.ReadOnlyError()
	}

	caps := provider.Capabilities()
	if !caps.NativeSubmit {
		plan := submitPlan(&prepared, "")
		if a.dryRun {
			if a.jsonMode {
				return a.emit(result{Data: plan})
			}
			a.renderer().RenderPlan(plan)
			return nil
		}
		return unsupportedSubmitError(&prepared, caps)
	}

	inspection, ierr := provider.Inspect(ctx, &domain.InspectRequest{Job: job, Target: prepared.Application})
	if ierr != nil {
		return joberrors.From(ierr)
	}

	plan := submitPlan(&prepared, inspection.Fingerprint)

	if a.dryRun {
		if a.jsonMode {
			return a.emit(result{Data: plan})
		}
		a.renderer().RenderPlan(plan)
		return nil
	}

	if staleErr := artifact.Stale(&prepared, inspection); staleErr != nil {
		return staleErr
	}

	if verr := artifact.Validate(&prepared, inspection); verr != nil {
		return verr
	}

	if gateErr := a.gate().Check(safety.TierMutation); gateErr != nil {
		return gateErr
	}

	submitted, serr := provider.Submit(ctx, &domain.SubmitRequest{Job: job, Target: prepared.Application, Artifact: prepared})
	if serr != nil {
		return joberrors.From(serr)
	}
	if !a.jsonMode {
		fmt.Fprintf(a.out, "submitted application for %s via %s\n", prepared.JobID, prepared.Provider)
	}
	return a.emit(result{Data: submitted})
}

func (a *App) loadArtifact(path string) (domain.ApplicationArtifact, *joberrors.Error) {
	if path == "-" {
		data, rerr := a.readInput("-")
		if rerr != nil {
			return domain.ApplicationArtifact{}, rerr
		}
		return artifact.Decode(data)
	}
	return artifact.ReadFile(path)
}

func sourceFromJobID(jobID string) domain.Source {
	source, _, err := domain.ParseJobID(jobID)
	if err != nil {
		return ""
	}
	return source
}

func submitPlan(prepared *domain.ApplicationArtifact, remoteFingerprint string) safety.Plan {
	details := map[string]any{
		"job_id":           prepared.JobID,
		"artifact_version": prepared.ArtifactVersion,
		"fingerprint":      prepared.Fingerprint,
		"attachments":      len(prepared.Attachments),
		"answers":          len(prepared.Answers),
	}
	if remoteFingerprint != "" {
		details["remote_fingerprint"] = remoteFingerprint
	}
	return safety.Plan{
		Command: "apply.submit",
		DryRun:  true,
		PlannedMutations: []safety.Mutation{{
			Action:     "submit_application",
			Target:     prepared.Application.URL,
			Provider:   string(prepared.Provider),
			ResourceID: prepared.JobID,
			Details:    details,
		}},
	}
}

func unsupportedSubmitError(prepared *domain.ApplicationArtifact, caps domain.Capabilities) *joberrors.Error {
	switch {
	case prepared.Provider == domain.ProviderLinkedIn:
		return joberrors.New(joberrors.LinkedInEasyApplyUnverified,
			"Easy Apply submission is disabled until the Voyager implementation is verified with a controlled live session; apply in a browser at "+prepared.Application.URL,
			joberrors.CatAPI, false, nil)
	case caps.BrowserRequired:
		return joberrors.New(joberrors.BrowserRequired,
			"native submission is not available for "+string(prepared.Provider)+"; apply in a browser at "+prepared.Application.URL,
			joberrors.CatAPI, false, nil)
	default:
		return joberrors.New(joberrors.NativeApplyUnsupported,
			"no native application implementation exists for "+string(prepared.Provider),
			joberrors.CatAPI, false, nil)
	}
}
