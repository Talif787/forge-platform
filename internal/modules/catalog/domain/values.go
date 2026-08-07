package domain

import (
	"regexp"
	"strings"
)

// dns1123Label matches a Kubernetes DNS-1123 label, the naming rule for
// services (SRD BR-040).
var dns1123Label = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)

type ServiceName struct{ v string }

func NewServiceName(s string) (ServiceName, error) {
	s = strings.TrimSpace(s)
	if len(s) < 1 || len(s) > 63 || !dns1123Label.MatchString(s) {
		return ServiceName{}, ErrInvalidServiceName
	}
	return ServiceName{v: s}, nil
}

func (n ServiceName) String() string { return n.v }

type Tier struct{ v int }

func NewTier(t int) (Tier, error) {
	if t < 1 || t > 4 {
		return Tier{}, ErrInvalidTier
	}
	return Tier{v: t}, nil
}

func (t Tier) Int() int         { return t.v }
func (t Tier) IsCritical() bool { return t.v <= 2 }

type Lifecycle string

const (
	LifecycleExperimental Lifecycle = "experimental"
	LifecycleProduction   Lifecycle = "production"
	LifecycleDeprecated   Lifecycle = "deprecated"
	LifecycleRetired      Lifecycle = "retired"
)

var lifecycleTransitions = map[Lifecycle]map[Lifecycle]bool{
	LifecycleExperimental: {LifecycleProduction: true, LifecycleDeprecated: true, LifecycleRetired: true},
	LifecycleProduction:   {LifecycleDeprecated: true, LifecycleRetired: true},
	LifecycleDeprecated:   {LifecycleProduction: true, LifecycleRetired: true},
	LifecycleRetired:      {},
}

func ParseLifecycle(s string) (Lifecycle, error) {
	l := Lifecycle(s)
	if _, ok := lifecycleTransitions[l]; !ok {
		return "", ErrInvalidLifecycle
	}
	return l, nil
}

func (l Lifecycle) canTransitionTo(target Lifecycle) bool {
	return lifecycleTransitions[l][target]
}

// Ownership is a value object. On-call is required before a service may run in
// production (SRD BR-042), enforced during lifecycle transitions.
type Ownership struct {
	OwningTeam string
	OnCallRef  string
}

func NewOwnership(team, onCall string) (Ownership, error) {
	team = strings.TrimSpace(team)
	if team == "" {
		return Ownership{}, ErrInvalidOwnership
	}
	return Ownership{OwningTeam: team, OnCallRef: strings.TrimSpace(onCall)}, nil
}

func (o Ownership) hasOnCall() bool { return o.OnCallRef != "" }
