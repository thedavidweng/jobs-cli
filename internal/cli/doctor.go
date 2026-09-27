package cli

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/thedavidweng/jobs-cli/internal/version"
)

type doctorCheck struct {
	Check  string `json:"check"`
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
	add := func(name string, ok bool, detail string) {
		checks = append(checks, doctorCheck{Check: name, OK: ok, Detail: detail})
	}

	if a.cfgErr != nil {
		add("config_file", false, a.cfgErr.Error())
	} else if _, err := os.Stat(path); err != nil {
		add("config_file", true, path+" (not present; built-in defaults are in use)")
	} else {
		add("config_file", true, path)
	}

	if info, err := os.Stat(cfg.Dir()); err != nil {
		add("config_dir", false, cfg.Dir()+" (not created yet)")
	} else {
		add("config_dir", info.IsDir(), fmt.Sprintf("%s (mode %04o)", cfg.Dir(), info.Mode().Perm()))
	}

	profileOK := cfg.Active != nil
	profileName := a.profile
	if cfg.ProfileName != "" {
		profileName = cfg.ProfileName
	}
	add("profile", profileOK, profileName)

	add("timeout", a.timeout > 0, a.timeout.String())

	sources := cfg.Sources()
	add("sources", len(sources) > 0, strings.Join(sources, ", "))

	status := cfg.SessionStore().Status()
	switch {
	case status.Complete:
		add("session", true, fmt.Sprintf("present (%s), cookies: %s", status.CapturedAt, strings.Join(status.CookieNames, ", ")))
	case status.Invalid:
		add("session", false, "present but invalid: "+status.InvalidReason+"; run `jobs-cli auth linkedin login` or `jobs-cli auth linkedin import --from-json -`")
	case status.Present:
		add("session", false, "present but incomplete (needs li_at and JSESSIONID); run `jobs-cli auth linkedin login`")
	default:
		add("session", false, "absent; run `jobs-cli auth linkedin login` for authenticated LinkedIn")
	}

	add("version", true, version.GetVersion())

	if f.connect {
		for _, endpoint := range []struct{ name, url string }{
			{"indeed", "https://apis.indeed.com/graphql"},
			{"linkedin", "https://www.linkedin.com/jobs-guest/jobs/api/seeMoreJobPostings/search"},
			{"greenhouse", "https://boards-api.greenhouse.io"},
		} {
			ok, detail := a.checkConnectivity(cmd.Context(), endpoint.url)
			add("connect:"+endpoint.name, ok, detail)
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
	for _, check := range report.Checks {
		status := "ok  "
		if !check.OK {
			status = "WARN"
		}
		fmt.Fprintf(a.out, "[%s] %-12s %s\n", status, check.Check, check.Detail)
	}
}
