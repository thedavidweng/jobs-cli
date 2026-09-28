package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/thedavidweng/jobs-cli/v2/internal/version"
)

// Doctor check statuses. info marks optional setup that is not done, which is
// not a problem, so OK stays true for it and only warn sets OK to false.
const (
	checkOK   = "ok"
	checkInfo = "info"
	checkWarn = "warn"
)

type doctorCheck struct {
	Check  string `json:"check"`
	Status string `json:"status"`
	OK     bool   `json:"ok"`
	Detail string `json:"detail"`
}

type doctorReport struct {
	Profile    string        `json:"profile"`
	ConfigFile string        `json:"config_file"`
	ConfigDir  string        `json:"config_dir"`
	Checks     []doctorCheck `json:"checks"`
	Connect    bool          `json:"connect"`
}

type doctorFlags struct {
	connect bool
}

func doctorCmd(a *App) *cobra.Command {
	f := &doctorFlags{}
	cmd := &cobra.Command{
		Use:     "doctor",
		GroupID: "utility",
		Short:   "Check local installation, config, and session state",
		Long: `doctor inspects the local installation: config file, config directory
permissions, active profile, configured sources, session file, and version.
Each check reports ok, info for optional setup that is not done (such as the
LinkedIn login), or WARN for a problem to fix.
Capability reporting for sources and providers lives in 'jobs-cli sources status'.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.runDoctor(cmd, f)
		},
	}
	cmd.Flags().BoolVar(&f.connect, "connect", false, "also run optional connectivity checks")
	return cmd
}

func (a *App) runDoctor(cmd *cobra.Command, f *doctorFlags) error {
	cfg := a.config()
	path := a.defaultConfigPath()
	report := doctorReport{
		Profile:    a.profile,
		ConfigFile: path,
		ConfigDir:  cfg.Dir(),
		Connect:    f.connect,
	}

	checks := make([]doctorCheck, 0, 8)
	add := func(name, status, detail string) {
		checks = append(checks, doctorCheck{Check: name, Status: status, OK: status != checkWarn, Detail: detail})
	}
	okOrWarn := func(ok bool) string {
		if ok {
			return checkOK
		}
		return checkWarn
	}

	if a.cfgErr != nil {
		add("config_file", checkWarn, a.cfgErr.Error())
	} else if _, err := os.Stat(path); err != nil {
		add("config_file", checkOK, path+" (not present; built-in defaults are in use)")
	} else {
		add("config_file", checkOK, path)
	}

	switch info, err := os.Stat(cfg.Dir()); {
	case errors.Is(err, fs.ErrNotExist):
		add("config_dir", checkOK, cfg.Dir()+" (not created yet; created when a LinkedIn session is first saved)")
	case err != nil:
		add("config_dir", checkWarn, err.Error())
	case !info.IsDir():
		add("config_dir", checkWarn, cfg.Dir()+" (not a directory)")
	default:
		add("config_dir", checkOK, fmt.Sprintf("%s (mode %04o)", cfg.Dir(), info.Mode().Perm()))
	}

	profileOK := cfg.Active != nil
	profileName := a.profile
	if cfg.ProfileName != "" {
		profileName = cfg.ProfileName
	}
	add("profile", okOrWarn(profileOK), profileName)

	add("timeout", okOrWarn(a.timeout > 0), a.timeout.String())

	sources := cfg.Sources()
	add("sources", okOrWarn(len(sources) > 0), strings.Join(sources, ", "))

	status := cfg.SessionStore().Status()
	switch {
	case status.Complete:
		add("session", checkOK, fmt.Sprintf("present (%s), cookies: %s", status.CapturedAt, strings.Join(status.CookieNames, ", ")))
	case status.Invalid:
		add("session", checkWarn, "present but invalid: "+status.InvalidReason+"; "+linkedInSessionReplaceHint)
	default:
		add("session", checkInfo, "absent (optional); run `jobs-cli auth linkedin login` for signed-in LinkedIn features")
	}

	add("version", checkOK, version.GetVersion())

	if f.connect {
		for _, endpoint := range []struct{ name, url string }{
			{"indeed", "https://apis.indeed.com/graphql"},
			{"linkedin", "https://www.linkedin.com/jobs-guest/jobs/api/seeMoreJobPostings/search"},
			{"greenhouse", "https://boards-api.greenhouse.io"},
		} {
			ok, detail := a.checkConnectivity(cmd.Context(), endpoint.url)
			add("connect:"+endpoint.name, okOrWarn(ok), detail)
		}
	}

	report.Checks = checks

	if !a.jsonMode {
		a.printDoctor(&report)
	}
	return a.emit(result{Data: report})
}

func (a *App) checkConnectivity(ctx context.Context, raw string) (ok bool, detail string) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, http.NoBody)
	if err != nil {
		return false, "invalid endpoint: " + err.Error()
	}
	resp, err := a.httpClient().Do(req)
	if err != nil {
		return false, "unreachable: " + err.Error()
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	return true, fmt.Sprintf("reachable (HTTP %d)", resp.StatusCode)
}

func (a *App) printDoctor(report *doctorReport) {
	fmt.Fprintf(a.out, "profile:     %s\n", report.Profile)
	fmt.Fprintf(a.out, "config file: %s\n", report.ConfigFile)
	fmt.Fprintf(a.out, "config dir:  %s\n", report.ConfigDir)
	problems := 0
	for _, check := range report.Checks {
		label := "ok  "
		switch check.Status {
		case checkInfo:
			label = "info"
		case checkWarn:
			label = "WARN"
			problems++
		}
		fmt.Fprintf(a.out, "[%s] %-12s %s\n", label, check.Check, check.Detail)
	}
	switch problems {
	case 0:
		fmt.Fprintln(a.out, "\nNo problems found.")
	case 1:
		fmt.Fprintln(a.out, "\n1 problem found. Fix the WARN line above.")
	default:
		fmt.Fprintf(a.out, "\n%d problems found. Fix the WARN lines above.\n", problems)
	}
}
