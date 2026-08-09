package app

import (
	"context"

	"github.com/forge-platform/forge/internal/platform/apperr"
)

// WorkflowView is the read model the API exposes for a provisioning workflow
// execution, drawn from Temporal's visibility records.
type WorkflowView struct {
	WorkflowID    string
	RunID         string
	Type          string
	Status        string
	StartTime     string
	CloseTime     string
	HistoryLength int64
}

// Reader lists and describes provisioning workflow executions.
type Reader interface {
	List(ctx context.Context) ([]WorkflowView, error)
	Get(ctx context.Context, workflowID string) (WorkflowView, error)
}

var (
	ErrNotFound = apperr.NotFound("WORKFLOW_NOT_FOUND", "provisioning workflow not found")
	ErrDisabled = apperr.Unavailable(
		"TEMPORAL_NOT_CONFIGURED",
		"the provisioning view is unavailable because Temporal is not reachable from this control plane",
	)
)
