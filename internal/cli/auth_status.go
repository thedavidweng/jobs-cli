package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/thedavidweng/jobs-cli/internal/config"
)

type authLinkedInStatus struct {
	Authenticated bool                 `json:"authenticated"`
	Verification  string               `json:"verification"`
	Session       config.SessionStatus `json:"session"`
}

type authStatusReport struct {
	Profile  string             `json:"profile"`
	LinkedIn authLinkedInStatus `json:"linkedin"`
}

func authStatusCmd(a *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Report session availability for the active profile",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.runAuthStatus()
		},
	}
	return cmd
}

func (a *App) runAuthStatus() error {
	cfg := a.config()
	status := cfg.SessionStore().Status()
	report := authStatusReport{
		Profile: a.profile,
		LinkedIn: authLinkedInStatus{
			Authenticated: status.Complete,
			Verification:  "VERIFIED SOURCE IMPLEMENTATION",
			Session:       status,
		},
	}
	if !a.jsonMode {
		fmt.Fprintf(a.out, "profile:  %s\n", report.Profile)
		fmt.Fprintf(a.out, "linkedin: authenticated=%t session=%t\n", report.LinkedIn.Authenticated, status.Present)
		fmt.Fprintln(a.out, "  voyager: VERIFIED SOURCE IMPLEMENTATION; Easy Apply submission remains disabled")
		switch {
		case status.Invalid:
			fmt.Fprintf(a.out, "  present but invalid: %s\n", status.InvalidReason)
			fmt.Fprintln(a.out, "  run `jobs-cli auth linkedin login` or `jobs-cli auth linkedin import --from-json -` to replace the session")
		case !status.Present:
			fmt.Fprintln(a.out, "  run `jobs-cli auth linkedin login` to import a session")
		}
	}
	return a.emit(result{Data: report})
}
