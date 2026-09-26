package cli

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/spf13/cobra"

	"github.com/thedavidweng/jobs-cli/internal/domain"
	joberrors "github.com/thedavidweng/jobs-cli/internal/errors"
	"github.com/thedavidweng/jobs-cli/internal/output"
)

type sourceRegistry interface {
	Source(name domain.Source, authenticated bool) (domain.SourceAdapter, *joberrors.Error)
}

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
}

func searchCmd(a *App) *cobra.Command {
	f := &searchFlags{}
	cmd := &cobra.Command{
		Use:     "search",
		GroupID: "jobs",
		Short:   "Search discovery sources and return results partitioned by Source",
		Long: `Search discovery sources. Results are partitioned by Source; jobs-cli never
merges or deduplicates across sources. Default sources are indeed and linkedin (Guest).

Continuation is per source: pass exactly one --source plus that source's native
--cursor or --offset. There is no cross-source page token.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.runSearch(cmd, f)
		},
	}
	cmd.Flags().StringVarP(&f.query, "query", "q", "", "search keywords")
	cmd.Flags().StringVarP(&f.location, "location", "l", "", "location filter")
	cmd.Flags().IntVar(&f.radius, "radius", 0, "search radius in source-native units")
	cmd.Flags().BoolVar(&f.remote, "remote", false, "restrict to remote roles where the source supports it")
	cmd.Flags().StringVar(&f.sort, "sort", "", "sort order (source-native)")
	cmd.Flags().IntVar(&f.limit, "limit", 25, "maximum results per source")
	cmd.Flags().IntVar(&f.offset, "offset", 0, "native offset for single-source continuation")
	cmd.Flags().StringVar(&f.cursor, "cursor", "", "native cursor for single-source continuation")
	cmd.Flags().StringSliceVar(&f.sources, "source", nil, "discovery source (repeatable; v1: indeed, linkedin)")
	cmd.Flags().BoolVar(&f.authenticated, "authenticated", false, "use authenticated LinkedIn (Voyager) instead of Guest")
	return cmd
}

func (a *App) runSearch(cmd *cobra.Command, f *searchFlags) error {
	ctx := cmd.Context()

	if f.query == "" && f.location == "" {
		return invalid("provide --query and/or --location")
	}

	sources, err := a.resolveSearchSources(cmd, f)
	if err != nil {
		return err
	}

	reg := a.registry()
	results := make([]partResult, len(sources))
	var wg sync.WaitGroup
	for i, name := range sources {
		wg.Add(1)
		go func(index int, source domain.Source) {
			defer wg.Done()
			results[index] = searchOne(ctx, reg, source, f)
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

func searchOne(ctx context.Context, reg sourceRegistry, source domain.Source, f *searchFlags) partResult {
	adapter, aerr := reg.Source(source, f.authenticated)
	if aerr != nil {
		return partResult{
			partition: domain.SearchPartition{Source: source, Jobs: []domain.Job{}, Error: aerr},
			err:       aerr,
		}
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
	partition, err := adapter.Search(ctx, req)
	if err != nil {
		e := joberrors.From(err)
		return partResult{
			partition: domain.SearchPartition{Source: source, Jobs: []domain.Job{}, Error: e},
			err:       e,
		}
	}
	if partition == nil {
		partition = &domain.SearchPartition{Source: source}
	}
	if partition.Source == "" {
		partition.Source = source
	}
	if partition.Jobs == nil {
		partition.Jobs = []domain.Job{}
	}
	return partResult{partition: *partition}
}

func remoteFilter(f *searchFlags) *bool {
	if !f.remote {
		return nil
	}
	value := true
	return &value
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
			return nil, invalid(fmt.Sprintf("unsupported --source %q; v1 supports: indeed, linkedin", name))
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
		fmt.Fprintf(a.out, "== %s (%d jobs) ==\n", partition.Source, len(partition.Jobs))
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
