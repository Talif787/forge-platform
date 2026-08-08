package app

import "context"

// DisabledReader is used when no Kubernetes cluster is configured. Every call
// returns ErrDisabled, which the API maps to 503, so the rest of the control
// plane runs normally without a cluster.
type DisabledReader struct{}

func (DisabledReader) List(context.Context, string) ([]ApplicationView, error) {
	return nil, ErrDisabled
}

func (DisabledReader) Get(context.Context, string, string) (ApplicationView, error) {
	return ApplicationView{}, ErrDisabled
}
