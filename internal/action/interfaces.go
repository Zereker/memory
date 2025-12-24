package action

import (
	"context"

	"github.com/Zereker/memory/internal/domain"
)

// LLMClient 定义 LLM 调用接口，用于依赖注入和测试 mock
type LLMClient interface {
	// GenEmbedding 生成文本的向量表示
	GenEmbedding(ctx context.Context, embedderName, text string) ([]float32, error)

	// Generate 调用 LLM 生成内容
	Generate(c *domain.AddContext, promptName string, input map[string]any, output any) error
}

// VectorStore 定义向量存储接口（用于测试 mock）
type VectorStore interface {
	// Store 存储文档
	Store(ctx context.Context, id string, doc map[string]any) error
}

// GraphStore 定义图存储接口（用于测试 mock）
type GraphStore interface {
	// MergeNode 创建或更新节点
	MergeNode(ctx context.Context, labels []string, mergeKey string, mergeValue any, properties map[string]any) error

	// CreateRelationship 创建关系
	CreateRelationship(ctx context.Context, fromLabel, fromKey string, fromValue any, toLabel, toKey string, toValue any, relType string, properties map[string]any) error
}
