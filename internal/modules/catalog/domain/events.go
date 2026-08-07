package domain

import "time"

type Event interface {
	EventType() string
	AggregateID() string
	OccurredAt() time.Time
}

type base struct {
	id string
	at time.Time
}

func (b base) AggregateID() string   { return b.id }
func (b base) OccurredAt() time.Time { return b.at }

type ServiceRegistered struct {
	base
	TenantID string
	Name     string
	Tier     int
}

func (ServiceRegistered) EventType() string { return "catalog.service.registered" }

type ServiceOwnershipChanged struct {
	base
	PreviousTeam string
	NewTeam      string
}

func (ServiceOwnershipChanged) EventType() string { return "catalog.service.ownership_changed" }

type ServiceLifecycleChanged struct {
	base
	From string
	To   string
}

func (ServiceLifecycleChanged) EventType() string { return "catalog.service.lifecycle_changed" }

type ServiceRetired struct {
	base
}

func (ServiceRetired) EventType() string { return "catalog.service.retired" }
