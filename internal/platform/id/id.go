package id

import "github.com/google/uuid"

// New returns a time-ordered UUIDv7, chosen over v4 to avoid index
// fragmentation on B-tree primary keys at scale.
func New() uuid.UUID { return uuid.Must(uuid.NewV7()) }

func Parse(s string) (uuid.UUID, error) { return uuid.Parse(s) }
