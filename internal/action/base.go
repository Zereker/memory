package action

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"reflect"
	"time"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
	"github.com/mitchellh/mapstructure"

	"github.com/Zereker/memory/internal/domain"
	pkggenkit "github.com/Zereker/memory/pkg/genkit"
)

const (
	EmbedderName = "ark/doubao-embedding-text-240715"
)

// BaseAction 提供 Action 的公共能力
type BaseAction struct {
	name   string
	logger *slog.Logger
	g      *genkit.Genkit // 公开以便子类访问
}

// NewBaseAction 创建 BaseAction
func NewBaseAction(name string) *BaseAction {
	return &BaseAction{
		name:   name,
		logger: slog.Default().With("module", name),
		g:      pkggenkit.Genkit(),
	}
}

// GenEmbedding 生成文本的向量表示
func (b *BaseAction) GenEmbedding(ctx context.Context, embedderName, text string) ([]float32, error) {
	resp, err := genkit.Embed(ctx, b.g, ai.WithEmbedderName(embedderName), ai.WithTextDocs(text))
	if err != nil {
		return nil, err
	}

	if len(resp.Embeddings) == 0 || len(resp.Embeddings[0].Embedding) == 0 {
		return nil, fmt.Errorf("empty embedding response")
	}

	return resp.Embeddings[0].Embedding, nil
}

// Generate 调用 LLM 生成内容
func (b *BaseAction) Generate(c *domain.AddContext, promptName string, input map[string]any, output any) error {
	prompt := genkit.LookupPrompt(b.g, promptName)
	if prompt == nil {
		return fmt.Errorf("prompt not found: %s", promptName)
	}

	resp, err := prompt.Execute(c.Context, ai.WithInput(input))
	if err != nil {
		return fmt.Errorf("prompt execute failed: %w", err)
	}

	if resp == nil {
		return fmt.Errorf("empty response")
	}

	// 记录 token 使用量
	if resp.Usage != nil {
		c.AddTokenUsage(b.name, resp.Usage.InputTokens, resp.Usage.OutputTokens)
		b.logger.Debug("llm response",
			"prompt", promptName,
			"input_tokens", resp.Usage.InputTokens,
			"output_tokens", resp.Usage.OutputTokens,
		)
	}

	if err := resp.Output(output); err != nil {
		return fmt.Errorf("parse output failed: %w", err)
	}

	return nil
}

// CosineSimilarity 计算两个向量的余弦相似度
func (b *BaseAction) CosineSimilarity(vec1, vec2 []float32) float64 {
	if len(vec1) != len(vec2) || len(vec1) == 0 {
		return 0
	}

	var dotProduct, normA, normB float64
	for i := range vec1 {
		dotProduct += float64(vec1[i]) * float64(vec2[i])
		normA += float64(vec1[i]) * float64(vec1[i])
		normB += float64(vec2[i]) * float64(vec2[i])
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}

// DocToEpisode 将 map 转换为 Episode
func (b *BaseAction) DocToEpisode(doc map[string]any) *domain.Episode {
	var ep domain.Episode

	config := &mapstructure.DecoderConfig{
		Result:           &ep,
		TagName:          "json",
		WeaklyTypedInput: true,
		DecodeHook:       mapstructure.ComposeDecodeHookFunc(b.float32SliceHook, b.timeHook),
	}

	decoder, err := mapstructure.NewDecoder(config)
	if err != nil {
		b.logger.Error("failed to create decoder", "error", err)
		return &domain.Episode{}
	}

	if err := decoder.Decode(doc); err != nil {
		b.logger.Error("failed to decode doc to episode", "error", err)
		return &domain.Episode{}
	}

	return &ep
}

// float32SliceHook 处理 []any/[]float32 -> []float32 转换
func (b *BaseAction) float32SliceHook(_, to reflect.Type, data any) (any, error) {
	if to != reflect.TypeOf([]float32{}) {
		return data, nil
	}

	// 已经是 []float32，直接返回
	if f32Slice, ok := data.([]float32); ok {
		return f32Slice, nil
	}

	// []any -> []float32
	slice, ok := data.([]any)
	if !ok {
		return data, nil
	}

	result := make([]float32, len(slice))
	for i, v := range slice {
		if f, ok := v.(float64); ok {
			result[i] = float32(f)
		}
	}

	return result, nil
}

// DocToSummary 将 map 转换为 Summary
func (b *BaseAction) DocToSummary(doc map[string]any) *domain.Summary {
	var s domain.Summary

	config := &mapstructure.DecoderConfig{
		Result:           &s,
		TagName:          "json",
		WeaklyTypedInput: true,
		DecodeHook:       mapstructure.ComposeDecodeHookFunc(b.float32SliceHook, b.timeHook, b.stringSliceHook),
	}

	decoder, err := mapstructure.NewDecoder(config)
	if err != nil {
		b.logger.Error("failed to create decoder", "error", err)
		return &domain.Summary{}
	}

	if err := decoder.Decode(doc); err != nil {
		b.logger.Error("failed to decode doc to summary", "error", err)
		return &domain.Summary{}
	}

	return &s
}

// DocToEdge 将 map 转换为 Edge
func (b *BaseAction) DocToEdge(doc map[string]any) *domain.Edge {
	var e domain.Edge

	config := &mapstructure.DecoderConfig{
		Result:           &e,
		TagName:          "json",
		WeaklyTypedInput: true,
		DecodeHook:       mapstructure.ComposeDecodeHookFunc(b.float32SliceHook, b.timeHook, b.stringSliceHook),
	}

	decoder, err := mapstructure.NewDecoder(config)
	if err != nil {
		b.logger.Error("failed to create decoder", "error", err)
		return &domain.Edge{}
	}

	if err := decoder.Decode(doc); err != nil {
		b.logger.Error("failed to decode doc to edge", "error", err)
		return &domain.Edge{}
	}

	return &e
}

// DocToEntity 将 map 转换为 Entity
func (b *BaseAction) DocToEntity(doc map[string]any) *domain.Entity {
	var entity domain.Entity

	// 处理 entity_type -> type 的映射（避免与 DocType 冲突）
	if entityType, ok := doc["entity_type"].(string); ok {
		doc["type"] = entityType
	}

	config := &mapstructure.DecoderConfig{
		Result:           &entity,
		TagName:          "json",
		WeaklyTypedInput: true,
		DecodeHook:       mapstructure.ComposeDecodeHookFunc(b.float32SliceHook, b.timeHook),
	}

	decoder, err := mapstructure.NewDecoder(config)
	if err != nil {
		b.logger.Error("failed to create decoder", "error", err)
		return &domain.Entity{}
	}

	if err := decoder.Decode(doc); err != nil {
		b.logger.Error("failed to decode doc to entity", "error", err)
		return &domain.Entity{}
	}

	return &entity
}

// stringSliceHook 处理 []any -> []string 转换
func (b *BaseAction) stringSliceHook(_, to reflect.Type, data any) (any, error) {
	if to != reflect.TypeOf([]string{}) {
		return data, nil
	}

	// 已经是 []string，直接返回
	if strSlice, ok := data.([]string); ok {
		return strSlice, nil
	}

	// []any -> []string
	slice, ok := data.([]any)
	if !ok {
		return data, nil
	}

	result := make([]string, 0, len(slice))
	for _, v := range slice {
		if s, ok := v.(string); ok {
			result = append(result, s)
		}
	}

	return result, nil
}

// timeHook 处理 string -> time.Time 转换
func (b *BaseAction) timeHook(_, to reflect.Type, data any) (any, error) {
	if to != reflect.TypeOf(time.Time{}) {
		return data, nil
	}

	// 已经是 time.Time，直接返回
	if t, ok := data.(time.Time); ok {
		return t, nil
	}

	// string -> time.Time
	str, ok := data.(string)
	if !ok {
		return data, nil
	}

	// 尝试多种时间格式
	formats := []string{
		time.RFC3339,
		time.RFC3339Nano,
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, str); err == nil {
			return t, nil
		}
	}

	return data, fmt.Errorf("unable to parse time: %s", str)
}
