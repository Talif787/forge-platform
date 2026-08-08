package domain

import "time"

// Tenant is the aggregate root of the tenant bounded context. All state changes
// go through its methods so invariants hold and events are recorded.
type Tenant struct {
	id        TenantID
	name      TenantName
	slug      Slug
	plan      Plan
	status    Status
	quota     Quota
	version   int64
	createdAt time.Time
	updatedAt time.Time

	events []Event
}

func Create(id TenantID, name TenantName, slug Slug, plan Plan, quota Quota, now time.Time) *Tenant {
	t := &Tenant{
		id: id, name: name, slug: slug, plan: plan, status: StatusActive, quota: quota,
		version: 1, createdAt: now, updatedAt: now,
	}
	t.record(TenantCreated{base: base{id: id.String(), at: now}, Name: name.String(), Slug: slug.String(), Plan: string(plan)})
	return t
}

func Reconstitute(id TenantID, name TenantName, slug Slug, plan Plan, status Status, quota Quota, version int64, createdAt, updatedAt time.Time) *Tenant {
	return &Tenant{
		id: id, name: name, slug: slug, plan: plan, status: status, quota: quota,
		version: version, createdAt: createdAt, updatedAt: updatedAt,
	}
}

func (t *Tenant) ChangeStatus(target Status, now time.Time) error {
	if t.status == target {
		return nil
	}
	if !t.status.canTransitionTo(target) {
		return ErrIllegalTransition
	}
	from := t.status
	t.status = target
	t.touch(now)
	t.record(TenantStatusChanged{base: base{id: t.id.String(), at: now}, From: string(from), To: string(target)})
	return nil
}

func (t *Tenant) UpdateQuota(q Quota, now time.Time) error {
	if t.status == StatusArchived {
		return ErrArchived
	}
	t.quota = q
	t.touch(now)
	t.record(TenantQuotaUpdated{base: base{id: t.id.String(), at: now}, MaxServices: q.MaxServices})
	return nil
}

func (t *Tenant) touch(now time.Time) {
	t.version++
	t.updatedAt = now
}

func (t *Tenant) record(e Event) { t.events = append(t.events, e) }

func (t *Tenant) PullEvents() []Event {
	events := t.events
	t.events = nil
	return events
}

func (t *Tenant) ID() TenantID       { return t.id }
func (t *Tenant) Name() TenantName   { return t.name }
func (t *Tenant) Slug() Slug         { return t.slug }
func (t *Tenant) Plan() Plan         { return t.plan }
func (t *Tenant) Status() Status     { return t.status }
func (t *Tenant) Quota() Quota       { return t.quota }
func (t *Tenant) Version() int64     { return t.version }
func (t *Tenant) CreatedAt() time.Time { return t.createdAt }
func (t *Tenant) UpdatedAt() time.Time { return t.updatedAt }
