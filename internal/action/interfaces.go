package action

import (
	"context"

	"github.com/Zereker/memory/internal/domain"
	"github.com/Zereker/memory/pkg/storage"
)

// LLMClient 用于测试注入的 LLM 客户端接口
// 推荐使用 pkg/genkit.MockPlugin 替代此接口
type LLMClient interface {
	// GenEmbedding 生成文本的向量表示
	GenEmbedding(ctx context.Context, embedderName, text string) ([]float32, error)

	// Generate 调用 LLM 生成内容
	Generate(c *domain.AddContext, promptName string, input map[string]any, output any) error
}

// VectorStore 定义向量存储接口
type VectorStore interface {
	// Store 存储文档
	Store(ctx context.Context, id string, doc map[string]any) error
}

// VectorSearchStore 定义向量搜索存储接口（用于检索）
type VectorSearchStore interface {
	VectorStore
	// Search 向量搜索
	Search(ctx context.Context, query storage.SearchQuery) ([]map[string]any, error)
}

// GraphSearchStore 定义图搜索存储接口（用于检索）
type GraphSearchStore interface {
	GraphStore
	// Traverse 图遍历
	Traverse(ctx context.Context, startLabel, startKey string, startValue any, relTypes []string, direction string, maxDepth, limit int) ([]map[string]any, error)
}

// GraphStore 定义图存储接口
type GraphStore interface {
	// MergeNode 创建或更新节点
	MergeNode(ctx context.Context, labels []string, mergeKey string, mergeValue any, properties map[string]any) error

	// CreateRelationship 创建关系
	CreateRelationship(ctx context.Context, fromLabel, fromKey string, fromValue any, toLabel, toKey string, toValue any, relType string, properties map[string]any) error
}
