package cli

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/thedavidweng/jobs-cli/internal/cookieimport"
	joberrors "github.com/thedavidweng/jobs-cli/internal/errors"
)

type authLoginFlags struct {
	browser string
}

var openLoginPage = openLinkedInLogin

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
	if cookieimport.UnsupportedBrowser(f.browser) {
		return joberrors.New(joberrors.InvalidArguments, "--browser must be chrome, safari, or firefox", joberrors.CatValidation, false, nil)
	}
	if err := openLoginPage(cmd.Context()); err != nil {
		return joberrors.New(joberrors.InternalError, "could not open LinkedIn login page; open https://www.linkedin.com/login in your browser, then retry", joberrors.CatInternal, false, nil)
	}
	fmt.Fprint(a.errOut, "Finish signing in to LinkedIn in your browser, then press Enter to import the session: ")
	if _, err := bufio.NewReader(a.in).ReadString('\n'); err != nil {
		return joberrors.New(joberrors.InvalidArguments, "waiting for login confirmation failed", joberrors.CatValidation, false, err)
	}
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

func openLinkedInLogin(ctx context.Context) error {
	const loginURL = "https://www.linkedin.com/login"
	var command *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		command = exec.CommandContext(ctx, "open", loginURL)
	case "windows":
		command = exec.CommandContext(ctx, "cmd", "/c", "start", "", loginURL)
	default:
		command = exec.CommandContext(ctx, "xdg-open", loginURL)
	}
	return command.Run()
}
