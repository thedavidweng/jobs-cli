package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/thedavidweng/jobs-cli/internal/domain"
)

type entryStatus struct {
	Name            string `json:"name"`
	Mode            string `json:"mode,omitempty"`
	Kind            string `json:"kind"`
	Role            string `json:"role"`
	AuthRequired    bool   `json:"auth_required"`
	BrowserRequired bool   `json:"browser_required"`
	Inspect         bool   `json:"inspect"`
	Prepare         bool   `json:"prepare"`
	NativeSubmit    bool   `json:"native_submit"`
	Verification    string `json:"verification"`
	Notes           string `json:"notes,omitempty"`
}

type sourcesStatusReport struct {
	Sources   []entryStatus `json:"sources"`
	Providers []entryStatus `json:"providers"`
}

func sourcesCmd(a *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "sources",
		GroupID: "access",
		Short:   "Inspect discovery sources and application providers",
	}
	cmd.AddCommand(sourcesStatusCmd(a))
	return cmd
}

func sourcesStatusCmd(a *App) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Report sources and application providers with capabilities and verification posture",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			report := sourcesStatusReport{
				Sources:   discoverySources(),
				Providers: applicationProviders(),
			}
			if !a.jsonMode {
				a.printSourceStatus(report)
			}
			return a.emit(result{Data: report})
		},
	}
}

func discoverySources() []entryStatus {
	return []entryStatus{
		{
			Name: "indeed", Mode: "mobile-graphql", Kind: "discovery_source", Role: "discovery",
			Verification: string(domain.VerifiedWorking),
			Notes:        "structured mobile GraphQL discovery; external jobs resolve to their provider; Indeed Apply is browser-required; no login required for discovery",
		},
		{
			Name: "linkedin", Mode: "guest", Kind: "discovery_source", Role: "discovery",
			Verification: string(domain.VerifiedWorking),
			Notes:        "anonymous Guest discovery; the default for --source linkedin",
		},
		{
			Name: "linkedin", Mode: "authenticated", Kind: "discovery_source", Role: "discovery",
			AuthRequired: true,
			Verification: string(domain.VerifiedSourceImpl),
			Notes:        "authenticated Voyager search/detail, used only with --authenticated; never a silent upgrade",
		},
	}
}

func applicationProviders() []entryStatus {
	return []entryStatus{
		providerEntry(domain.ProviderGreenhouse, "native", string(domain.VerifiedWorking),
			"public Board API; first native end-to-end application path"),
		providerEntry(domain.ProviderLinkedIn, "native", string(domain.VerifiedSourceImpl),
			"Easy Apply inspect is available when LinkedIn-native apply is offered; non-Easy-Apply jobs are BROWSER_REQUIRED; submit stays disabled until live verification (LINKEDIN_EASY_APPLY_UNVERIFIED)"),
		providerEntry(domain.ProviderIndeed, "browser", string(domain.BrowserRequiredPosture),
			"the browser flow may require an authenticated Indeed session; jobs-cli does not manage it"),
		providerEntry(domain.ProviderLever, "browser", string(domain.VerifiedWorking),
			"public Postings API for read; hCaptcha blocks native candidate submission"),
		providerEntry(domain.ProviderAshby, "browser", string(domain.VerifiedWorking),
			"public job-board API read with compensation; submission requires employer credentials"),
		providerEntry(domain.ProviderWorkday, "browser", string(domain.VerifiedSourceImpl),
			"CXS JSON read; candidate account, verification, and wizard stay in the browser"),
		providerEntry(domain.ProviderSmartRecruiters, "browser", string(domain.VerifiedWorking),
			"public company postings read; candidate submission stays in the browser"),
		providerEntry(domain.ProviderICIMS, "browser", string(domain.PartiallyVerified),
			"public Schema.org/JSON-LD detail where available; native submission is not claimed"),
		providerEntry(domain.ProviderExternal, "browser", string(domain.BrowserRequiredPosture),
			"unrecognized employer site; return the URL for an external browser-capable Agent"),
		providerEntry(domain.ProviderUnknown, "browser", string(domain.BrowserRequiredPosture),
			"provider could not be determined"),
	}
}

// providerEntry builds an Application Provider row from the canonical domain table.
func providerEntry(provider domain.ApplicationProvider, role, verification, notes string) entryStatus {
	capabilities := domain.CapabilitiesFor(provider)
	return entryStatus{
		Name:            string(provider),
		Kind:            "application_provider",
		Role:            role,
		AuthRequired:    capabilities.AuthRequired,
		BrowserRequired: capabilities.BrowserRequired,
		Inspect:         capabilities.Inspect,
		Prepare:         capabilities.Prepare,
		NativeSubmit:    capabilities.NativeSubmit,
		Verification:    verification,
		Notes:           notes,
	}
}

func (a *App) printSourceStatus(report sourcesStatusReport) {
	fmt.Fprintln(a.out, "DISCOVERY SOURCES")
	fmt.Fprintf(a.out, "  %-12s %-16s %-6s %-8s %-32s %s\n", "NAME", "MODE", "AUTH", "BROWSER", "VERIFICATION", "NOTES")
	for _, entry := range report.Sources {
		fmt.Fprintf(a.out, "  %-12s %-16s %-6t %-8t %-32s %s\n", entry.Name, entry.Mode, entry.AuthRequired, entry.BrowserRequired, entry.Verification, entry.Notes)
	}
	fmt.Fprintln(a.out)
	fmt.Fprintln(a.out, "APPLICATION PROVIDERS")
	fmt.Fprintf(a.out, "  %-16s %-6s %-8s %-8s %-8s %-8s %-32s %s\n", "NAME", "AUTH", "BROWSER", "INSPECT", "PREPARE", "SUBMIT", "VERIFICATION", "NOTES")
	for _, entry := range report.Providers {
		fmt.Fprintf(a.out, "  %-16s %-6t %-8t %-8t %-8t %-8t %-32s %s\n",
			entry.Name, entry.AuthRequired, entry.BrowserRequired, entry.Inspect, entry.Prepare, entry.NativeSubmit, entry.Verification, entry.Notes)
	}
}
