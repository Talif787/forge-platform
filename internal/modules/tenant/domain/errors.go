package domain

import "github.com/forge-platform/forge/internal/platform/apperr"

var (
	ErrInvalidTenantID   = apperr.Validation("INVALID_TENANT_ID", "tenant id is not a valid identifier")
	ErrInvalidTenantName = apperr.Validation("INVALID_TENANT_NAME", "tenant name must be 1 to 100 characters")
	ErrInvalidSlug       = apperr.Validation("INVALID_SLUG", "slug must be a DNS-1123 label of 1 to 63 characters")
	ErrInvalidPlan       = apperr.Validation("INVALID_PLAN", "plan must be one of free, standard, enterprise")
	ErrInvalidStatus     = apperr.Validation("INVALID_STATUS", "status must be one of active, suspended, archived")
	ErrInvalidQuota      = apperr.Validation("INVALID_QUOTA", "max services must not be negative")

	ErrIllegalTransition = apperr.Precondition("ILLEGAL_STATUS_TRANSITION", "the requested status transition is not permitted")
	ErrArchived          = apperr.Precondition("TENANT_ARCHIVED", "an archived tenant cannot be modified")

	ErrTenantNotFound  = apperr.NotFound("TENANT_NOT_FOUND", "tenant not found")
	ErrSlugConflict    = apperr.Conflict("SLUG_CONFLICT", "a tenant with this slug already exists")
	ErrVersionConflict = apperr.Conflict("VERSION_CONFLICT", "the tenant was modified concurrently; retry with the current version")
)
