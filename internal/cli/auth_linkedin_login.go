package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	joberrors "github.com/thedavidweng/jobs-cli/internal/errors"
)

type authLoginFlags struct {
	browser string
}

func authLinkedInLoginCmd(a *App) *cobra.Command {
	f := &authLoginFlags{}
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Guided LinkedIn login followed by browser cookie import",
		Long: `Opens LinkedIn's login page, waits for you to finish, then imports the
Voyager session cookies from your local browser cookie store into the per-profile
session file. This is session import, not OAuth. Session secrets are never printed.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.runAuthLinkedInLogin(cmd, f)
		},
	}
	cmd.Flags().StringVar(&f.browser, "browser", "", "browser to import cookies from (chrome, safari, firefox); auto-detected when omitted")
	return cmd
}

func (a *App) runAuthLinkedInLogin(cmd *cobra.Command, f *authLoginFlags) error {
	service := a.registry().LinkedInAuth
	if service == nil {
		return joberrors.New(joberrors.NotImplemented, "guided LinkedIn session import is not implemented yet", joberrors.CatInternal, false, nil)
	}
	session, err := service.Login(cmd.Context(), f.browser)
	if err != nil {
		return joberrors.From(err)
	}
	payload := map[string]any{
		"profile":      session.Profile,
		"browser":      session.Browser,
		"captured_at":  session.CapturedAt,
		"cookie_names": session.CookieNames(),
	}
	if !a.jsonMode {
		fmt.Fprintf(a.out, "stored LinkedIn session for profile %s (%s, %d cookies)\n", session.Profile, session.Browser, len(session.Cookies))
	}
	return a.emit(result{Data: payload})
}
