package domain

import (
	"github.com/google/uuid"

	"github.com/forge-platform/forge/internal/platform/id"
)

type ServiceID struct{ v uuid.UUID }

func NewServiceID() ServiceID { return ServiceID{v: id.New()} }

func ServiceIDFromString(s string) (ServiceID, error) {
	u, err := uuid.Parse(s)
	if err != nil {
		return ServiceID{}, ErrInvalidServiceID
	}
	return ServiceID{v: u}, nil
}

func (s ServiceID) String() string  { return s.v.String() }
func (s ServiceID) UUID() uuid.UUID { return s.v }
func (s ServiceID) IsZero() bool    { return s.v == uuid.Nil }

type TenantID struct{ v uuid.UUID }

func TenantIDFromString(s string) (TenantID, error) {
	u, err := uuid.Parse(s)
	if err != nil {
		return TenantID{}, ErrInvalidTenantID
	}
	return TenantID{v: u}, nil
}

func (t TenantID) String() string  { return t.v.String() }
func (t TenantID) UUID() uuid.UUID { return t.v }
func (t TenantID) IsZero() bool    { return t.v == uuid.Nil }
