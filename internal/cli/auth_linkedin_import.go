package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/thedavidweng/jobs-cli/internal/config"
	joberrors "github.com/thedavidweng/jobs-cli/internal/errors"
)

type authImportFlags struct {
	fromJSON string
}

func authLinkedInImportCmd(a *App) *cobra.Command {
	f := &authImportFlags{}
	cmd := &cobra.Command{
		Use:   "import",
		Short: "Import a LinkedIn session from JSON (advanced, headless)",
		Long: `Import the documented LinkedIn session JSON (docs/session-format.md) and
store it for the active profile, applying the CLI's session-storage protections
(0700 directory / 0600 file on Unix platforms).

This is the advanced/headless path for machines with no local browser cookie
store (servers, containers, agents). The guided browser import (auth linkedin
login) remains the primary path for humans. Session secrets are never printed.

Read the JSON with --from-json - (stdin). Invalid input fails with a validation
error and writes nothing.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.runAuthLinkedInImport(f)
		},
	}
	cmd.Flags().StringVar(&f.fromJSON, "from-json", "", "session JSON (- for stdin)")
	return cmd
}

func (a *App) runAuthLinkedInImport(f *authImportFlags) error {
	if f.fromJSON == "" {
		return invalid("--from-json is required (use - to read the session JSON from stdin)")
	}
	data, readErr := a.readInput(f.fromJSON)
	if readErr != nil {
		return readErr
	}
	if strings.TrimSpace(string(data)) == "" {
		return invalid("--from-json payload is empty")
	}
	session := &config.LinkedInSession{}
	if err := json.Unmarshal(data, session); err != nil {
		return joberrors.New(joberrors.InvalidArguments, "session JSON is not valid: "+err.Error(), joberrors.CatValidation, false, err)
	}
	if value, ok := session.Cookies[config.CookieJSessionID]; ok {
		session.Cookies[config.CookieJSessionID] = config.TrimSmartCookieQuotes(value)
	}
	store := a.config().SessionStore()
	if err := config.ValidateLinkedInSession(session, store.Profile()); err != nil {
		return joberrors.New(joberrors.InvalidArguments, "session JSON failed validation: "+err.Error(), joberrors.CatValidation, false, nil)
	}
	if err := store.Save(session); err != nil {
		return joberrors.New(joberrors.InternalError, "store LinkedIn session: "+err.Error(), joberrors.CatInternal, false, err)
	}
	payload := map[string]any{
		"profile":      session.Profile,
		"browser":      session.Browser,
		"captured_at":  session.CapturedAt,
		"cookie_names": session.CookieNames(),
		"path":         store.Path(),
	}
	if !a.jsonMode {
		fmt.Fprintf(a.out, "imported LinkedIn session for profile %s (%d cookies) to %s\n", session.Profile, len(session.Cookies), store.Path())
	}
	return a.emit(result{Data: payload})
}
