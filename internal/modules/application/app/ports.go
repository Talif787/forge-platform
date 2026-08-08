package app

import (
	"context"

	"github.com/forge-platform/forge/internal/platform/apperr"
)

// Condition mirrors a Kubernetes status condition, trimmed to what the portal
// needs.
type Condition struct {
	Type               string
	Status             string
	Reason             string
	Message            string
	ObservedGeneration int64
}

// ApplicationView is the read model the API exposes: declared intent plus the
// live reconcile status the operator writes back.
type ApplicationView struct {
	Namespace          string
	Name               string
	Image              string
	Port               int32
	DesiredReplicas    int32
	Tier               int32
	Expose             bool
	Phase              string
	ReadyReplicas      int32
	ObservedGeneration int64
	Conditions         []Condition
}

// Reader lists and gets Application resources. Implementations back this with a
// Kubernetes client (live) or a disabled stub (no cluster configured).
type Reader interface {
	List(ctx context.Context, namespace string) ([]ApplicationView, error)
	Get(ctx context.Context, namespace, name string) (ApplicationView, error)
}

var (
	ErrNotFound = apperr.NotFound("APPLICATION_NOT_FOUND", "application not found")
	ErrDisabled = apperr.Unavailable(
		"KUBERNETES_NOT_CONFIGURED",
		"the applications view is unavailable because no Kubernetes cluster is configured for this control plane",
	)
)
