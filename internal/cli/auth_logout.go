package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	joberrors "github.com/thedavidweng/jobs-cli/internal/errors"
)

func authLogoutCmd(a *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logout",
		Short: "Remove the stored local session for the active profile",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.runAuthLogout()
		},
	}
	return cmd
}

func (a *App) runAuthLogout() error {
	service := a.registry().LinkedInAuth
	if service == nil {
		return joberrors.New(joberrors.NotImplemented, "session removal is not available", joberrors.CatInternal, false, nil)
	}
	removed, err := service.Logout()
	if err != nil {
		return joberrors.New(joberrors.InternalError, err.Error(), joberrors.CatInternal, false, err)
	}
	payload := map[string]any{"profile": a.profile, "removed": removed}
	if !a.jsonMode {
		if removed {
			fmt.Fprintf(a.out, "removed stored session for profile %s\n", a.profile)
		} else {
			fmt.Fprintf(a.out, "no stored session for profile %s\n", a.profile)
		}
	}
	return a.emit(result{Data: payload})
}
