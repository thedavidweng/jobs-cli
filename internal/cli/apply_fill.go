package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/thedavidweng/jobs-cli/v2/internal/domain"
	joberrors "github.com/thedavidweng/jobs-cli/v2/internal/errors"
	"github.com/thedavidweng/jobs-cli/v2/internal/safety"
)

func applyFillCmd(a *App) *cobra.Command {
	var path string
	cmd := &cobra.Command{Use: "fill", Short: "Fill a browser application and stop at review", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		if path == "" {
			return invalid("--artifact is required")
		}
		prepared, err := a.loadArtifact(path)
		if err != nil {
			return err
		}
		if a.readOnly {
			return safety.ReadOnlyError()
		}
		if a.dryRun {
			plan := submitPlan(&prepared, "")
			plan.Command = "apply.fill"
			plan.PlannedMutations[0].Action = "fill_application"
			if !a.jsonMode {
				a.renderer().RenderPlan(plan)
			}
			return a.emit(result{Data: plan})
		}
		if err := a.gate().Check(safety.TierMutation); err != nil {
			return err
		}
		provider, err := a.applyProvider(prepared.Provider)
		if err != nil {
			return err
		}
		filler, ok := provider.(interface {
			Fill(context.Context, *domain.SubmitRequest) (*domain.SubmissionResult, error)
		})
		if !ok {
			return domain.BrowserRequiredError(string(prepared.Provider), "filling", prepared.Application.URL)
		}
		outcome, ferr := filler.Fill(cmd.Context(), &domain.SubmitRequest{Target: prepared.Application, Artifact: prepared})
		if ferr != nil {
			return joberrors.From(ferr)
		}
		if !a.jsonMode {
			fmt.Fprintf(a.out, "%s: %s\n", prepared.JobID, outcome.Status)
		}
		return a.emit(result{Data: outcome})
	}}
	cmd.Flags().StringVar(&path, "artifact", "", "prepared Application Artifact file (- for stdin)")
	return cmd
}
