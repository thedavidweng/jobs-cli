package output

import (
	"time"

	"github.com/thedavidweng/jobs-cli/internal/errors"
)

const SchemaVersion = "2026-09-26"

type Envelope struct {
	OK   bool     `json:"ok"`
	Data any      `json:"data,omitempty"`
	Meta Metadata `json:"meta"`
}

type ErrorEnvelope struct {
	OK    bool          `json:"ok"`
	Error *errors.Error `json:"error"`
	Meta  Metadata      `json:"meta"`
}

type Metadata struct {
	Command       string          `json:"command"`
	Profile       string          `json:"profile,omitempty"`
	DurationMS    int64           `json:"duration_ms"`
	SchemaVersion string          `json:"schema_version"`
	RequestID     string          `json:"request_id,omitempty"`
	Warnings      []string        `json:"warnings,omitempty"`
	Pagination    *PaginationMeta `json:"pagination,omitempty"`
	Partitions    []PartitionMeta `json:"partitions,omitempty"`
}

type PaginationMeta struct {
	Limit      int    `json:"limit,omitempty"`
	Offset     int    `json:"offset,omitempty"`
	Total      int    `json:"total,omitempty"`
	HasMore    bool   `json:"has_more"`
	NextCursor string `json:"next_cursor,omitempty"`
}

type PartitionMeta struct {
	Source     string          `json:"source"`
	OK         bool            `json:"ok"`
	Pagination *PaginationMeta `json:"pagination,omitempty"`
}

func NewEnvelope(command, profile, schemaVersion, requestID string, data any, duration time.Duration) *Envelope {
	return &Envelope{
		OK:   true,
		Data: data,
		Meta: Metadata{
			Command:       command,
			Profile:       profile,
			DurationMS:    duration.Milliseconds(),
			SchemaVersion: schemaVersion,
			RequestID:     requestID,
		},
	}
}

func NewErrorEnvelope(command, profile, schemaVersion, requestID string, err *errors.Error, duration time.Duration) *ErrorEnvelope {
	return &ErrorEnvelope{
		OK:    false,
		Error: err,
		Meta: Metadata{
			Command:       command,
			Profile:       profile,
			DurationMS:    duration.Milliseconds(),
			SchemaVersion: schemaVersion,
			RequestID:     requestID,
		},
	}
}
