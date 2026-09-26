package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/thedavidweng/jobs-cli/internal/domain"
	joberrors "github.com/thedavidweng/jobs-cli/internal/errors"
)

type applyInspectFlags struct {
	authenticated bool
}

func applyInspectCmd(a *App) *cobra.Command {
	f := &applyInspectFlags{}
	cmd := &cobra.Command{
		Use:   "inspect <job-id>",
		Short: "Fetch application requirements without mutating anything",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.runApplyInspect(cmd, f, args[0])
		},
	}
	cmd.Flags().BoolVar(&f.authenticated, "authenticated", false, "use authenticated LinkedIn (Voyager) instead of Guest")
	return cmd
}

func (a *App) runApplyInspect(cmd *cobra.Command, f *applyInspectFlags, jobID string) error {
	ctx := cmd.Context()
	job, target, err := a.loadTarget(ctx, jobID, f.authenticated)
	if err != nil {
		return err
	}
	provider, perr := a.registry().Provider(target.Provider)
	if perr != nil {
		return perr
	}
	inspection, ierr := provider.Inspect(ctx, &domain.InspectRequest{Job: job, Target: target})
	if ierr != nil {
		return joberrors.From(ierr)
	}
	if !a.jsonMode {
		a.printInspection(inspection)
	}
	return a.emit(result{Data: inspection})
}

func (a *App) printInspection(inspection *domain.ApplicationInspection) {
	fmt.Fprintf(a.out, "provider:      %s\n", inspection.Provider)
	fmt.Fprintf(a.out, "url:           %s\n", inspection.Application.URL)
	fmt.Fprintf(a.out, "fingerprint:   %s\n", inspection.Fingerprint)
	fmt.Fprintf(a.out, "resume:        %t\n", inspection.AcceptsResume)
	fmt.Fprintf(a.out, "cover letter:  %t\n", inspection.AcceptsCoverLetter)
	fmt.Fprintf(a.out, "fields:        %d\n", len(inspection.Fields))
	for _, field := range inspection.Fields {
		required := ""
		if field.Required {
			required = " (required)"
		}
		fmt.Fprintf(a.out, "  %-20s %-10s%s\n", field.Name, field.Type, required)
	}
	fmt.Fprintf(a.out, "questions:     %d\n", len(inspection.Questions))
	for _, question := range inspection.Questions {
		required := ""
		if question.Required {
			required = " (required)"
		}
		fmt.Fprintf(a.out, "  %-20s %-12s %s%s\n", question.ID, question.Type, question.Label, required)
	}
}
