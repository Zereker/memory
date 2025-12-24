package genkit

import (
	"context"
	"testing"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMockPlugin_DefaultBehavior(t *testing.T) {
	ctx := context.Background()

	// 使用默认配置初始化 mock plugin
	mockPlugin := InitForTest(ctx, DefaultMockConfig(), "")

	g := Genkit()
	require.NotNil(t, g)

	// 测试 embedder - 使用 genkit.LookupEmbedder
	embedder := genkit.LookupEmbedder(g, "mock/test-embedding")
	require.NotNil(t, embedder, "mock embedder should be registered")

	resp, err := embedder.Embed(ctx, &ai.EmbedRequest{
		Input: []*ai.Document{ai.DocumentFromText("hello", nil)},
	})
	require.NoError(t, err)
	assert.Len(t, resp.Embeddings, 1)
	assert.Len(t, resp.Embeddings[0].Embedding, 1536) // default dim

	// 测试 model - 使用 genkit.LookupModel
	model := genkit.LookupModel(g, "mock/test-llm")
	require.NotNil(t, model, "mock model should be registered")

	modelResp, err := model.Generate(ctx, &ai.ModelRequest{
		Messages: []*ai.Message{ai.NewUserTextMessage("hello")},
	}, nil)
	require.NoError(t, err)
	assert.Equal(t, "hello", modelResp.Text()) // echo behavior

	_ = mockPlugin // 可用于配置自定义响应
}

func TestMockPlugin_CustomResponses(t *testing.T) {
	ctx := context.Background()

	// 初始化
	mockPlugin := InitForTest(ctx, DefaultMockConfig(), "")

	// 配置自定义 embedder 响应
	customVector := []float32{0.1, 0.2, 0.3}
	mockPlugin.SetEmbedderVectorResponse("test-embedding", customVector)

	// 配置自定义 model JSON 响应
	mockPlugin.SetModelJSONResponse("test-llm", map[string]any{
		"topic":        "测试主题",
		"significance": 5,
	})

	g := Genkit()

	// 验证自定义 embedder 响应
	embedder := genkit.LookupEmbedder(g, "mock/test-embedding")
	resp, err := embedder.Embed(ctx, &ai.EmbedRequest{
		Input: []*ai.Document{ai.DocumentFromText("test", nil)},
	})
	require.NoError(t, err)
	assert.Equal(t, customVector, resp.Embeddings[0].Embedding)

	// 验证自定义 model 响应
	model := genkit.LookupModel(g, "mock/test-llm")
	modelResp, err := model.Generate(ctx, &ai.ModelRequest{
		Messages: []*ai.Message{ai.NewUserTextMessage("test")},
	}, nil)
	require.NoError(t, err)
	assert.Contains(t, modelResp.Text(), "测试主题")
}

func TestMockPlugin_Embed(t *testing.T) {
	ctx := context.Background()

	// 使用 genkit.Embed 函数测试
	_ = InitForTest(ctx, DefaultMockConfig(), "")
	g := Genkit()

	resp, err := genkit.Embed(ctx, g, ai.WithEmbedderName("mock/test-embedding"), ai.WithTextDocs("hello world"))
	require.NoError(t, err)
	assert.Len(t, resp.Embeddings, 1)
}
