package graph

import (
	"context"
	"fmt"
	"time"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

// Package-level instance
var neo4jInstance *Neo4jStore

// Init initializes the graph package with config.
func Init(cfg Neo4jConfig) error {
	if !cfg.Enabled {
		return nil
	}

	store, err := newStore(cfg)
	if err != nil {
		return err
	}

	neo4jInstance = store
	return nil
}

// NewStore returns the Neo4jStore instance.
func NewStore() *Neo4jStore {
	return neo4jInstance
}

// Close closes the Neo4jStore connection.
func Close(ctx context.Context) error {
	if neo4jInstance != nil {
		return neo4jInstance.Close(ctx)
	}
	return nil
}

// Neo4jConfig holds Neo4j connection configuration
type Neo4jConfig struct {
	Enabled  bool   `toml:"enabled"`
	URI      string `toml:"uri"`
	Username string `toml:"username"`
	Password string `toml:"password"`
	Database string `toml:"database"`
}

// Validate checks Neo4j configuration.
func (c *Neo4jConfig) Validate() error {
	if c.URI == "" {
		return fmt.Errorf("uri is required")
	}
	if c.Database == "" {
		return fmt.Errorf("database is required")
	}
	return nil
}

// Neo4jStore provides a generic Neo4j graph store
type Neo4jStore struct {
	driver   neo4j.DriverWithContext
	database string
}

// newStore creates a new Neo4j graph store
func newStore(cfg Neo4jConfig) (*Neo4jStore, error) {
	driver, err := neo4j.NewDriverWithContext(cfg.URI, neo4j.BasicAuth(cfg.Username, cfg.Password, ""))
	if err != nil {
		return nil, fmt.Errorf("failed to create Neo4j driver: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := driver.VerifyConnectivity(ctx); err != nil {
		return nil, fmt.Errorf("failed to verify Neo4j connectivity: %w", err)
	}

	return &Neo4jStore{
		driver:   driver,
		database: cfg.Database,
	}, nil
}

// ============================================================================
// Generic Query Methods
// ============================================================================

// Run executes a Cypher query and returns results as []map[string]any
func (s *Neo4jStore) Run(ctx context.Context, cypher string, params map[string]any) ([]map[string]any, error) {
	session := s.driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: s.database})
	defer session.Close(ctx)

	result, err := session.Run(ctx, cypher, params)
	if err != nil {
		return nil, fmt.Errorf("cypher execution failed: %w", err)
	}

	records, err := result.Collect(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to collect results: %w", err)
	}

	results := make([]map[string]any, 0, len(records))
	for _, record := range records {
		row := make(map[string]any)
		for _, key := range record.Keys {
			val, _ := record.Get(key)
			row[key] = s.convertValue(val)
		}
		results = append(results, row)
	}

	return results, nil
}

// RunWrite executes a write Cypher query in a transaction
func (s *Neo4jStore) RunWrite(ctx context.Context, cypher string, params map[string]any) error {
	session := s.driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: s.database})
	defer session.Close(ctx)

	_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		_, err := tx.Run(ctx, cypher, params)
		return nil, err
	})

	return err
}

// RunWriteBatch executes multiple write Cypher queries in a single transaction
func (s *Neo4jStore) RunWriteBatch(ctx context.Context, queries []string, paramsList []map[string]any) error {
	if len(queries) != len(paramsList) {
		return fmt.Errorf("queries and params length mismatch")
	}

	session := s.driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: s.database})
	defer session.Close(ctx)

	_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		for i, query := range queries {
			if _, err := tx.Run(ctx, query, paramsList[i]); err != nil {
				return nil, err
			}
		}
		return nil, nil
	})

	return err
}

// ============================================================================
// Node Operations
// ============================================================================

// MergeNode creates or updates a node based on a match key
func (s *Neo4jStore) MergeNode(ctx context.Context, labels []string, matchKey string, matchValue any, properties map[string]any) error {
	if len(labels) == 0 {
		return fmt.Errorf("at least one label is required")
	}

	labelStr := ":" + labels[0]
	for _, l := range labels[1:] {
		labelStr += ":" + l
	}

	cypher := fmt.Sprintf(`
		MERGE (n%s {%s: $match_value})
		SET n += $props
	`, labelStr, matchKey)

	return s.RunWrite(ctx, cypher, map[string]any{
		"match_value": matchValue,
		"props":       properties,
	})
}

// GetNode retrieves a node by a property match
func (s *Neo4jStore) GetNode(ctx context.Context, label, key string, value any) (map[string]any, error) {
	cypher := fmt.Sprintf(`
		MATCH (n:%s {%s: $value})
		RETURN n
		LIMIT 1
	`, label, key)

	results, err := s.Run(ctx, cypher, map[string]any{"value": value})
	if err != nil {
		return nil, err
	}

	if len(results) == 0 {
		return nil, nil
	}

	if node, ok := results[0]["n"].(map[string]any); ok {
		return node, nil
	}

	return nil, nil
}

// FindNodes finds nodes matching the given criteria
func (s *Neo4jStore) FindNodes(ctx context.Context, label string, filters map[string]any, limit int) ([]map[string]any, error) {
	if limit <= 0 {
		limit = 100
	}

	whereClause := ""
	params := map[string]any{"limit": limit}

	i := 0
	for key, value := range filters {
		if i == 0 {
			whereClause = " WHERE "
		} else {
			whereClause += " AND "
		}
		paramKey := fmt.Sprintf("p%d", i)
		whereClause += fmt.Sprintf("n.%s = $%s", key, paramKey)
		params[paramKey] = value
		i++
	}

	cypher := fmt.Sprintf(`
		MATCH (n:%s)%s
		RETURN n
		LIMIT $limit
	`, label, whereClause)

	results, err := s.Run(ctx, cypher, params)
	if err != nil {
		return nil, err
	}

	nodes := make([]map[string]any, 0, len(results))
	for _, row := range results {
		if node, ok := row["n"].(map[string]any); ok {
			nodes = append(nodes, node)
		}
	}

	return nodes, nil
}

// DeleteNode deletes a node and its relationships
func (s *Neo4jStore) DeleteNode(ctx context.Context, label, key string, value any) error {
	cypher := fmt.Sprintf(`
		MATCH (n:%s {%s: $value})
		DETACH DELETE n
	`, label, key)

	return s.RunWrite(ctx, cypher, map[string]any{"value": value})
}

// ============================================================================
// Relationship Operations
// ============================================================================

// CreateRelationship creates a relationship between two nodes
func (s *Neo4jStore) CreateRelationship(ctx context.Context,
	fromLabel, fromKey string, fromValue any,
	toLabel, toKey string, toValue any,
	relType string, properties map[string]any) error {

	cypher := fmt.Sprintf(`
		MATCH (from:%s {%s: $from_value})
		MATCH (to:%s {%s: $to_value})
		MERGE (from)-[r:%s]->(to)
		SET r += $props
	`, fromLabel, fromKey, toLabel, toKey, relType)

	return s.RunWrite(ctx, cypher, map[string]any{
		"from_value": fromValue,
		"to_value":   toValue,
		"props":      properties,
	})
}

// FindRelationships finds relationships from a node
func (s *Neo4jStore) FindRelationships(ctx context.Context,
	fromLabel, fromKey string, fromValue any,
	relType string, limit int) ([]map[string]any, error) {

	if limit <= 0 {
		limit = 100
	}

	relPattern := ""
	if relType != "" {
		relPattern = ":" + relType
	}

	cypher := fmt.Sprintf(`
		MATCH (from:%s {%s: $from_value})-[r%s]->(to)
		RETURN r, from, to, type(r) as rel_type
		LIMIT $limit
	`, fromLabel, fromKey, relPattern)

	return s.Run(ctx, cypher, map[string]any{
		"from_value": fromValue,
		"limit":      limit,
	})
}

// DeleteRelationship deletes a relationship by property
func (s *Neo4jStore) DeleteRelationship(ctx context.Context, key string, value any) error {
	cypher := fmt.Sprintf(`
		MATCH ()-[r {%s: $value}]->()
		DELETE r
	`, key)

	return s.RunWrite(ctx, cypher, map[string]any{"value": value})
}

// ============================================================================
// Graph Traversal
// ============================================================================

// Traverse performs a graph traversal from a starting node
func (s *Neo4jStore) Traverse(ctx context.Context,
	startLabel, startKey string, startValue any,
	relTypes []string, direction string,
	maxDepth, limit int) ([]map[string]any, error) {

	if maxDepth <= 0 {
		maxDepth = 2
	}
	if limit <= 0 {
		limit = 100
	}

	relPattern := ""
	if len(relTypes) > 0 {
		relPattern = ":" + relTypes[0]
		for _, rt := range relTypes[1:] {
			relPattern += "|" + rt
		}
	}

	leftArrow, rightArrow := "-", "->"
	if direction == "incoming" {
		leftArrow, rightArrow = "<-", "-"
	} else if direction == "both" {
		leftArrow, rightArrow = "-", "-"
	}

	cypher := fmt.Sprintf(`
		MATCH (start:%s {%s: $start_value})%s[%s*1..%d]%s(related)
		RETURN DISTINCT related
		LIMIT $limit
	`, startLabel, startKey, leftArrow, relPattern, maxDepth, rightArrow)

	results, err := s.Run(ctx, cypher, map[string]any{
		"start_value": startValue,
		"limit":       limit,
	})
	if err != nil {
		return nil, err
	}

	nodes := make([]map[string]any, 0, len(results))
	for _, row := range results {
		if node, ok := row["related"].(map[string]any); ok {
			nodes = append(nodes, node)
		}
	}

	return nodes, nil
}

// ============================================================================
// Utility Methods
// ============================================================================

// Health checks Neo4j connection
func (s *Neo4jStore) Health(ctx context.Context) error {
	return s.driver.VerifyConnectivity(ctx)
}

// Close closes the Neo4j connection
func (s *Neo4jStore) Close(ctx context.Context) error {
	return s.driver.Close(ctx)
}

// convertValue converts Neo4j types to Go types
func (s *Neo4jStore) convertValue(val any) any {
	switch v := val.(type) {
	case neo4j.Node:
		return v.Props
	case neo4j.Relationship:
		return v.Props
	case neo4j.Path:
		// Return path as a map with nodes and relationships
		nodes := make([]map[string]any, len(v.Nodes))
		rels := make([]map[string]any, len(v.Relationships))
		for i, node := range v.Nodes {
			nodes[i] = node.Props
		}
		for i, rel := range v.Relationships {
			rels[i] = rel.Props
		}
		return map[string]any{"nodes": nodes, "relationships": rels}
	default:
		return val
	}
}
