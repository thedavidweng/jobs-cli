package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/thedavidweng/jobs-cli/v2/internal/domain"
	joberrors "github.com/thedavidweng/jobs-cli/v2/internal/errors"
)

type showFlags struct {
	resolve       bool
	authenticated bool
}

func showCmd(a *App) *cobra.Command {
	f := &showFlags{}
	cmd := &cobra.Command{
		Use:     "show <job-id>",
		GroupID: "jobs",
		Short:   "Show the normalized Job for a compound Job ID",
		Long: `Show returns the normalized Job only. Application resolution is opt-in via
--resolve; a failed best-effort resolution becomes a warning, not an error.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.runShow(cmd, f, args[0])
		},
	}
	cmd.Flags().BoolVar(&f.resolve, "resolve", false, "attach a best-effort Application Target")
	cmd.Flags().BoolVar(&f.authenticated, "authenticated", false, "use authenticated LinkedIn (Voyager) instead of Guest")
	return cmd
}

func (a *App) runShow(cmd *cobra.Command, f *showFlags, jobID string) error {
	ctx := cmd.Context()
	loaded, err := a.loadJob(ctx, jobID, f.authenticated)
	if err != nil {
		return err
	}
	job := *loaded

	var warnings []string
	if f.resolve {
		target, rerr := a.registry().Resolve(ctx, applicationURL(&job))
		if rerr != nil {
			warnings = append(warnings, "resolve: "+rerr.Error())
		} else {
			job.Application = target
		}
	}
	if !a.full {
		job.Diagnostics = nil
	}
	if !a.jsonMode {
		a.printJob(&job)
	}
	return a.emit(result{Data: job, Warnings: warnings})
}

func (a *App) loadJob(ctx context.Context, jobID string, authenticated bool) (*domain.Job, *joberrors.Error) {
	source, sourceJobID, err := domain.ParseJobID(jobID)
	if err != nil {
		return nil, invalid(err.Error())
	}
	adapter, aerr := a.registry().Source(source, authenticated)
	if aerr != nil {
		return nil, aerr
	}
	job, derr := adapter.Detail(ctx, &domain.DetailRequest{SourceJobID: sourceJobID, Authenticated: authenticated})
	if derr != nil {
		return nil, joberrors.From(derr)
	}
	if job == nil {
		return nil, joberrors.New(joberrors.ResourceNotFound, "job "+jobID+" was not found", joberrors.CatAPI, false, nil)
	}
	if job.Source == "" {
		job.Source = source
	}
	if job.SourceJobID == "" {
		job.SourceJobID = sourceJobID
	}
	if job.ID == "" {
		job.ID = domain.FormatJobID(job.Source, job.SourceJobID)
	}
	return job, nil
}

func applicationURL(job *domain.Job) string {
	if job == nil {
		return ""
	}
	if job.ApplicationURL != "" {
		return job.ApplicationURL
	}
	return job.SourceURL
}

func (a *App) printJob(job *domain.Job) {
	fmt.Fprintf(a.out, "%s  %s\n", job.ID, job.Title)
	fmt.Fprintf(a.out, "  employer:     %s\n", job.Employer)
	fmt.Fprintf(a.out, "  location:     %s (%s)\n", job.Location, workplaceLabel(job))
	if job.PostedDate != "" {
		fmt.Fprintf(a.out, "  posted:       %s\n", job.PostedDate)
	}
	if job.Compensation != nil {
		fmt.Fprintf(a.out, "  compensation: %s\n", compensationLabel(job.Compensation))
	}
	if job.SourceURL != "" {
		fmt.Fprintf(a.out, "  source:       %s\n", job.SourceURL)
	}
	if job.ApplicationURL != "" {
		fmt.Fprintf(a.out, "  apply:        %s\n", job.ApplicationURL)
	}
	if job.Application != nil {
		fmt.Fprintf(a.out, "  provider:     %s (%s)\n", job.Application.Provider, job.Application.URL)
	}
	if job.Description != "" {
		if a.full {
			fmt.Fprintf(a.out, "  description:\n%s\n", job.Description)
			return
		}
		fmt.Fprintf(a.out, "  description:  %s\n", truncate(job.Description, 200))
	}
}

func workplaceLabel(job *domain.Job) string {
	if job.Remote {
		return string(domain.WorkplaceRemote)
	}
	if job.Workplace == "" {
		return string(domain.WorkplaceUnknown)
	}
	return string(job.Workplace)
}

func compensationLabel(c *domain.Compensation) string {
	parts := make([]string, 0, len(c.Amounts))
	for _, amount := range c.Amounts {
		switch {
		case amount.Min != nil && amount.Max != nil:
			parts = append(parts, fmt.Sprintf("%.0f-%.0f", *amount.Min, *amount.Max))
		case amount.Min != nil:
			parts = append(parts, fmt.Sprintf("%.0f+", *amount.Min))
		case amount.Max != nil:
			parts = append(parts, fmt.Sprintf("up to %.0f", *amount.Max))
		}
	}
	out := c.Summary
	if out == "" {
		out = joinNonEmpty(parts, ", ")
	}
	if c.Currency != "" {
		out = c.Currency + " " + out
	}
	if c.Interval != "" && c.Interval != domain.IntervalUnknown {
		out += "/" + string(c.Interval)
	}
	return out
}

func joinNonEmpty(parts []string, sep string) string {
	out := ""
	for _, part := range parts {
		if part == "" {
			continue
		}
		if out != "" {
			out += sep
		}
		out += part
	}
	return out
}

func truncate(s string, limit int) string {
	if limit <= 0 || len(s) <= limit {
		return s
	}
	return s[:limit] + "..."
}
