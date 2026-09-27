package safety

import (
	"fmt"

	"github.com/thedavidweng/jobs-cli/v2/internal/errors"
)

type OperationTier string

const (
	TierRead     OperationTier = "read"
	TierMutation OperationTier = "mutation"
)

type Gate struct {
	ReadOnly bool
	DryRun   bool
	Confirm  bool
}

func ReadOnlyError() *errors.Error {
	return errors.New(errors.ReadOnlyViolation, "remote writes are blocked in read-only mode", errors.CatSafety, false, nil)
}

func (g Gate) Check(tier OperationTier) *errors.Error {
	if tier == TierRead {
		return nil
	}
	if g.ReadOnly {
		return ReadOnlyError()
	}
	if g.DryRun {
		return nil
	}
	if !g.Confirm {
		return errors.New(errors.ConfirmationRequired, fmt.Sprintf("this %s operation requires --confirm to execute", tier), errors.CatSafety, false, nil)
	}
	return nil
}

type Mutation struct {
	Action     string         `json:"action"`
	Target     string         `json:"target,omitempty"`
	Provider   string         `json:"provider,omitempty"`
	ResourceID string         `json:"resource_id,omitempty"`
	Details    map[string]any `json:"details,omitempty"`
}

type Plan struct {
	Command          string     `json:"command"`
	DryRun           bool       `json:"dry_run"`
	PlannedMutations []Mutation `json:"planned_mutations"`
	Notes            []string   `json:"notes,omitempty"`
}
