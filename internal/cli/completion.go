package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func completionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "completion [bash|zsh|fish|powershell]",
		Short: "Generate shell completion scripts",
		Long: `To load completions:

Bash:
  $ source <(jobs-cli completion bash)

Zsh:
  $ jobs-cli completion zsh > "${fpath[1]}/_jobs-cli"

Fish:
  $ jobs-cli completion fish | source
`,
		GroupID:               "utility",
		DisableFlagsInUseLine: true,
		ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
		Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			switch args[0] {
			case "bash":
				return cmd.Root().GenBashCompletionV2(out, true)
			case "zsh":
				return cmd.Root().GenZshCompletion(out)
			case "fish":
				return cmd.Root().GenFishCompletion(out, true)
			case "powershell":
				return cmd.Root().GenPowerShellCompletionWithDesc(out)
			default:
				fmt.Fprintf(cmd.ErrOrStderr(), "unknown shell: %s\n", args[0])
				return nil
			}
		},
	}
}
