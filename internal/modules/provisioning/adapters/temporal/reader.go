package temporal

import (
	"context"
	"errors"
	"strings"
	"time"

	"go.temporal.io/api/serviceerror"
	workflowpb "go.temporal.io/api/workflow/v1"
	"go.temporal.io/api/workflowservice/v1"
	"go.temporal.io/sdk/client"

	"github.com/forge-platform/forge/internal/modules/provisioning/app"
)

const provisionWorkflowType = "ProvisionTenantWorkflow"

// Reader reads provisioning workflow executions through Temporal's visibility
// API. Any failure to reach Temporal is reported as unavailable (503), so the
// rest of the control plane is unaffected.
type Reader struct {
	c         client.Client
	namespace string
}

func NewReader(c client.Client, namespace string) *Reader {
	return &Reader{c: c, namespace: namespace}
}

func (r *Reader) List(ctx context.Context) ([]app.WorkflowView, error) {
	resp, err := r.c.ListWorkflow(ctx, &workflowservice.ListWorkflowExecutionsRequest{
		Namespace: r.namespace,
		PageSize:  100,
	})
	if err != nil {
		return nil, app.ErrDisabled
	}
	views := make([]app.WorkflowView, 0, len(resp.GetExecutions()))
	for _, e := range resp.GetExecutions() {
		if e.GetType().GetName() != provisionWorkflowType {
			continue
		}
		views = append(views, toView(e))
	}
	return views, nil
}

func (r *Reader) Get(ctx context.Context, workflowID string) (app.WorkflowView, error) {
	resp, err := r.c.DescribeWorkflowExecution(ctx, workflowID, "")
	if err != nil {
		var notFound *serviceerror.NotFound
		if errors.As(err, &notFound) {
			return app.WorkflowView{}, app.ErrNotFound
		}
		return app.WorkflowView{}, app.ErrDisabled
	}
	return toView(resp.GetWorkflowExecutionInfo()), nil
}

func toView(e *workflowpb.WorkflowExecutionInfo) app.WorkflowView {
	v := app.WorkflowView{
		WorkflowID:    e.GetExecution().GetWorkflowId(),
		RunID:         e.GetExecution().GetRunId(),
		Type:          e.GetType().GetName(),
		Status:        strings.TrimPrefix(e.GetStatus().String(), "WORKFLOW_EXECUTION_STATUS_"),
		HistoryLength: e.GetHistoryLength(),
	}
	if t := e.GetStartTime(); t != nil {
		v.StartTime = t.AsTime().UTC().Format(time.RFC3339)
	}
	if t := e.GetCloseTime(); t != nil {
		v.CloseTime = t.AsTime().UTC().Format(time.RFC3339)
	}
	return v
}
