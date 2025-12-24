package action

import (
	"context"

	"github.com/Zereker/memory/pkg/relation"
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

func (m *MockVectorStore) Search(ctx context.Context, query vector.SearchQuery) ([]map[string]any, error) {
	m.SearchCalls = append(m.SearchCalls, query)
	return m.SearchFunc(ctx, query)
}

// MockRelationStore 用于测试的关系存储 mock
// 实现 relation.Store 接口
type MockRelationStore struct {
	CreateRelationFunc  func(ctx context.Context, rel relation.Relation) error
	DeleteByEventIDFunc func(ctx context.Context, eventID string) error

	CreateRelationCalls  []relation.Relation
	DeleteByEventIDCalls []string
}

func NewMockRelationStore() *MockRelationStore {
	return &MockRelationStore{
		CreateRelationFunc: func(ctx context.Context, rel relation.Relation) error {
			return nil
		},
		DeleteByEventIDFunc: func(ctx context.Context, eventID string) error {
			return nil
		},
	}
}

func (m *MockRelationStore) CreateRelation(ctx context.Context, rel relation.Relation) error {
	m.CreateRelationCalls = append(m.CreateRelationCalls, rel)
	return m.CreateRelationFunc(ctx, rel)
}

func (m *MockRelationStore) DeleteByEventID(ctx context.Context, eventID string) error {
	m.DeleteByEventIDCalls = append(m.DeleteByEventIDCalls, eventID)
	return m.DeleteByEventIDFunc(ctx, eventID)
}

func (m *MockRelationStore) Close(_ context.Context) error {
	return nil
}
