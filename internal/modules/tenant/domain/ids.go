package domain

import (
	"github.com/google/uuid"

	"github.com/forge-platform/forge/internal/platform/id"
)

type TenantID struct{ v uuid.UUID }

func NewTenantID() TenantID { return TenantID{v: id.New()} }

func TenantIDFromString(s string) (TenantID, error) {
	u, err := uuid.Parse(s)
	if err != nil {
		return TenantID{}, ErrInvalidTenantID
	}
	return TenantID{v: u}, nil
}

func (t TenantID) String() string { return t.v.String() }
func (t TenantID) IsZero() bool   { return t.v == uuid.Nil }
