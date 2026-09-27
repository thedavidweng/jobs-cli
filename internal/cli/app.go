package cli

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/thedavidweng/jobs-cli/internal/config"
	joberrors "github.com/thedavidweng/jobs-cli/internal/errors"
	"github.com/thedavidweng/jobs-cli/internal/httpclient"
	"github.com/thedavidweng/jobs-cli/internal/output"
	"github.com/thedavidweng/jobs-cli/internal/registry"
	"github.com/thedavidweng/jobs-cli/internal/safety"
)

type Options struct {
	Stdout          io.Writer
	Stderr          io.Writer
	Stdin           io.Reader
	ConfigPath      string
	ConfigDir       string
	BaseTransport   http.RoundTripper
	RegistryFactory func(cfg *config.Config, client *http.Client) *registry.Registry
}

type result struct {
	Data       any
	Warnings   []string
	Pagination *output.PaginationMeta
	Partitions []output.PartitionMeta
}

type App struct {
	opts   *Options
	out    io.Writer
	errOut io.Writer
	in     io.Reader

	configPath string
	jsonMode   bool
	pretty     bool
	full       bool
	readOnly   bool
	dryRun     bool
	confirm    bool
	timeout    time.Duration
	profile    string

	requestID string
	command   string
	start     time.Time

	cfg    *config.Config
	cfgErr error
	reg    *registry.Registry
	root   *cobra.Command
	cancel context.CancelFunc
}

func New(opts *Options) *App {
	a := &App{opts: opts}
	a.out = opts.Stdout
	if a.out == nil {
		a.out = os.Stdout
	}
	a.errOut = opts.Stderr
	if a.errOut == nil {
		a.errOut = os.Stderr
	}
	a.in = opts.Stdin
	if a.in == nil {
		a.in = os.Stdin
	}
	a.profile = "default"
	a.timeout = 30 * time.Second
	a.root = a.newRoot()
	return a
}

func Execute() {
	os.Exit(New(&Options{}).Run(os.Args[1:]))
}

func (a *App) Run(args []string) int {
	a.root.SetArgs(args)
	if target, _, err := a.root.Find(args); err == nil && target != nil {
		a.command = commandID(target)
	}
	err := a.root.Execute()
	a.stopContext()
	if err == nil {
		return 0
	}
	e := joberrors.From(err)
	if e == nil {
		return 0
	}
	if a.jsonMode {
		a.renderer().RenderError(output.NewErrorEnvelope(a.command, a.profile, output.SchemaVersion, a.requestID, e, time.Since(a.start)))
	} else {
		fmt.Fprintln(a.errOut, "Error:", err)
	}
	return e.ExitCode()
}

func (a *App) renderer() *output.Renderer {
	return output.NewRenderer(a.out, a.errOut, a.jsonMode, a.pretty)
}

func (a *App) emit(r result) error {
	warnings := r.Warnings
	for _, warning := range warnings {
		a.renderer().PrintDiagnostic("Warning: " + warning)
	}
	if a.jsonMode {
		env := output.NewEnvelope(a.command, a.profile, output.SchemaVersion, a.requestID, r.Data, time.Since(a.start))
		env.Meta.Warnings = warnings
		env.Meta.Pagination = r.Pagination
		env.Meta.Partitions = r.Partitions
		a.renderer().RenderSuccess(env)
	}
	return nil
}

func (a *App) config() *config.Config {
	if a.cfg == nil {
		cfg, err := config.Load(a.defaultConfigPath())
		a.cfg = cfg
		if a.cfgErr == nil {
			a.cfgErr = err
		}
	}
	return a.cfg
}

func (a *App) registry() *registry.Registry {
	if a.reg == nil {
		cfg, client := a.config(), a.httpClient()
		if a.opts.RegistryFactory != nil {
			a.reg = a.opts.RegistryFactory(cfg, client)
		} else {
			a.reg = registry.New(client, cfg)
		}
	}
	return a.reg
}

func (a *App) httpClient() *http.Client {
	base := a.opts.BaseTransport
	if base == nil {
		base = http.DefaultTransport
	}
	client := httpclient.New(base, httpclient.DefaultPolicy())
	if a.timeout > 0 {
		client.Timeout = a.timeout
	}
	return client
}

func (a *App) defaultConfigPath() string {
	if a.configPath != "" {
		return a.configPath
	}
	if a.opts.ConfigPath != "" {
		return a.opts.ConfigPath
	}
	if a.opts.ConfigDir != "" {
		return filepath.Join(a.opts.ConfigDir, "config.yaml")
	}
	if v := os.Getenv("JOBS_CONFIG"); v != "" {
		return v
	}
	return config.DefaultConfigPath()
}

func (a *App) prepare(cmd *cobra.Command) error {
	a.requestID = uuid.NewString()
	a.start = time.Now()
	a.command = commandID(cmd)

	a.jsonMode = a.jsonMode || envBool("JOBS_JSON")
	a.pretty = a.pretty || envBool("JOBS_PRETTY")
	a.full = a.full || envBool("JOBS_FULL")
	a.readOnly = a.readOnly || envBool("JOBS_READ_ONLY")
	a.dryRun = a.dryRun || envBool("JOBS_DRY_RUN")
	a.confirm = a.confirm || envBool("JOBS_CONFIRM")

	cfg, err := config.Load(a.defaultConfigPath())
	a.cfg = cfg
	a.cfgErr = err
	if err != nil {
		a.renderer().PrintDiagnostic("Warning: " + err.Error())
	}
	if !flagChanged(cmd, "profile") {
		a.profile = cfg.ProfileName
	}
	if !flagChanged(cmd, "timeout") && cfg.Active != nil && cfg.Active.Timeout > 0 {
		a.timeout = cfg.Active.Timeout
	}
	if !flagChanged(cmd, "read-only") && cfg.Active != nil && cfg.Active.ReadOnly {
		a.readOnly = true
	}

	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	if a.timeout > 0 {
		ctx, a.cancel = context.WithTimeout(ctx, a.timeout)
	}
	cmd.SetContext(ctx)
	return nil
}

func (a *App) stopContext() {
	if a.cancel != nil {
		a.cancel()
		a.cancel = nil
	}
}

func (a *App) gate() safety.Gate {
	return safety.Gate{ReadOnly: a.readOnly, DryRun: a.dryRun, Confirm: a.confirm}
}

func (a *App) readInput(path string) ([]byte, *joberrors.Error) {
	if path == "" {
		return nil, invalid("a file path is required")
	}
	if path == "-" {
		data, err := io.ReadAll(a.in)
		if err != nil {
			return nil, joberrors.New(joberrors.InvalidArguments, fmt.Sprintf("read stdin: %v", err), joberrors.CatValidation, false, err)
		}
		return data, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, joberrors.New(joberrors.InvalidArguments, fmt.Sprintf("read %s: %v", path, err), joberrors.CatValidation, false, err)
	}
	return data, nil
}

func invalid(message string) *joberrors.Error {
	return joberrors.New(joberrors.InvalidArguments, message, joberrors.CatValidation, false, nil)
}

func envBool(key string) bool {
	return config.ParseBool(os.Getenv(key))
}

func flagChanged(cmd *cobra.Command, name string) bool {
	f := cmd.Root().PersistentFlags().Lookup(name)
	return f != nil && f.Changed
}

func commandID(cmd *cobra.Command) string {
	if cmd == nil {
		return ""
	}
	path := cmd.CommandPath()
	fields := strings.Fields(path)
	if len(fields) <= 1 {
		return path
	}
	return strings.Join(fields[1:], ".")
}

func firstError(errs []error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return fmt.Errorf("no sources succeeded")
}
