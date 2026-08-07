package domain

import "time"

// Service is the catalog aggregate root. All state changes go through its
// methods so invariants hold and domain events are recorded. Version supports
// optimistic concurrency.
type Service struct {
	id          ServiceID
	tenantID    TenantID
	name        ServiceName
	description string
	tier        Tier
	lifecycle   Lifecycle
	repository  string
	ownership   Ownership
	version     int64
	createdAt   time.Time
	updatedAt   time.Time

	events []Event
}

func Register(id ServiceID, tenantID TenantID, name ServiceName, description string, tier Tier, repository string, ownership Ownership, now time.Time) *Service {
	s := &Service{
		id:          id,
		tenantID:    tenantID,
		name:        name,
		description: description,
		tier:        tier,
		lifecycle:   LifecycleExperimental,
		repository:  repository,
		ownership:   ownership,
		version:     1,
		createdAt:   now,
		updatedAt:   now,
	}
	s.record(ServiceRegistered{base: base{id: id.String(), at: now}, TenantID: tenantID.String(), Name: name.String(), Tier: tier.Int()})
	return s
}

// Reconstitute rebuilds an aggregate from persistence without emitting events.
func Reconstitute(id ServiceID, tenantID TenantID, name ServiceName, description string, tier Tier, lifecycle Lifecycle, repository string, ownership Ownership, version int64, createdAt, updatedAt time.Time) *Service {
	return &Service{
		id: id, tenantID: tenantID, name: name, description: description,
		tier: tier, lifecycle: lifecycle, repository: repository, ownership: ownership,
		version: version, createdAt: createdAt, updatedAt: updatedAt,
	}
}

func (s *Service) ChangeOwnership(o Ownership, now time.Time) error {
	if s.lifecycle == LifecycleRetired {
		return ErrAlreadyRetired
	}
	if s.lifecycle == LifecycleProduction && !o.hasOnCall() {
		return ErrOnCallRequired
	}
	previous := s.ownership.OwningTeam
	s.ownership = o
	s.touch(now)
	s.record(ServiceOwnershipChanged{base: base{id: s.id.String(), at: now}, PreviousTeam: previous, NewTeam: o.OwningTeam})
	return nil
}

func (s *Service) ChangeLifecycle(target Lifecycle, now time.Time) error {
	if s.lifecycle == target {
		return nil
	}
	if !s.lifecycle.canTransitionTo(target) {
		return ErrIllegalTransition
	}
	if target == LifecycleProduction && !s.ownership.hasOnCall() {
		return ErrOnCallRequired
	}
	from := s.lifecycle
	s.lifecycle = target
	s.touch(now)
	s.record(ServiceLifecycleChanged{base: base{id: s.id.String(), at: now}, From: string(from), To: string(target)})
	return nil
}

func (s *Service) Retire(now time.Time) error {
	if s.lifecycle == LifecycleRetired {
		return nil
	}
	s.lifecycle = LifecycleRetired
	s.touch(now)
	s.record(ServiceRetired{base: base{id: s.id.String(), at: now}})
	return nil
}

func (s *Service) touch(now time.Time) {
	s.version++
	s.updatedAt = now
}

func (s *Service) record(e Event) { s.events = append(s.events, e) }

// PullEvents returns and clears recorded events for the caller to persist.
func (s *Service) PullEvents() []Event {
	events := s.events
	s.events = nil
	return events
}

func (s *Service) ID() ServiceID        { return s.id }
func (s *Service) TenantID() TenantID   { return s.tenantID }
func (s *Service) Name() ServiceName    { return s.name }
func (s *Service) Description() string  { return s.description }
func (s *Service) Tier() Tier           { return s.tier }
func (s *Service) Lifecycle() Lifecycle { return s.lifecycle }
func (s *Service) Repository() string   { return s.repository }
func (s *Service) Ownership() Ownership { return s.ownership }
func (s *Service) Version() int64       { return s.version }
func (s *Service) CreatedAt() time.Time { return s.createdAt }
func (s *Service) UpdatedAt() time.Time { return s.updatedAt }
