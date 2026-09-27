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

const linkedInLoginURL = "https://www.linkedin.com/login"

const manualLoginPrompt = "open %s in a browser, sign in to LinkedIn, then press Enter to import the session: "

type authLoginFlags struct {
	browser string
	noOpen  bool
}

var openLoginPage = openLinkedInLogin

func authLinkedInLoginCmd(a *App) *cobra.Command {
	f := &authLoginFlags{}
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Guided LinkedIn login followed by browser cookie import",
		Long: `Opens LinkedIn's login page, waits for you to finish, then imports the
Voyager session cookies from your local browser cookie store into the per-profile
session file. This is session import, not OAuth. Session secrets are never printed.

When no browser opener is available (open, xdg-open, start) it prints the login
URL and waits for you to sign in manually instead of failing. --no-open skips the
browser attempt entirely.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.runAuthLinkedInLogin(cmd, f)
		},
	}
	cmd.Flags().StringVar(&f.browser, "browser", "", "browser to import cookies from (chrome, safari, firefox); auto-detected when omitted")
	cmd.Flags().BoolVar(&f.noOpen, "no-open", false, "do not open a browser; print the login URL and wait for a manual sign-in")
	return cmd
}

func (a *App) runAuthLinkedInLogin(cmd *cobra.Command, f *authLoginFlags) error {
	if cookieimport.UnsupportedBrowser(f.browser) {
		return joberrors.New(joberrors.InvalidArguments, "--browser must be chrome, safari, or firefox", joberrors.CatValidation, false, nil)
	}
	if f.noOpen {
		a.printManualLoginPrompt(false)
	} else if err := openLoginPage(cmd.Context()); err != nil {
		a.printManualLoginPrompt(true)
	} else {
		fmt.Fprint(a.errOut, "Finish signing in to LinkedIn in your browser, then press Enter to import the session: ")
	}
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

func (a *App) printManualLoginPrompt(openerFailed bool) {
	if openerFailed {
		fmt.Fprintf(a.errOut, "could not open a browser automatically; "+manualLoginPrompt, linkedInLoginURL)
		return
	}
	fmt.Fprintf(a.errOut, manualLoginPrompt, linkedInLoginURL)
}

func openLinkedInLogin(ctx context.Context) error {
	var command *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		command = exec.CommandContext(ctx, "open", linkedInLoginURL)
	case "windows":
		command = exec.CommandContext(ctx, "cmd", "/c", "start", "", linkedInLoginURL)
	default:
		command = exec.CommandContext(ctx, "xdg-open", linkedInLoginURL)
	}
	return command.Run()
}
