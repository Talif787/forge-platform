package api

import (
	"time"

	"github.com/forge-platform/forge/internal/modules/catalog/domain"
)

type RegisterServiceRequest struct {
	TenantID    string `json:"tenantId"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Tier        int    `json:"tier"`
	Repository  string `json:"repository"`
	OwningTeam  string `json:"owningTeam"`
	OnCallRef   string `json:"onCallRef"`
}

type ChangeOwnershipRequest struct {
	OwningTeam string `json:"owningTeam"`
	OnCallRef  string `json:"onCallRef"`
}

type ChangeLifecycleRequest struct {
	Lifecycle string `json:"lifecycle"`
}

type OwnershipDTO struct {
	OwningTeam string `json:"owningTeam"`
	OnCallRef  string `json:"onCallRef,omitempty"`
}

type ServiceResponse struct {
	ID          string       `json:"id"`
	TenantID    string       `json:"tenantId"`
	Name        string       `json:"name"`
	Description string       `json:"description,omitempty"`
	Tier        int          `json:"tier"`
	Lifecycle   string       `json:"lifecycle"`
	Repository  string       `json:"repository,omitempty"`
	Ownership   OwnershipDTO `json:"ownership"`
	Version     int64        `json:"version"`
	CreatedAt   time.Time    `json:"createdAt"`
	UpdatedAt   time.Time    `json:"updatedAt"`
}

func toResponse(s *domain.Service) ServiceResponse {
	return ServiceResponse{
		ID:          s.ID().String(),
		TenantID:    s.TenantID().String(),
		Name:        s.Name().String(),
		Description: s.Description(),
		Tier:        s.Tier().Int(),
		Lifecycle:   string(s.Lifecycle()),
		Repository:  s.Repository(),
		Ownership:   OwnershipDTO{OwningTeam: s.Ownership().OwningTeam, OnCallRef: s.Ownership().OnCallRef},
		Version:     s.Version(),
		CreatedAt:   s.CreatedAt(),
		UpdatedAt:   s.UpdatedAt(),
	}
}
