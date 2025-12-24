package action

import (
	"context"

	"github.com/Zereker/memory/pkg/vector"
)

// MockVectorStore 用于测试的向量存储 mock
// 实现 vector.Store 接口
type MockVectorStore struct {
	StoreFunc  func(ctx context.Context, id string, doc map[string]any) error
	SearchFunc func(ctx context.Context, query vector.SearchQuery) ([]map[string]any, error)

	StoreCalls  []struct{ ID string; Doc map[string]any }
	SearchCalls []vector.SearchQuery
}

func NewMockVectorStore() *MockVectorStore {
	return &MockVectorStore{
		StoreFunc: func(ctx context.Context, id string, doc map[string]any) error {
			return nil
		},
		SearchFunc: func(ctx context.Context, query vector.SearchQuery) ([]map[string]any, error) {
			return nil, nil
		},
	}
}

func (m *MockVectorStore) Store(ctx context.Context, id string, doc map[string]any) error {
	m.StoreCalls = append(m.StoreCalls, struct{ ID string; Doc map[string]any }{id, doc})
	return m.StoreFunc(ctx, id, doc)
}

func (m *MockVectorStore) Get(ctx context.Context, id string) (map[string]any, error) {
	return nil, nil
}

func (m *MockVectorStore) Search(ctx context.Context, query vector.SearchQuery) ([]map[string]any, error) {
	m.SearchCalls = append(m.SearchCalls, query)
	return m.SearchFunc(ctx, query)
}

func (m *MockVectorStore) Delete(ctx context.Context, id string) error {
	return nil
}

func (m *MockVectorStore) DeleteByQuery(ctx context.Context, filters map[string]any) (int, error) {
	return 0, nil
}

func (m *MockVectorStore) Count(ctx context.Context, filters map[string]any) (int, error) {
	return 0, nil
}

func (m *MockVectorStore) Update(ctx context.Context, id string, doc map[string]any) error {
	return nil
}

func (m *MockVectorStore) UpdateFields(ctx context.Context, id string, fields map[string]any) error {
	return nil
}

func (m *MockVectorStore) Close() error {
	return nil
}

// MockGraphStore 用于测试的图存储 mock
// 实现 graph.Store 接口
type MockGraphStore struct {
	MergeNodeFunc          func(ctx context.Context, labels []string, mergeKey string, mergeValue any, properties map[string]any) error
	CreateRelationshipFunc func(ctx context.Context, fromLabel, fromKey string, fromValue any, toLabel, toKey string, toValue any, relType string, properties map[string]any) error
	TraverseFunc           func(ctx context.Context, startLabel, startKey string, startValue any, relTypes []string, direction string, maxDepth, limit int) ([]map[string]any, error)

	MergeNodeCalls          []map[string]any
	CreateRelationshipCalls []map[string]any
	TraverseCalls           []map[string]any
}

func NewMockGraphStore() *MockGraphStore {
	return &MockGraphStore{
		MergeNodeFunc: func(ctx context.Context, labels []string, mergeKey string, mergeValue any, properties map[string]any) error {
			return nil
		},
		CreateRelationshipFunc: func(ctx context.Context, fromLabel, fromKey string, fromValue any, toLabel, toKey string, toValue any, relType string, properties map[string]any) error {
			return nil
		},
		TraverseFunc: func(ctx context.Context, startLabel, startKey string, startValue any, relTypes []string, direction string, maxDepth, limit int) ([]map[string]any, error) {
			return nil, nil
		},
	}
}

func (m *MockGraphStore) MergeNode(ctx context.Context, labels []string, mergeKey string, mergeValue any, properties map[string]any) error {
	m.MergeNodeCalls = append(m.MergeNodeCalls, map[string]any{
		"labels":     labels,
		"mergeKey":   mergeKey,
		"mergeValue": mergeValue,
		"properties": properties,
	})
	return m.MergeNodeFunc(ctx, labels, mergeKey, mergeValue, properties)
}

func (m *MockGraphStore) CreateRelationship(ctx context.Context, fromLabel, fromKey string, fromValue any, toLabel, toKey string, toValue any, relType string, properties map[string]any) error {
	m.CreateRelationshipCalls = append(m.CreateRelationshipCalls, map[string]any{
		"fromLabel":  fromLabel,
		"fromKey":    fromKey,
		"fromValue":  fromValue,
		"toLabel":    toLabel,
		"toKey":      toKey,
		"toValue":    toValue,
		"relType":    relType,
		"properties": properties,
	})
	return m.CreateRelationshipFunc(ctx, fromLabel, fromKey, fromValue, toLabel, toKey, toValue, relType, properties)
}

func (m *MockGraphStore) Traverse(ctx context.Context, startLabel, startKey string, startValue any, relTypes []string, direction string, maxDepth, limit int) ([]map[string]any, error) {
	m.TraverseCalls = append(m.TraverseCalls, map[string]any{
		"startLabel": startLabel,
		"startKey":   startKey,
		"startValue": startValue,
		"relTypes":   relTypes,
		"direction":  direction,
		"maxDepth":   maxDepth,
		"limit":      limit,
	})
	return m.TraverseFunc(ctx, startLabel, startKey, startValue, relTypes, direction, maxDepth, limit)
}
