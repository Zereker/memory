package relation

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Package-level singleton instance.
var pgInstance *PostgresStore

// Init initializes the relation package with config.
func Init(cfg PostgresConfig) error {
	if !cfg.Enabled {
		return nil
	}

	store, err := newPostgresStore(cfg)
	if err != nil {
		return err
	}

	pgInstance = store
	return nil
}

// NewStore returns the PostgresStore singleton instance.
func NewStore() *PostgresStore {
	return pgInstance
}

// Close closes the PostgresStore connection.
func Close(ctx context.Context) error {
	if pgInstance != nil {
		return pgInstance.Close(ctx)
	}
	return nil
}

// PostgresStore implements Store using PostgreSQL.
type PostgresStore struct {
	pool *pgxpool.Pool
}

func newPostgresStore(cfg PostgresConfig) (*PostgresStore, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, cfg.DSN())
	if err != nil {
		return nil, fmt.Errorf("failed to create pgx pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping PostgreSQL: %w", err)
	}

	store := &PostgresStore{pool: pool}
	if err := store.ensureSchema(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ensure schema: %w", err)
	}

	return store, nil
}

// ensureSchema creates the event_relations table and indexes if they don't exist.
func (s *PostgresStore) ensureSchema(ctx context.Context) error {
	ddl := `
CREATE TABLE IF NOT EXISTS event_relations (
    id              TEXT        PRIMARY KEY,
    from_event_id   TEXT        NOT NULL,
    to_event_id     TEXT        NOT NULL,
    relation_type   TEXT        NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_event_relations_from ON event_relations (from_event_id);
CREATE INDEX IF NOT EXISTS idx_event_relations_to   ON event_relations (to_event_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_event_relations_unique
    ON event_relations (from_event_id, to_event_id, relation_type);
`
	_, err := s.pool.Exec(ctx, ddl)
	return err
}

// CreateRelation inserts or updates an event relation (UPSERT).
func (s *PostgresStore) CreateRelation(ctx context.Context, rel Relation) error {
	query := `
INSERT INTO event_relations (id, from_event_id, to_event_id, relation_type, created_at)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (from_event_id, to_event_id, relation_type)
DO UPDATE SET id = EXCLUDED.id, created_at = EXCLUDED.created_at
`
	_, err := s.pool.Exec(ctx, query, rel.ID, rel.FromEventID, rel.ToEventID, rel.RelationType, rel.CreatedAt)
	if err != nil {
		return fmt.Errorf("failed to create relation: %w", err)
	}
	return nil
}

// DeleteByEventID deletes all relations involving the given event ID.
func (s *PostgresStore) DeleteByEventID(ctx context.Context, eventID string) error {
	query := `DELETE FROM event_relations WHERE from_event_id = $1 OR to_event_id = $1`
	_, err := s.pool.Exec(ctx, query, eventID)
	if err != nil {
		return fmt.Errorf("failed to delete relations for event %s: %w", eventID, err)
	}
	return nil
}

// Close releases the connection pool.
func (s *PostgresStore) Close(_ context.Context) error {
	s.pool.Close()
	return nil
}
