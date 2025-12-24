package action

import (
	"context"
	"testing"

	"github.com/Zereker/memory/internal/domain"
	"github.com/Zereker/memory/pkg/storage"
)

// TestMemoryAdd 测试 Memory.Add 完整流程
func TestMemoryAdd(t *testing.T) {
	ctx := context.Background()

	c := domain.NewAddContext(ctx, "agent_test", "user_test", "session_test")
	c.Messages = domain.Messages{
		{Role: domain.RoleUser, Name: "小明", Content: "我叫小明，在北京做产品经理，我女朋友叫小红"},
		{Role: domain.RoleAssistant, Name: "AI助手", Content: "你好小明！产品经理是个很有挑战的职业，小红也在北京吗？"},
	}

	// 创建完整的 Add chain
	chain := domain.NewActionChain()
	chain.Use(NewEpisodeStorageAction())
	chain.Use(NewExtractionAction())
	chain.Use(NewSummaryAction())

	chain.Run(c)

	// 验证 Episodes
	if len(c.Episodes) != 2 {
		t.Fatalf("期望 2 个 episodes，实际 %d", len(c.Episodes))
	}

	// 验证 Entities
	t.Logf("提取到 %d 个实体", len(c.Entities))
	for _, entity := range c.Entities {
		t.Logf("  - %s (%s): %s", entity.Name, entity.Type, entity.Description)
	}

	// 验证 Edges
	t.Logf("提取到 %d 条关系", len(c.Edges))
	for _, edge := range c.Edges {
		t.Logf("  - %s", edge.Fact)
	}

	// 验证 token 使用量
	usage := c.TotalTokenUsage()
	if usage.InputTokens == 0 || usage.OutputTokens == 0 {
		t.Error("Token 使用量为 0")
	}

	t.Logf("总 Token 使用量: input=%d, output=%d", usage.InputTokens, usage.OutputTokens)
}

// TestMemoryAddWithTopicChange 测试主题变化时的摘要生成
func TestMemoryAddWithTopicChange(t *testing.T) {
	store := storage.NewStore()
	if store == nil {
		t.Skip("OpenSearch 不可用，跳过集成测试")
	}

	ctx := context.Background()
	sessionID := "session_topic_change_test"

	// 清理测试数据
	_, _ = store.DeleteByQuery(ctx, map[string]any{"session_id": sessionID})

	runChain := func(c *domain.AddContext) {
		chain := domain.NewActionChain()
		chain.Use(NewEpisodeStorageAction())
		chain.Use(NewSummaryAction())
		chain.Run(c)
	}

	// 第一轮对话：咖啡话题
	c1 := domain.NewAddContext(ctx, "agent_test", "user_test", sessionID)
	c1.Messages = domain.Messages{
		{Role: domain.RoleUser, Name: "用户", Content: "我特别喜欢喝咖啡，每天早上都要去星巴克买一杯拿铁"},
		{Role: domain.RoleAssistant, Name: "AI助手", Content: "拿铁确实很好喝，星巴克的咖啡品质也不错"},
	}

	runChain(c1)

	if c1.Error() != nil {
		t.Fatalf("第一轮对话失败: %v", c1.Error())
	}

	// 断言：第一轮应该存储 2 个 episodes
	if len(c1.Episodes) != 2 {
		t.Fatalf("第一轮对话应该有 2 个 episodes，实际 %d", len(c1.Episodes))
	}

	// 断言：Episode 必须有 topic 和 topic_embedding
	for _, ep := range c1.Episodes {
		if ep.Topic == "" {
			t.Error("Episode topic 不应该为空")
		}
		if len(ep.TopicEmbedding) == 0 {
			t.Error("Episode topic_embedding 不应该为空")
		}
	}

	// 断言：第一轮不应该生成 summary（没有历史主题可比较）
	if len(c1.Summaries) != 0 {
		t.Errorf("第一轮不应该生成 summary，实际生成了 %d 个", len(c1.Summaries))
	}

	// 第二轮对话：天气话题（主题变化）
	c2 := domain.NewAddContext(ctx, "agent_test", "user_test", sessionID)
	c2.Messages = domain.Messages{
		{Role: domain.RoleUser, Name: "用户", Content: "今天天气真好，适合出去散步"},
		{Role: domain.RoleAssistant, Name: "AI助手", Content: "是的，阳光明媚，温度也很舒适"},
	}

	runChain(c2)

	if c2.Error() != nil {
		t.Fatalf("第二轮对话失败: %v", c2.Error())
	}

	// 断言：第二轮应该存储 2 个 episodes
	if len(c2.Episodes) != 2 {
		t.Fatalf("第二轮对话应该有 2 个 episodes，实际 %d", len(c2.Episodes))
	}

	// 断言：第二轮应该检测到主题变化并生成 summary
	if len(c2.Summaries) == 0 {
		t.Error("第二轮应该检测到主题变化并生成 summary")
	}

	t.Logf("第一轮: episodes=%d, topic=%s", len(c1.Episodes), c1.Episodes[0].Topic)
	t.Logf("第二轮: episodes=%d, topic=%s, summaries=%d",
		len(c2.Episodes), c2.Episodes[0].Topic, len(c2.Summaries))
}

// TestMemoryAddSameTopic 测试相同主题时不生成摘要
func TestMemoryAddSameTopic(t *testing.T) {
	store := storage.NewStore()
	if store == nil {
		t.Skip("OpenSearch 不可用，跳过集成测试")
	}

	ctx := context.Background()
	sessionID := "session_same_topic_test"

	// 清理测试数据
	_, _ = store.DeleteByQuery(ctx, map[string]any{"session_id": sessionID})

	runChain := func(c *domain.AddContext) {
		chain := domain.NewActionChain()
		chain.Use(NewEpisodeStorageAction())
		chain.Use(NewSummaryAction())
		chain.Run(c)
	}

	// 第一轮对话：咖啡话题
	c1 := domain.NewAddContext(ctx, "agent_test", "user_test", sessionID)
	c1.Messages = domain.Messages{
		{Role: domain.RoleUser, Name: "用户", Content: "我喜欢喝拿铁咖啡"},
		{Role: domain.RoleAssistant, Name: "AI助手", Content: "拿铁咖啡很好喝"},
	}

	runChain(c1)

	if c1.Error() != nil {
		t.Fatalf("第一轮对话失败: %v", c1.Error())
	}

	if len(c1.Episodes) != 2 {
		t.Fatalf("第一轮对话应该有 2 个 episodes，实际 %d", len(c1.Episodes))
	}

	// 第二轮对话：仍然是咖啡话题（相似主题）
	c2 := domain.NewAddContext(ctx, "agent_test", "user_test", sessionID)
	c2.Messages = domain.Messages{
		{Role: domain.RoleUser, Name: "用户", Content: "我也喜欢喝拿铁咖啡"},
		{Role: domain.RoleAssistant, Name: "AI助手", Content: "拿铁咖啡确实不错"},
	}

	runChain(c2)

	if c2.Error() != nil {
		t.Fatalf("第二轮对话失败: %v", c2.Error())
	}

	if len(c2.Episodes) != 2 {
		t.Fatalf("第二轮对话应该有 2 个 episodes，实际 %d", len(c2.Episodes))
	}

	// 断言：相同主题不应该生成 summary
	if len(c2.Summaries) != 0 {
		t.Errorf("相同主题不应该生成 summary，实际生成了 %d 个", len(c2.Summaries))
	}

	t.Logf("第一轮: episodes=%d, topic=%s", len(c1.Episodes), c1.Episodes[0].Topic)
	t.Logf("第二轮: episodes=%d, topic=%s, summaries=%d (符合预期)",
		len(c2.Episodes), c2.Episodes[0].Topic, len(c2.Summaries))
}

// mockAddAction 用于测试的 mock action
type mockAddAction struct {
	name    string
	handler func(c *domain.AddContext)
}

func (m *mockAddAction) Name() string {
	return m.name
}

func (m *mockAddAction) Handle(c *domain.AddContext) {
	if m.handler != nil {
		m.handler(c)
	}
}

// TestActionChainAbort 测试 action chain 中断机制
func TestActionChainAbort(t *testing.T) {
	ctx := context.Background()

	executed := []string{}

	// 创建一个会中断的 action
	abortAction := &mockAddAction{
		name: "abort",
		handler: func(c *domain.AddContext) {
			executed = append(executed, "abort")
			c.Abort()
		},
	}

	// 创建一个正常的 action
	normalAction := &mockAddAction{
		name: "normal",
		handler: func(c *domain.AddContext) {
			executed = append(executed, "normal")
			c.Next()
		},
	}

	c := domain.NewAddContext(ctx, "agent_test", "user_test", "session_test")

	chain := domain.NewActionChain()
	chain.Use(abortAction)
	chain.Use(normalAction)
	chain.Run(c)

	// 验证只执行了第一个 action
	if len(executed) != 1 || executed[0] != "abort" {
		t.Errorf("期望只执行 abort，实际执行了: %v", executed)
	}

	if !c.IsAborted() {
		t.Error("context 应该被标记为 aborted")
	}
}
