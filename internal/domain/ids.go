package domain

import (
	"fmt"
	"strings"
)

type Source string

const (
	SourceIndeed   Source = "indeed"
	SourceLinkedIn Source = "linkedin"
)

func (s Source) Valid() bool {
	switch s {
	case SourceIndeed, SourceLinkedIn:
		return true
	default:
		return false
	}
}

// MarketScoped reports whether the Source searches one country's index, so a
// search must resolve a Market before calling it.
func (s Source) MarketScoped() bool {
	return s == SourceIndeed
}

func FormatJobID(source Source, sourceJobID string) string {
	return string(source) + ":" + sourceJobID
}

func ParseJobID(id string) (Source, string, error) {
	source, sourceJobID, found := strings.Cut(strings.TrimSpace(id), ":")
	if !found || source == "" || sourceJobID == "" {
		return "", "", fmt.Errorf("invalid job id %q: expected the form <source>:<source-job-id>", id)
	}
	if strings.Contains(sourceJobID, ":") {
		return "", "", fmt.Errorf("invalid job id %q: source job id must not contain ':'", id)
	}
	return Source(source), sourceJobID, nil
}

func LooksLikeJobID(id string) bool {
	if strings.Contains(id, "://") {
		return false
	}
	source, sourceJobID, found := strings.Cut(strings.TrimSpace(id), ":")
	return found && source != "" && sourceJobID != "" && !strings.Contains(sourceJobID, ":")
}
