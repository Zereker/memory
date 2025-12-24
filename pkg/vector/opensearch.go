package vector

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/opensearch-project/opensearch-go/v4"
	"github.com/opensearch-project/opensearch-go/v4/opensearchapi"
)

// Document status constants for soft delete
const (
	StatusActive   = "active"
	StatusArchived = "archived"
	StatusDeleted  = "deleted"
)

// Package-level singleton instance
var storeInstance *OpenSearchStore

// Init initializes the OpenSearch store singleton with config.
func Init(cfg OpenSearchConfig) error {
	store, err := NewOpenSearchStore(cfg)
	if err != nil {
		return err
	}
	storeInstance = store
	return nil
}

// NewStore returns the singleton OpenSearch store instance.
func NewStore() *OpenSearchStore {
	return storeInstance
}

// OpenSearchConfig holds OpenSearch configuration
type OpenSearchConfig struct {
	Addresses    []string `toml:"addresses"`
	Username     string   `toml:"username"`
	Password     string   `toml:"password"`
	IndexName    string   `toml:"index"`
	EmbeddingDim int      `toml:"embedding_dim"`
	InsecureSSL  bool     `toml:"insecure_ssl"`
}

// Validate checks OpenSearch configuration
func (c *OpenSearchConfig) Validate() error {
	if len(c.Addresses) == 0 {
		return fmt.Errorf("addresses is required")
	}
	if c.IndexName == "" {
		return fmt.Errorf("index is required")
	}
	if c.EmbeddingDim <= 0 {
		return fmt.Errorf("embedding_dim must be positive")
	}
	return nil
}

// SearchQuery represents a generic search query
type SearchQuery struct {
	// Filters for exact match (field -> value)
	Filters map[string]any

	// TermsFilters for multi-value match (field -> []values)
	TermsFilters map[string][]string

	// RangeFilters for range queries (field -> {gte/lte/gt/lt -> value})
	RangeFilters map[string]map[string]any

	// Embedding vector for k-NN search
	Embedding []float32

	// TextQuery for full-text search (searches raw_content and content fields)
	TextQuery string

	// HybridSearch enables combining vector and text search
	// When true and both Embedding and TextQuery are provided, uses hybrid search
	HybridSearch bool

	// Score threshold for filtering results
	ScoreThreshold float64

	// Limit on results
	Limit int
}

// OpenSearchStore implements a generic vector store using OpenSearch k-NN
type OpenSearchStore struct {
	client       *opensearchapi.Client
	indexName    string
	embeddingDim int
}

// NewOpenSearchStore creates a new OpenSearch store
func NewOpenSearchStore(cfg OpenSearchConfig) (*OpenSearchStore, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if cfg.InsecureSSL {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}

	clientCfg := opensearchapi.Config{
		Client: opensearch.Config{
			Addresses: cfg.Addresses,
			Username:  cfg.Username,
			Password:  cfg.Password,
			Transport: transport,
		},
	}

	client, err := opensearchapi.NewClient(clientCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create OpenSearch client: %w", err)
	}

	store := &OpenSearchStore{
		client:       client,
		indexName:    cfg.IndexName,
		embeddingDim: cfg.EmbeddingDim,
	}

	return store, nil
}

// Store stores a document with the given ID
// The doc map should contain all fields including "embedding" as []float32
func (s *OpenSearchStore) Store(ctx context.Context, id string, doc map[string]any) error {
	// Add status if not present
	if _, ok := doc["status"]; !ok {
		doc["status"] = StatusActive
	}

	docBody, err := json.Marshal(doc)
	if err != nil {
		return fmt.Errorf("failed to marshal document: %w", err)
	}

	_, err = s.client.Index(ctx, opensearchapi.IndexReq{
		Index:      s.indexName,
		DocumentID: id,
		Body:       bytes.NewReader(docBody),
		Params:     opensearchapi.IndexParams{Refresh: "true"},
	})
	if err != nil {
		return fmt.Errorf("failed to index document: %w", err)
	}

	return nil
}

// Get retrieves a document by ID
func (s *OpenSearchStore) Get(ctx context.Context, id string) (map[string]any, error) {
	resp, err := s.client.Document.Get(ctx, opensearchapi.DocumentGetReq{
		Index:      s.indexName,
		DocumentID: id,
	})
	if err != nil {
		return nil, nil
	}

	if !resp.Found {
		return nil, nil
	}

	var doc map[string]any
	if err := json.Unmarshal(resp.Source, &doc); err != nil {
		return nil, fmt.Errorf("failed to unmarshal document: %w", err)
	}

	// Convert embedding back to []float32
	s.convertEmbeddingToFloat32(doc)

	return doc, nil
}

// Search searches for documents based on query
func (s *OpenSearchStore) Search(ctx context.Context, query SearchQuery) ([]map[string]any, error) {
	// Build filters
	var filters []map[string]any
	filters = append(filters, map[string]any{"term": map[string]any{"status": StatusActive}})

	// Add exact match filters
	for field, value := range query.Filters {
		filters = append(filters, map[string]any{"term": map[string]any{field: value}})
	}

	// Add terms filters (multi-value)
	for field, values := range query.TermsFilters {
		filters = append(filters, map[string]any{"terms": map[string]any{field: values}})
	}

	// Add range filters
	for field, rangeSpec := range query.RangeFilters {
		filters = append(filters, map[string]any{"range": map[string]any{field: rangeSpec}})
	}

	k := query.Limit
	if k <= 0 {
		k = 10
	}

	var searchQuery map[string]any
	hasEmbedding := len(query.Embedding) > 0
	hasTextQuery := query.TextQuery != ""

	// Hybrid search: combine k-NN and full-text search
	if query.HybridSearch && hasEmbedding && hasTextQuery {
		searchQuery = s.buildHybridQuery(query.Embedding, query.TextQuery, filters, k)
	} else if hasEmbedding {
		// Vector-only search (k-NN)
		searchQuery = map[string]any{
			"size": k,
			"query": map[string]any{
				"bool": map[string]any{
					"must":   map[string]any{"knn": map[string]any{"embedding": map[string]any{"vector": query.Embedding, "k": k}}},
					"filter": filters,
				},
			},
		}
	} else if hasTextQuery {
		// Text-only search (full-text)
		searchQuery = map[string]any{
			"size": k,
			"query": map[string]any{
				"bool": map[string]any{
					"must": map[string]any{
						"multi_match": map[string]any{
							"query":  query.TextQuery,
							"fields": []string{"raw_content^2", "content"}, // 原文权重更高
							"type":   "best_fields",
						},
					},
					"filter": filters,
				},
			},
		}
	} else {
		// No search criteria, just filter with sorting by created_at
		searchQuery = map[string]any{
			"size": k,
			"sort": []map[string]any{{"created_at": map[string]any{"order": "desc"}}},
			"query": map[string]any{
				"bool": map[string]any{"filter": filters},
			},
		}
	}

	queryBody, _ := json.Marshal(searchQuery)
	searchResp, err := s.client.Search(ctx, &opensearchapi.SearchReq{
		Indices: []string{s.indexName},
		Body:    bytes.NewReader(queryBody),
	})
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	// Parse results
	var results []map[string]any
	for _, hit := range searchResp.Hits.Hits {
		var doc map[string]any
		if err := json.Unmarshal(hit.Source, &doc); err != nil {
			continue
		}

		score := float64(hit.Score)
		if query.ScoreThreshold > 0 && score < query.ScoreThreshold {
			continue
		}

		// Convert embedding back to []float32
		s.convertEmbeddingToFloat32(doc)

		// Add score to document
		doc["_score"] = score

		results = append(results, doc)
	}

	return results, nil
}

// buildHybridQuery builds a hybrid query combining k-NN and full-text search
// Uses OpenSearch's bool query with should clauses to combine scores
func (s *OpenSearchStore) buildHybridQuery(embedding []float32, textQuery string, filters []map[string]any, k int) map[string]any {
	return map[string]any{
		"size": k,
		"query": map[string]any{
			"bool": map[string]any{
				"should": []map[string]any{
					// k-NN 向量检索
					{
						"knn": map[string]any{
							"embedding": map[string]any{
								"vector": embedding,
								"k":      k,
							},
						},
					},
					// 全文检索（搜索原文和摘要）
					{
						"multi_match": map[string]any{
							"query":  textQuery,
							"fields": []string{"raw_content^2", "content"}, // 原文权重更高
							"type":   "best_fields",
							"boost":  0.5, // 全文检索权重稍低于向量
						},
					},
				},
				"minimum_should_match": 1,
				"filter":               filters,
			},
		},
	}
}

// convertEmbeddingToFloat32 converts embedding fields from []any to []float32
func (s *OpenSearchStore) convertEmbeddingToFloat32(doc map[string]any) {
	for _, field := range []string{"embedding", "content_embedding", "topic_embedding"} {
		if emb, ok := doc[field]; ok {
			if embSlice, ok := emb.([]any); ok {
				embedding32 := make([]float32, len(embSlice))
				for i, v := range embSlice {
					if f, ok := v.(float64); ok {
						embedding32[i] = float32(f)
					}
				}
				doc[field] = embedding32
			}
		}
	}
}

// Delete deletes a document by ID
func (s *OpenSearchStore) Delete(ctx context.Context, id string) error {
	_, err := s.client.Document.Delete(ctx, opensearchapi.DocumentDeleteReq{
		Index:      s.indexName,
		DocumentID: id,
		Params:     opensearchapi.DocumentDeleteParams{Refresh: "true"},
	})
	if err != nil {
		return fmt.Errorf("failed to delete document: %w", err)
	}
	return nil
}

// DeleteByQuery deletes documents matching the filters
func (s *OpenSearchStore) DeleteByQuery(ctx context.Context, filters map[string]any) (int, error) {
	var filterClauses []map[string]any
	filterClauses = append(filterClauses, map[string]any{"term": map[string]any{"status": StatusActive}})

	for field, value := range filters {
		filterClauses = append(filterClauses, map[string]any{"term": map[string]any{field: value}})
	}

	query := map[string]any{
		"query": map[string]any{"bool": map[string]any{"filter": filterClauses}},
	}

	queryBody, _ := json.Marshal(query)
	resp, err := s.client.Document.DeleteByQuery(ctx, opensearchapi.DocumentDeleteByQueryReq{
		Indices: []string{s.indexName},
		Body:    bytes.NewReader(queryBody),
		Params:  opensearchapi.DocumentDeleteByQueryParams{Refresh: opensearchapi.ToPointer(true)},
	})
	if err != nil {
		return 0, fmt.Errorf("delete by query failed: %w", err)
	}

	return resp.Deleted, nil
}

// Count counts documents matching the filters
func (s *OpenSearchStore) Count(ctx context.Context, filters map[string]any) (int, error) {
	var filterClauses []map[string]any
	filterClauses = append(filterClauses, map[string]any{"term": map[string]any{"status": StatusActive}})

	for field, value := range filters {
		filterClauses = append(filterClauses, map[string]any{"term": map[string]any{field: value}})
	}

	query := map[string]any{
		"query": map[string]any{"bool": map[string]any{"filter": filterClauses}},
	}

	queryBody, _ := json.Marshal(query)
	resp, err := s.client.Search(ctx, &opensearchapi.SearchReq{
		Indices: []string{s.indexName},
		Body:    bytes.NewReader(queryBody),
		Params: opensearchapi.SearchParams{
			Size:           opensearchapi.ToPointer(0),
			TrackTotalHits: true,
		},
	})
	if err != nil {
		return 0, fmt.Errorf("count failed: %w", err)
	}

	return resp.Hits.Total.Value, nil
}

// Update updates a document (upsert)
func (s *OpenSearchStore) Update(ctx context.Context, id string, doc map[string]any) error {
	return s.Store(ctx, id, doc)
}

// UpdateFields updates specific fields using a script
func (s *OpenSearchStore) UpdateFields(ctx context.Context, id string, fields map[string]any) error {
	// Build script source and params
	var scriptParts []string
	params := make(map[string]any)

	for field, value := range fields {
		paramName := "p_" + field
		scriptParts = append(scriptParts, fmt.Sprintf("ctx._source.%s = params.%s", field, paramName))
		params[paramName] = value
	}

	updateScript := map[string]any{
		"script": map[string]any{
			"source": joinStrings(scriptParts, "; "),
			"params": params,
		},
	}

	updateBody, _ := json.Marshal(updateScript)
	_, err := s.client.Update(ctx, opensearchapi.UpdateReq{
		Index:      s.indexName,
		DocumentID: id,
		Body:       bytes.NewReader(updateBody),
	})
	if err != nil {
		return fmt.Errorf("update fields failed: %w", err)
	}

	return nil
}

// joinStrings joins strings with separator
func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += sep + strs[i]
	}
	return result
}

// Close closes the OpenSearch connection
func (s *OpenSearchStore) Close() error {
	return nil
}
