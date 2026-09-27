package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/thedavidweng/jobs-cli/v2/internal/version"
)

type versionPayload struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	Date      string `json:"date"`
	BuiltBy   string `json:"built_by"`
	GoVersion string `json:"go_version"`
	BuildInfo string `json:"build_info"`
}

func versionCmd(a *App) *cobra.Command {
	return &cobra.Command{
		Use:     "version",
		GroupID: "utility",
		Short:   "Print the jobs-cli version",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			payload := versionPayload{
				Version:   version.GetVersion(),
				Commit:    version.GetCommit(),
				Date:      version.GetDate(),
				BuiltBy:   version.GetBuiltBy(),
				GoVersion: version.GetGoVersion(),
				BuildInfo: version.GetBuildInfo(),
			}
			if !a.jsonMode {
				fmt.Fprintf(a.out, "jobs-cli version %s (commit: %s, date: %s, built by: %s)\n",
					payload.Version, payload.Commit, payload.Date, payload.BuiltBy)
				return nil
			}
			return a.emit(result{Data: payload})
		},
	}
}
