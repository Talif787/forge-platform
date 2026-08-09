package api

import "github.com/forge-platform/forge/internal/modules/provisioning/app"

type WorkflowResponse struct {
	WorkflowID    string `json:"workflowId"`
	RunID         string `json:"runId"`
	Type          string `json:"type"`
	Status        string `json:"status"`
	StartTime     string `json:"startTime,omitempty"`
	CloseTime     string `json:"closeTime,omitempty"`
	HistoryLength int64  `json:"historyLength"`
}

func toResponse(v app.WorkflowView) WorkflowResponse {
	return WorkflowResponse{
		WorkflowID:    v.WorkflowID,
		RunID:         v.RunID,
		Type:          v.Type,
		Status:        v.Status,
		StartTime:     v.StartTime,
		CloseTime:     v.CloseTime,
		HistoryLength: v.HistoryLength,
	}
}
