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

type TenantCreated struct {
	base
	Name string `json:"name"`
	Slug string `json:"slug"`
	Plan string `json:"plan"`
}

func (TenantCreated) EventType() string { return "tenant.created" }

type TenantStatusChanged struct {
	base
	From string `json:"from"`
	To   string `json:"to"`
}

func (TenantStatusChanged) EventType() string { return "tenant.status_changed" }

type TenantQuotaUpdated struct {
	base
	MaxServices int `json:"maxServices"`
}

func (TenantQuotaUpdated) EventType() string { return "tenant.quota_updated" }
