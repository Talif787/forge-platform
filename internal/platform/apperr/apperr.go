package apperr

import (
	"errors"
	"fmt"
	"net/http"
)

type Kind int

const (
	KindInternal Kind = iota
	KindValidation
	KindNotFound
	KindConflict
	KindUnauthorized
	KindForbidden
	KindPreconditionFailed
	KindTooManyRequests
)

type Error struct {
	Kind    Kind
	Code    string
	Message string
	Details []string
	cause   error
}

func (e *Error) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.cause)
	}
	return e.Message
}

func (e *Error) Unwrap() error { return e.cause }

func (e *Error) WithCause(err error) *Error { e.cause = err; return e }
func (e *Error) WithDetails(d ...string) *Error {
	e.Details = append(e.Details, d...)
	return e
}

func New(kind Kind, code, message string) *Error {
	return &Error{Kind: kind, Code: code, Message: message}
}

func Validation(code, message string) *Error { return New(KindValidation, code, message) }
func NotFound(code, message string) *Error   { return New(KindNotFound, code, message) }
func Conflict(code, message string) *Error   { return New(KindConflict, code, message) }
func Unauthorized(code, message string) *Error {
	return New(KindUnauthorized, code, message)
}
func Forbidden(code, message string) *Error { return New(KindForbidden, code, message) }
func Precondition(code, message string) *Error {
	return New(KindPreconditionFailed, code, message)
}
func TooManyRequests(code, message string) *Error {
	return New(KindTooManyRequests, code, message)
}
func Internal(code, message string) *Error { return New(KindInternal, code, message) }

func HTTPStatus(err error) int {
	var e *Error
	if !errors.As(err, &e) {
		return http.StatusInternalServerError
	}
	switch e.Kind {
	case KindValidation:
		return http.StatusUnprocessableEntity
	case KindNotFound:
		return http.StatusNotFound
	case KindConflict:
		return http.StatusConflict
	case KindUnauthorized:
		return http.StatusUnauthorized
	case KindForbidden:
		return http.StatusForbidden
	case KindPreconditionFailed:
		return http.StatusPreconditionFailed
	case KindTooManyRequests:
		return http.StatusTooManyRequests
	default:
		return http.StatusInternalServerError
	}
}

func As(err error) (*Error, bool) {
	var e *Error
	if errors.As(err, &e) {
		return e, true
	}
	return nil, false
}
