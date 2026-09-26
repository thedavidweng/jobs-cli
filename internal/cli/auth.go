package cli

import (
	"github.com/spf13/cobra"
)

func authCmd(a *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "auth",
		GroupID: "access",
		Short:   "Manage local session state",
	}
	cmd.AddCommand(authStatusCmd(a))
	cmd.AddCommand(authLinkedInCmd(a))
	cmd.AddCommand(authLogoutCmd(a))
	return cmd
}

func authLinkedInCmd(a *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "linkedin",
		Short: "LinkedIn session",
	}
	cmd.AddCommand(authLinkedInLoginCmd(a))
	return cmd
}
