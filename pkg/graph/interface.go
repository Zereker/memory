package graph

import "context"

// GraphStore defines the interface for graph storage operations
type GraphStore interface {
	// MergeNode creates or updates a node based on a match key
	MergeNode(ctx context.Context, labels []string, matchKey string, matchValue any, properties map[string]any) error

	// CreateRelationship creates a relationship between two nodes
	CreateRelationship(ctx context.Context, fromLabel, fromKey string, fromValue any, toLabel, toKey string, toValue any, relType string, properties map[string]any) error

	// Traverse performs a graph traversal from a starting node
	Traverse(ctx context.Context, startLabel, startKey string, startValue any, relTypes []string, direction string, maxDepth, limit int) ([]map[string]any, error)
}
