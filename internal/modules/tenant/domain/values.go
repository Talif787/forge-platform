package domain

import (
	"regexp"
	"strings"
)

var slugPattern = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)

type TenantName struct{ v string }

func NewTenantName(s string) (TenantName, error) {
	s = strings.TrimSpace(s)
	if len(s) < 1 || len(s) > 100 {
		return TenantName{}, ErrInvalidTenantName
	}
	return TenantName{v: s}, nil
}

func (n TenantName) String() string { return n.v }

// Slug is a stable, URL-safe DNS-1123 label used to reference the tenant.
type Slug struct{ v string }

func NewSlug(s string) (Slug, error) {
	s = strings.TrimSpace(s)
	if len(s) < 1 || len(s) > 63 || !slugPattern.MatchString(s) {
		return Slug{}, ErrInvalidSlug
	}
	return Slug{v: s}, nil
}

func (s Slug) String() string { return s.v }

type Plan string

const (
	PlanFree       Plan = "free"
	PlanStandard   Plan = "standard"
	PlanEnterprise Plan = "enterprise"
)

var validPlans = map[Plan]bool{PlanFree: true, PlanStandard: true, PlanEnterprise: true}

func ParsePlan(s string) (Plan, error) {
	p := Plan(s)
	if !validPlans[p] {
		return "", ErrInvalidPlan
	}
	return p, nil
}

type Status string

const (
	StatusActive    Status = "active"
	StatusSuspended Status = "suspended"
	StatusArchived  Status = "archived"
)

var statusTransitions = map[Status]map[Status]bool{
	StatusActive:    {StatusSuspended: true, StatusArchived: true},
	StatusSuspended: {StatusActive: true, StatusArchived: true},
	StatusArchived:  {},
}

func ParseStatus(s string) (Status, error) {
	st := Status(s)
	if _, ok := statusTransitions[st]; !ok {
		return "", ErrInvalidStatus
	}
	return st, nil
}

func (s Status) canTransitionTo(target Status) bool { return statusTransitions[s][target] }

// Quota bounds tenant resource usage. MaxServices caps how many catalog
// services the tenant may own (enforcement is wired in a later phase).
type Quota struct{ MaxServices int }

func NewQuota(maxServices int) (Quota, error) {
	if maxServices < 0 {
		return Quota{}, ErrInvalidQuota
	}
	return Quota{MaxServices: maxServices}, nil
}
