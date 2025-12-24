package storage

import "context"

// VectorStore defines the generic interface for vector storage backends.
// All methods work with map[string]any for maximum flexibility.
// Domain-specific types should be converted at the application layer.
type VectorStore interface {
	// Store stores a document with the given ID
	Store(ctx context.Context, id string, doc map[string]any) error

	// Get retrieves a document by ID
	Get(ctx context.Context, id string) (map[string]any, error)

	// Search searches for documents based on query
	Search(ctx context.Context, query SearchQuery) ([]map[string]any, error)

	// Delete deletes a document by ID
	Delete(ctx context.Context, id string) error

	// DeleteByQuery deletes documents matching the filters
	DeleteByQuery(ctx context.Context, filters map[string]any) (int, error)

	// Count counts documents matching the filters
	Count(ctx context.Context, filters map[string]any) (int, error)

	// Update updates a document (upsert)
	Update(ctx context.Context, id string, doc map[string]any) error

	// UpdateFields updates specific fields of a document
	UpdateFields(ctx context.Context, id string, fields map[string]any) error

	// Close closes the storage connection
	Close() error
}
