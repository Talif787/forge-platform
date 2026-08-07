package httpx

import (
	"encoding/json"
	"net/http"

	"github.com/forge-platform/forge/internal/platform/apperr"
	"github.com/forge-platform/forge/internal/platform/log"
)

type ErrorBody struct {
	Code          string   `json:"code"`
	Message       string   `json:"message"`
	Details       []string `json:"details,omitempty"`
	CorrelationID string   `json:"correlationId"`
}

type Page[T any] struct {
	Items      []T    `json:"items"`
	NextCursor string `json:"nextCursor,omitempty"`
}

func JSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if body != nil {
		_ = json.NewEncoder(w).Encode(body)
	}
}

func Error(w http.ResponseWriter, r *http.Request, err error) {
	status := apperr.HTTPStatus(err)
	body := ErrorBody{Code: "INTERNAL", Message: "an unexpected error occurred", CorrelationID: CorrelationID(r.Context())}
	if e, ok := apperr.As(err); ok {
		body.Code = e.Code
		body.Message = e.Message
		body.Details = e.Details
	}
	if status >= 500 {
		log.From(r.Context()).Error("request failed", "error", err.Error(), "code", body.Code)
	}
	JSON(w, status, body)
}

func Decode(w http.ResponseWriter, r *http.Request, dst any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		Error(w, r, apperr.Validation("INVALID_BODY", "request body is not valid JSON: "+err.Error()))
		return false
	}
	return true
}
