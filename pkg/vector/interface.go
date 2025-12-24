package vector

import "context"

// Store defines the interface for vector storage backends.
type Store interface {
	// Store stores a document with the given ID
	Store(ctx context.Context, id string, doc map[string]any) error

	// Search searches for documents based on query
	Search(ctx context.Context, query SearchQuery) ([]map[string]any, error)
}
