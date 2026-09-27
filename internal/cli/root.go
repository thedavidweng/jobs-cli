package cli

import (
	"time"

	"github.com/spf13/cobra"

	"github.com/thedavidweng/jobs-cli/internal/version"
)

func (a *App) newRoot() *cobra.Command {
	root := &cobra.Command{
		Use:     "jobs-cli",
		Short:   "Discover jobs and apply through application providers",
		Version: version.GetVersion(),
		Long: `jobs-cli discovers jobs across sources (Indeed, LinkedIn) and routes
applications through the provider that actually accepts them (Greenhouse, LinkedIn,
Lever, Ashby, Workday, SmartRecruiters, iCIMS, or an external site).

A Source is where a Job was discovered. An Application Provider is the system that
accepts the application. They are never the same field.`,
		Example: `  jobs-cli search --query "backend engineer" --location "Vancouver, BC"
  jobs-cli --json search --query "golang" --source indeed --country US
  jobs-cli show indeed:abc123 --resolve
  jobs-cli resolve indeed:abc123
  jobs-cli apply inspect indeed:abc123
  jobs-cli apply prepare indeed:abc123 --manifest app.json --out artifact.json
  jobs-cli apply submit --artifact artifact.json --confirm`,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return a.prepare(cmd)
		},
	}

	root.SetOut(a.out)
	root.SetErr(a.errOut)

	root.AddGroup(&cobra.Group{ID: "jobs", Title: "Job Discovery"})
	root.AddGroup(&cobra.Group{ID: "apply", Title: "Applications"})
	root.AddGroup(&cobra.Group{ID: "access", Title: "Account & Sources"})
	root.AddGroup(&cobra.Group{ID: "utility", Title: "Utilities"})

	root.PersistentFlags().BoolVar(&a.jsonMode, "json", false, "emit exactly one machine-readable JSON document on stdout")
	root.PersistentFlags().BoolVar(&a.pretty, "pretty", false, "pretty-print JSON output")
	root.PersistentFlags().BoolVar(&a.full, "full", false, "print full payloads instead of summaries")
	root.PersistentFlags().BoolVar(&a.readOnly, "read-only", false, "block every remote write")
	root.PersistentFlags().BoolVar(&a.dryRun, "dry-run", false, "preview a mutation as planned_mutations without executing it")
	root.PersistentFlags().BoolVar(&a.confirm, "confirm", false, "explicitly authorize a mutation")
	root.PersistentFlags().DurationVar(&a.timeout, "timeout", 30*time.Second, "per-invocation timeout")
	root.PersistentFlags().StringVar(&a.profile, "profile", "default", "use a named profile")
	root.PersistentFlags().StringVar(&a.configPath, "config", "", "config file (default is <config-dir>/config.yaml)")

	root.AddCommand(searchCmd(a))
	root.AddCommand(showCmd(a))
	root.AddCommand(resolveCmd(a))
	root.AddCommand(applyCmd(a))
	root.AddCommand(authCmd(a))
	root.AddCommand(sourcesCmd(a))
	root.AddCommand(doctorCmd(a))
	root.AddCommand(versionCmd(a))
	root.AddCommand(completionCmd())

	return root
}
