package domain

import "github.com/thedavidweng/jobs-cli/internal/errors"

type PaginationKind string

const (
	PaginationCursor     PaginationKind = "cursor"
	PaginationOffset     PaginationKind = "offset"
	PaginationStartCount PaginationKind = "start_count"
)

type NativePagination struct {
	Kind   PaginationKind `json:"kind"`
	Cursor string         `json:"cursor,omitempty"`
	Offset int            `json:"offset,omitempty"`
	Start  int            `json:"start,omitempty"`
	Count  int            `json:"count,omitempty"`
}

type Pagination struct {
	Limit      int               `json:"limit,omitempty"`
	Offset     int               `json:"offset,omitempty"`
	Total      int               `json:"total,omitempty"`
	HasMore    bool              `json:"has_more"`
	NextCursor string            `json:"next_cursor,omitempty"`
	Native     *NativePagination `json:"native,omitempty"`
}

type SearchRequest struct {
	Keywords      string
	Location      string
	Radius        int
	Remote        *bool
	Sort          string
	Limit         int
	Offset        int
	Cursor        string
	Authenticated bool
	Country       string
	Locale        string
}

type SearchPartition struct {
	Source     Source        `json:"source"`
	Jobs       []Job         `json:"jobs"`
	Pagination *Pagination   `json:"pagination,omitempty"`
	Error      *errors.Error `json:"error,omitempty"`
}

type SearchResult struct {
	Partitions []SearchPartition `json:"partitions"`
}

type DetailRequest struct {
	SourceJobID   string
	Authenticated bool
	Country       string
	Locale        string
}
