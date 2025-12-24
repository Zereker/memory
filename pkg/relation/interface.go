package relation

import (
	"context"
	"time"
)

// Relation represents an event relationship stored in PostgreSQL.
type Relation struct {
	ID           string
	FromEventID  string
	ToEventID    string
	RelationType string // "causal" / "temporal"
	CreatedAt    time.Time
}

// Store defines the interface for event relation storage.
type Store interface {
	// CreateRelation creates or updates an event relation (UPSERT semantics).
	CreateRelation(ctx context.Context, rel Relation) error

	// DeleteByEventID deletes all relations involving the given event ID.
	DeleteByEventID(ctx context.Context, eventID string) error

	// Close releases resources held by the store.
	Close(ctx context.Context) error
}
