package cli

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/thedavidweng/jobs-cli/v2/internal/domain"
	joberrors "github.com/thedavidweng/jobs-cli/v2/internal/errors"
)

func applyCmd(a *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "apply",
		GroupID: "apply",
		Short:   "Inspect, prepare, and submit an application",
		Long: `The application lifecycle is inspect -> prepare -> submit.

inspect and prepare take a compound Job ID and resolve the Application Target as
needed. submit consumes a versioned Application Artifact produced by prepare, and
never guesses a Job ID on its own.`,
	}
	cmd.PersistentFlags().StringVar(&a.browserEndpoint, "browser-endpoint", "", "explicit local Chrome CDP endpoint")
	cmd.PersistentFlags().StringVar(&a.browserTab, "browser-tab", "", "existing browser tab ID for this Job")
	cmd.PersistentFlags().StringVar(&a.browserState, "state", "", "browser workflow state file (required for fill and submit)")
	cmd.PersistentFlags().StringVar(&a.greenhouseKeyFile, "greenhouse-key-file", "", "employer Job Board API key file")
	cmd.PersistentFlags().StringVar(&a.greenhouseBoard, "greenhouse-board", "", "board authorized by the provided API key")
	cmd.AddCommand(applyFillCmd(a))
	cmd.AddCommand(applyInspectCmd(a))
	cmd.AddCommand(applyPrepareCmd(a))
	cmd.AddCommand(applySubmitCmd(a))
	return cmd
}

func (a *App) loadTarget(ctx context.Context, jobID string, authenticated bool) (domain.Job, domain.ApplicationTarget, *joberrors.Error) {
	job, err := a.loadJob(ctx, jobID, authenticated)
	if err != nil {
		return domain.Job{}, domain.ApplicationTarget{}, err
	}
	if job.Application != nil {
		return *job, *job.Application, nil
	}
	rawURL := applicationURL(job)
	if rawURL == "" {
		return domain.Job{}, domain.ApplicationTarget{}, joberrors.New(joberrors.ATSResolutionFailed, "job "+job.ID+" has no application URL to resolve", joberrors.CatAPI, false, nil)
	}
	target, rerr := a.registry().Resolve(ctx, rawURL)
	if rerr != nil {
		return domain.Job{}, domain.ApplicationTarget{}, rerr
	}
	return *job, *target, nil
}
