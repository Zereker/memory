package action

import (
	"context"
	"log"
	"os"
	"testing"
	"time"

	"github.com/Zereker/memory/internal/domain"
	genkitpkg "github.com/Zereker/memory/pkg/genkit"
	"github.com/Zereker/memory/pkg/graph"
	"github.com/Zereker/memory/pkg/storage"
)

const (
	defaultModel     = "doubao-pro-32k"
	defaultEmbedding = "doubao-embedding-text-240715"
)

// testHelper 测试辅助结构，在 TestMain 中初始化
var (
	testHelper *BaseAction
)

// skipIntegrationTests 标记是否跳过集成测试
var skipIntegrationTests bool

// TestMain 统一初始化测试依赖，避免多次初始化
func TestMain(m *testing.M) {
	ctx := context.Background()

	// 从环境变量读取 API Key
	apiKey := os.Getenv("ARK_API_KEY")
	if apiKey == "" {
		log.Println("ARK_API_KEY not set, skipping integration tests")
		skipIntegrationTests = true
		os.Exit(0) // 跳过所有测试
	}

	// 初始化 Genkit：Ark 用于 LLM 和 Embedding
	err := genkitpkg.Init(ctx, genkitpkg.Config{
		Ark: genkitpkg.ArkConfig{
			APIKey:  apiKey,
			BaseURL: "https://ark.cn-beijing.volces.com/api/v3",
			Models: []genkitpkg.ModelConfig{
				{
					Name:  defaultModel,
					Type:  genkitpkg.ModelTypeLLM,
					Model: defaultModel,
				},
				{
					Name:  defaultEmbedding,
					Type:  genkitpkg.ModelTypeEmbedding,
					Model: defaultEmbedding,
					Dim:   2560,
				},
			},
		},
	})

	if err != nil {
		log.Fatalf("Failed to initialize genkit: %v", err)
	}

	// 初始化 OpenSearch 存储
	// 注意：运行测试前需先执行 make init INDEX=memories_test 初始化测试索引
	_ = storage.Init(storage.OpenSearchConfig{
		Addresses:    []string{"http://localhost:9200"},
		IndexName:    "memories_test",
		EmbeddingDim: 2560,
	})

	// 初始化 Neo4j 图存储
	_ = graph.Init(graph.Neo4jConfig{
		Enabled:  true,
		URI:      "bolt://localhost:7687",
		Username: "neo4j",
		Password: "YOUR_NEO4J_PASSWORD",
		Database: "neo4j",
	})

	// 初始化测试辅助结构
	testHelper = NewBaseAction("test")

	os.Exit(m.Run())
}

// TestEmbeddingGeneration 测试 embedding 生成
func TestEmbeddingGeneration(t *testing.T) {
	ctx := context.Background()

	embedding, err := testHelper.GenEmbedding(ctx, EmbedderName, "测试文本")
	if err != nil {
		t.Fatalf("生成 embedding 失败: %v", err)
	}

	t.Logf("Embedding 长度: %d", len(embedding))
	if len(embedding) == 0 {
		t.Fatal("Embedding 为空")
	}

	t.Logf("前5个值: %v", embedding[:5])
}

// TestEmbeddingSimilarity 测试 embedding 相似度
func TestEmbeddingSimilarity(t *testing.T) {
	ctx := context.Background()

	// 生成两个相似文本的 embedding
	emb1, err := testHelper.GenEmbedding(ctx, EmbedderName, "我喜欢喝咖啡")
	if err != nil {
		t.Fatalf("生成 embedding1 失败: %v", err)
	}

	emb2, err := testHelper.GenEmbedding(ctx, EmbedderName, "我爱喝咖啡")
	if err != nil {
		t.Fatalf("生成 embedding2 失败: %v", err)
	}

	// 生成一个不相关文本的 embedding
	emb3, err := testHelper.GenEmbedding(ctx, EmbedderName, "今天天气很好")
	if err != nil {
		t.Fatalf("生成 embedding3 失败: %v", err)
	}

	// 计算余弦相似度
	sim12 := testHelper.CosineSimilarity(emb1, emb2)
	sim13 := testHelper.CosineSimilarity(emb1, emb3)

	t.Logf("'喜欢喝咖啡' vs '爱喝咖啡' 相似度: %.4f", sim12)
	t.Logf("'喜欢喝咖啡' vs '今天天气很好' 相似度: %.4f", sim13)

	// 验证相似文本的相似度应该更高
	if sim12 <= sim13 {
		t.Errorf("相似文本的相似度应该更高: sim12=%.4f, sim13=%.4f", sim12, sim13)
	}
}

// TestDocToEpisode 测试 map 到 Episode 的转换
func TestDocToEpisode(t *testing.T) {
	tests := []struct {
		name     string
		doc      map[string]any
		validate func(*testing.T, *domain.Episode)
	}{
		{
			name: "基本字段转换",
			doc: map[string]any{
				"id":      "ep_test_123",
				"role":    "user",
				"name":    "小明",
				"content": "测试内容",
				"topic":   "测试主题",
			},
			validate: func(t *testing.T, ep *domain.Episode) {
				if ep.ID != "ep_test_123" {
					t.Errorf("ID 不匹配: 期望 ep_test_123, 实际 %s", ep.ID)
				}
				if ep.Role != "user" {
					t.Errorf("Role 不匹配: 期望 user, 实际 %s", ep.Role)
				}
				if ep.Name != "小明" {
					t.Errorf("Name 不匹配: 期望 小明, 实际 %s", ep.Name)
				}
				if ep.Content != "测试内容" {
					t.Errorf("Content 不匹配: 期望 测试内容, 实际 %s", ep.Content)
				}
				if ep.Topic != "测试主题" {
					t.Errorf("Topic 不匹配: 期望 测试主题, 实际 %s", ep.Topic)
				}
			},
		},
		{
			name: "时间字段转换 - RFC3339格式",
			doc: map[string]any{
				"id":         "ep_time_test",
				"created_at": "2024-01-15T10:30:00Z",
				"timestamp":  "2024-01-15T10:30:00Z",
			},
			validate: func(t *testing.T, ep *domain.Episode) {
				expected := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
				if !ep.CreatedAt.Equal(expected) {
					t.Errorf("CreatedAt 不匹配: 期望 %v, 实际 %v", expected, ep.CreatedAt)
				}
				if !ep.Timestamp.Equal(expected) {
					t.Errorf("Timestamp 不匹配: 期望 %v, 实际 %v", expected, ep.Timestamp)
				}
			},
		},
		{
			name: "时间字段转换 - RFC3339Nano格式",
			doc: map[string]any{
				"id":         "ep_time_nano",
				"created_at": "2024-01-15T10:30:00.123456789Z",
			},
			validate: func(t *testing.T, ep *domain.Episode) {
				if ep.CreatedAt.IsZero() {
					t.Error("CreatedAt 应该被解析")
				}
				if ep.CreatedAt.Year() != 2024 || ep.CreatedAt.Month() != 1 || ep.CreatedAt.Day() != 15 {
					t.Errorf("CreatedAt 日期不正确: %v", ep.CreatedAt)
				}
			},
		},
		{
			name: "embedding 字段转换 - []any",
			doc: map[string]any{
				"id":                "ep_emb_test",
				"content_embedding": []any{0.1, 0.2, 0.3},
				"topic_embedding":   []any{0.4, 0.5, 0.6},
			},
			validate: func(t *testing.T, ep *domain.Episode) {
				if len(ep.Embedding) != 3 {
					t.Errorf("Embedding 长度不匹配: 期望 3, 实际 %d", len(ep.Embedding))
				}
				if len(ep.TopicEmbedding) != 3 {
					t.Errorf("TopicEmbedding 长度不匹配: 期望 3, 实际 %d", len(ep.TopicEmbedding))
				}
			},
		},
		{
			name: "embedding 字段转换 - []float32",
			doc: map[string]any{
				"id":                "ep_emb_f32",
				"content_embedding": []float32{0.1, 0.2, 0.3},
				"topic_embedding":   []float32{0.4, 0.5, 0.6},
			},
			validate: func(t *testing.T, ep *domain.Episode) {
				if len(ep.Embedding) != 3 {
					t.Errorf("Embedding 长度不匹配: 期望 3, 实际 %d", len(ep.Embedding))
				}
				if len(ep.TopicEmbedding) != 3 {
					t.Errorf("TopicEmbedding 长度不匹配: 期望 3, 实际 %d", len(ep.TopicEmbedding))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ep := testHelper.DocToEpisode(tt.doc)
			tt.validate(t, ep)
		})
	}
}
