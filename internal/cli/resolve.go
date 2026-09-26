package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/thedavidweng/jobs-cli/internal/domain"
	joberrors "github.com/thedavidweng/jobs-cli/internal/errors"
)

type resolveFlags struct {
	authenticated bool
}

func resolveCmd(a *App) *cobra.Command {
	f := &resolveFlags{}
	cmd := &cobra.Command{
		Use:     "resolve <job-id|url>",
		GroupID: "jobs",
		Short:   "Resolve the Application Target for a Job ID or application URL",
		Long: `resolve answers "where and how do I apply?" and returns only an
Application Target (canonical URL, Application Provider, identifiers, capabilities).
Use show for Job fields.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.runResolve(cmd, f, args[0])
		},
	}
	cmd.Flags().BoolVar(&f.authenticated, "authenticated", false, "use authenticated LinkedIn (Voyager) instead of Guest")
	return cmd
}

func (a *App) runResolve(cmd *cobra.Command, f *resolveFlags, input string) error {
	ctx := cmd.Context()

	rawURL := input
	if domain.LooksLikeJobID(input) {
		job, err := a.loadJob(ctx, input, f.authenticated)
		if err != nil {
			return err
		}
		if job.Application != nil {
			return a.reportTarget(job.Application)
		}
		rawURL = applicationURL(job)
		if rawURL == "" {
			return joberrors.New(joberrors.ATSResolutionFailed, "job "+input+" has no application URL to resolve", joberrors.CatAPI, false, nil)
		}
	} else if !strings.Contains(input, "://") {
		return invalid(fmt.Sprintf("expected a compound Job ID (<source>:<source-job-id>) or an absolute URL, got %q", input))
	}

	target, err := a.registry().Resolve(ctx, rawURL)
	if err != nil {
		return err
	}
	return a.reportTarget(target)
}

func (a *App) reportTarget(target *domain.ApplicationTarget) error {
	if !a.jsonMode {
		a.printTarget(target)
	}
	return a.emit(result{Data: target})
}

func (a *App) printTarget(target *domain.ApplicationTarget) {
	fmt.Fprintf(a.out, "provider:     %s\n", target.Provider)
	fmt.Fprintf(a.out, "url:          %s\n", target.URL)
	if target.Verification != "" {
		fmt.Fprintf(a.out, "verification: %s\n", target.Verification)
	}
	fmt.Fprintf(a.out, "capabilities: inspect=%t prepare=%t native_submit=%t browser_required=%t auth_required=%t\n",
		target.Capabilities.Inspect,
		target.Capabilities.Prepare,
		target.Capabilities.NativeSubmit,
		target.Capabilities.BrowserRequired,
		target.Capabilities.AuthRequired)
}
