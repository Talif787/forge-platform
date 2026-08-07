package domain

import "github.com/forge-platform/forge/internal/platform/apperr"

var (
	ErrInvalidServiceID   = apperr.Validation("INVALID_SERVICE_ID", "service id is not a valid identifier")
	ErrInvalidTenantID    = apperr.Validation("INVALID_TENANT_ID", "tenant id is not a valid identifier")
	ErrInvalidServiceName = apperr.Validation("INVALID_SERVICE_NAME", "service name must be a DNS-1123 label of 1 to 63 characters")
	ErrInvalidTier        = apperr.Validation("INVALID_TIER", "tier must be an integer from 1 to 4")
	ErrInvalidLifecycle   = apperr.Validation("INVALID_LIFECYCLE", "lifecycle must be one of experimental, production, deprecated, retired")
	ErrInvalidOwnership   = apperr.Validation("INVALID_OWNERSHIP", "owning team is required")

	ErrIllegalTransition = apperr.Precondition("ILLEGAL_LIFECYCLE_TRANSITION", "the requested lifecycle transition is not permitted")
	ErrOnCallRequired    = apperr.Precondition("ONCALL_REQUIRED", "a service cannot enter production without an on-call reference")
	ErrAlreadyRetired    = apperr.Precondition("SERVICE_RETIRED", "a retired service cannot be modified")

	ErrServiceNotFound = apperr.NotFound("SERVICE_NOT_FOUND", "service not found")
	ErrNameConflict    = apperr.Conflict("SERVICE_NAME_CONFLICT", "a service with this name already exists in the tenant")
	ErrVersionConflict = apperr.Conflict("VERSION_CONFLICT", "the service was modified concurrently; retry with the current version")
)
