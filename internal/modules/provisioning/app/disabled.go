package app

import "context"

// DisabledReader is used when the Temporal client could not be constructed.
// Every call returns ErrDisabled, which the API maps to 503.
type DisabledReader struct{}

func (DisabledReader) List(context.Context) ([]WorkflowView, error) {
	return nil, ErrDisabled
}

func (DisabledReader) Get(context.Context, string) (WorkflowView, error) {
	return WorkflowView{}, ErrDisabled
}
