package api

import (
	"time"

	"github.com/forge-platform/forge/internal/modules/tenant/domain"
)

type CreateTenantRequest struct {
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Plan        string `json:"plan"`
	MaxServices int    `json:"maxServices"`
}

type ChangeStatusRequest struct {
	Status string `json:"status"`
}

type UpdateQuotaRequest struct {
	MaxServices int `json:"maxServices"`
}

type TenantResponse struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Slug        string    `json:"slug"`
	Plan        string    `json:"plan"`
	Status      string    `json:"status"`
	MaxServices int       `json:"maxServices"`
	Version     int64     `json:"version"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

func toResponse(t *domain.Tenant) TenantResponse {
	return TenantResponse{
		ID:          t.ID().String(),
		Name:        t.Name().String(),
		Slug:        t.Slug().String(),
		Plan:        string(t.Plan()),
		Status:      string(t.Status()),
		MaxServices: t.Quota().MaxServices,
		Version:     t.Version(),
		CreatedAt:   t.CreatedAt(),
		UpdatedAt:   t.UpdatedAt(),
	}
}
