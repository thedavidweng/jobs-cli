package cli

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/spf13/cobra"

	"github.com/thedavidweng/jobs-cli/v2/internal/domain"
	joberrors "github.com/thedavidweng/jobs-cli/v2/internal/errors"
	"github.com/thedavidweng/jobs-cli/v2/internal/market"
	"github.com/thedavidweng/jobs-cli/v2/internal/output"
	"github.com/thedavidweng/jobs-cli/v2/internal/registry"
)

type searchFlags struct {
	query         string
	location      string
	radius        int
	remote        bool
	sort          string
	limit         int
	offset        int
	cursor        string
	sources       []string
	authenticated bool
	country       string
	locale        string
}

func searchCmd(a *App) *cobra.Command {
	f := &searchFlags{}
	cmd := &cobra.Command{
		Use:     "search",
		GroupID: "jobs",
		Short:   "Search discovery sources and return results partitioned by Source",
		Long: `Search discovery sources. Results are partitioned by Source; jobs-cli never
merges or deduplicates across sources. Default sources are indeed and linkedin (Guest).
--authenticated switches the linkedin partition to authenticated Voyager; sources
without an authenticated variant keep their single implementation.

YZi, CivicInfo BC, and TransLink can list their boards without keywords. Use
--source yzi, civicinfo, or translink; their limit is 1–100. Location matching
is case-insensitive text matching. These boards do not support --radius.

Indeed searches one market (country) and has no default. The market comes from
--country, else from the end of --location (a US state, Canadian province, or
country name: "Austin, TX", "Toronto, ON", "London, United Kingdom"), else
JOBS_COUNTRY, else the profile country. Two-letter endings other than US and UK
are read as US states or Canadian provinces, so "San Francisco, CA" is
California. Without a market the indeed partition fails with MARKET_REQUIRED
and other sources still run.

Continuation is per source: pass exactly one --source plus that source's native
--cursor or --offset. There is no cross-source page token.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.runSearch(cmd, f)
		},
	}
	cmd.Flags().StringVarP(&f.query, "query", "q", "", "search keywords")
	cmd.Flags().StringVarP(&f.location, "location", "l", "", "location filter (with --authenticated, LinkedIn needs a geo URN like urn:li:fsd_geo:<id> or a known location)")
	cmd.Flags().IntVar(&f.radius, "radius", 0, "search radius in source-native units")
	cmd.Flags().BoolVar(&f.remote, "remote", false, "restrict to remote roles where the source supports it")
	cmd.Flags().StringVar(&f.sort, "sort", "", "sort order (source-native)")
	cmd.Flags().IntVar(&f.limit, "limit", 25, "maximum results per source")
	cmd.Flags().IntVar(&f.offset, "offset", 0, "native offset for single-source continuation")
	cmd.Flags().StringVar(&f.cursor, "cursor", "", "native cursor for single-source continuation")
	cmd.Flags().StringSliceVar(&f.sources, "source", nil, "discovery source (repeatable; indeed, linkedin, yzi, civicinfo, translink)")
	cmd.Flags().BoolVar(&f.authenticated, "authenticated", false, "use authenticated LinkedIn (Voyager) instead of Guest (LinkedIn only)")
	cmd.Flags().StringVar(&f.country, "country", "", "Indeed market as an ISO country code, e.g. US, CA, GB (overrides --location inference, JOBS_COUNTRY, and the profile)")
	cmd.Flags().StringVar(&f.locale, "locale", "", "Indeed locale as language-REGION, e.g. fr-CA (default: JOBS_LOCALE or the profile locale when its region is the market, else en-<country>)")
	return cmd
}

func (a *App) runSearch(cmd *cobra.Command, f *searchFlags) error {
	ctx := cmd.Context()

	sources, err := a.resolveSearchSources(cmd, f)
	if err != nil {
		return err
	}
	if f.query == "" && f.location == "" {
		for _, source := range sources {
			if source == domain.SourceIndeed || source == domain.SourceLinkedIn {
				return invalid("provide --query and/or --location")
			}
		}
	}

	mkt, marketErr := a.searchMarket(f)

	reg := a.registry()
	results := make([]partResult, len(sources))
	var wg sync.WaitGroup
	for i, name := range sources {
		wg.Add(1)
		go func(index int, source domain.Source) {
			defer wg.Done()
			results[index] = searchOne(ctx, reg, source, f, mkt, marketErr)
		}(i, name)
	}
	wg.Wait()

	partitions := make([]domain.SearchPartition, 0, len(sources))
	metaParts := make([]output.PartitionMeta, 0, len(sources))
	var warnings []string
	var failures []error
	for i, name := range sources {
		entry := results[i]
		partitions = append(partitions, entry.partition)
		meta := output.PartitionMeta{Source: string(name), OK: entry.err == nil}
		if entry.partition.Pagination != nil {
			meta.Pagination = paginationMeta(entry.partition.Pagination)
		}
		metaParts = append(metaParts, meta)
		if entry.err != nil {
			failures = append(failures, entry.err)
			warnings = append(warnings, fmt.Sprintf("%s: %s", name, entry.err.Error()))
		}
	}

	if len(failures) == len(sources) {
		return firstError(failures)
	}

	if !a.full {
		for i := range partitions {
			for j := range partitions[i].Jobs {
				partitions[i].Jobs[j].Diagnostics = nil
			}
		}
	}

	data := domain.SearchResult{Partitions: partitions}
	if !a.jsonMode {
		a.printSearch(data)
	}
	return a.emit(result{Data: data, Warnings: warnings, Partitions: metaParts})
}

type partResult struct {
	partition domain.SearchPartition
	err       error
}

// searchOne runs one Source. A market-scoped Source searches the resolved
// market, or fails its own partition when none resolved; other Sources never
// see a market.
func searchOne(ctx context.Context, reg *registry.Registry, source domain.Source, f *searchFlags, mkt *domain.Market, marketErr *joberrors.Error) partResult {
	adapter, aerr := reg.Source(source, f.authenticated)
	if aerr != nil {
		return failedPartition(source, nil, aerr)
	}
	req := &domain.SearchRequest{
		Keywords:      f.query,
		Location:      f.location,
		Radius:        f.radius,
		Remote:        remoteFilter(f),
		Sort:          f.sort,
		Limit:         f.limit,
		Offset:        f.offset,
		Cursor:        f.cursor,
		Authenticated: f.authenticated,
	}
	if source.MarketScoped() {
		if marketErr != nil {
			return failedPartition(source, nil, marketErr)
		}
		req.Market = mkt
	}
	partition, err := adapter.Search(ctx, req)
	if err != nil {
		return failedPartition(source, req.Market, joberrors.From(err))
	}
	out := domain.SearchPartition{Source: source}
	if partition != nil {
		out = *partition
	}
	if out.Source == "" {
		out.Source = source
	}
	if out.Jobs == nil {
		out.Jobs = []domain.Job{}
	}
	out.Market = req.Market
	return partResult{partition: out}
}

func failedPartition(source domain.Source, mkt *domain.Market, err *joberrors.Error) partResult {
	return partResult{
		partition: domain.SearchPartition{Source: source, Market: mkt, Jobs: []domain.Job{}, Error: err},
		err:       err,
	}
}

func remoteFilter(f *searchFlags) *bool {
	if !f.remote {
		return nil
	}
	value := true
	return &value
}

func (a *App) searchMarket(f *searchFlags) (*domain.Market, *joberrors.Error) {
	configured := a.config().Market()
	return market.Resolve(&market.Inputs{
		Country:        f.country,
		Locale:         f.locale,
		Location:       f.location,
		EnvCountry:     configured.EnvCountry,
		EnvLocale:      configured.EnvLocale,
		ProfileCountry: configured.ProfileCountry,
		ProfileLocale:  configured.ProfileLocale,
	})
}

func (a *App) resolveSearchSources(cmd *cobra.Command, f *searchFlags) ([]domain.Source, error) {
	requested := f.sources
	if len(requested) == 0 {
		requested = a.config().Sources()
	}
	sources := make([]domain.Source, 0, len(requested))
	for _, name := range requested {
		source := domain.Source(strings.ToLower(strings.TrimSpace(name)))
		if !source.Valid() {
			return nil, invalid(fmt.Sprintf("unsupported --source %q; supports: indeed, linkedin, yzi, civicinfo, translink", name))
		}
		sources = append(sources, source)
	}
	if len(sources) == 0 {
		return nil, invalid("no discovery sources requested")
	}
	continuing := f.offset > 0 || f.cursor != ""
	if continuing {
		if !cmd.Flags().Changed("source") || len(sources) != 1 {
			return nil, invalid("continuation requires exactly one --source plus that source's native --cursor or --offset")
		}
	}
	return sources, nil
}

func paginationMeta(p *domain.Pagination) *output.PaginationMeta {
	if p == nil {
		return nil
	}
	return &output.PaginationMeta{
		Limit:      p.Limit,
		Offset:     p.Offset,
		Total:      p.Total,
		HasMore:    p.HasMore,
		NextCursor: p.NextCursor,
	}
}

func (a *App) printSearch(data domain.SearchResult) {
	for i := range data.Partitions {
		partition := &data.Partitions[i]
		if partition.Error != nil {
			fmt.Fprintf(a.out, "== %s: failed (%s) ==\n", partition.Source, partition.Error.Code)
			continue
		}
		fmt.Fprintf(a.out, "== %s (%d jobs%s) ==\n", partition.Source, len(partition.Jobs), marketLabel(partition.Market))
		for j := range partition.Jobs {
			job := &partition.Jobs[j]
			fmt.Fprintf(a.out, "  %-28s %s\n", job.ID, job.Title)
			fmt.Fprintf(a.out, "  %-28s %s\n", "", job.Employer+"  "+job.Location)
		}
		if partition.Pagination != nil && partition.Pagination.HasMore {
			fmt.Fprintf(a.out, "  more available (cursor: %s)\n", partition.Pagination.NextCursor)
		}
	}
}

var marketOrigins = map[domain.MarketOrigin]string{
	domain.MarketFromFlag:     "--country",
	domain.MarketFromLocation: "--location",
	domain.MarketFromEnv:      "JOBS_COUNTRY",
	domain.MarketFromProfile:  "profile",
}

func marketLabel(m *domain.Market) string {
	if m == nil {
		return ""
	}
	return fmt.Sprintf(", market %s from %s", m.Country, marketOrigins[m.Origin])
}
