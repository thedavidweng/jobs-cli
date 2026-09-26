package e2e

import (
	"bytes"
	"net/http"
	"strings"
	"testing"

	"github.com/thedavidweng/jobs-cli/internal/cli"
	"github.com/thedavidweng/jobs-cli/internal/config"
	"github.com/thedavidweng/jobs-cli/internal/domain"
	"github.com/thedavidweng/jobs-cli/internal/lever"
	"github.com/thedavidweng/jobs-cli/internal/registry"
	"github.com/thedavidweng/jobs-cli/internal/testutil"
)

func TestLeverTargetThroughApplyInspectCommand(t *testing.T) {
	target := domain.ApplicationTarget{URL: "https://jobs.lever.co/acme/a/apply", Provider: domain.ProviderLever, Capabilities: domain.Capabilities{BrowserRequired: true}}
	job := domain.NewJob(domain.SourceIndeed, "source-a")
	job.Title = "Engineer"
	job.Application = &target
	reg := testutil.NewRegistry(map[domain.Source]domain.SourceAdapter{domain.SourceIndeed: &testutil.FakeSource{SourceName: domain.SourceIndeed, Job: &job}}, &testutil.FakeResolver{}, map[domain.ApplicationProvider]domain.ApplyProvider{domain.ProviderLever: lever.NewProvider(http.DefaultClient)})
	var stdout, stderr bytes.Buffer
	app := cli.New(&cli.Options{Stdout: &stdout, Stderr: &stderr, Stdin: strings.NewReader(""), ConfigDir: t.TempDir(), RegistryFactory: func(*config.Config, *http.Client) *registry.Registry { return reg }})
	if code := app.Run([]string{"--json", "apply", "inspect", "indeed:source-a"}); code != 6 {
		t.Fatalf("exit=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	doc := decodeOne(t, stdout.String())
	if doc.Error == nil || doc.Error.Code != "BROWSER_REQUIRED" {
		t.Fatalf("envelope=%+v", doc)
	}
}
