package action

import (
	"context"
	"strings"
	"testing"

	"github.com/Zereker/memory/internal/domain"
)

// TestRetrievalAction 单元测试
func TestRetrievalAction(t *testing.T) {
	t.Run("Name", func(t *testing.T) {
		action := NewRetrievalAction()
		if action.Name() != "retrieval" {
			t.Errorf("期望 name=retrieval, 实际 %s", action.Name())
		}
	})

	t.Run("EmptyQuery", func(t *testing.T) {
		req := &domain.RetrieveRequest{
			AgentID: "agent_test",
			UserID:  "user_test",
			Query:   "",
		}
		c := domain.NewRecallContext(context.Background(), req)

		chain := domain.NewRecallChain()
		chain.Use(NewRetrievalAction())
		chain.Run(c)

		// 空查询应该正常处理（可能生成空 embedding）
		if c.Error() != nil {
			t.Logf("空查询产生错误（预期行为）: %v", c.Error())
		}
	})

	t.Run("DefaultLimit", func(t *testing.T) {
		req := &domain.RetrieveRequest{
			AgentID: "agent_test",
			UserID:  "user_test",
			Query:   "test",
			Limit:   0, // 未设置
		}
		c := domain.NewRecallContext(context.Background(), req)

		if c.Limit != 10 {
			t.Errorf("默认 limit 应该是 10, 实际 %d", c.Limit)
		}
	})
}

// TestFormatMemoryContext 测试记忆上下文格式化
func TestFormatMemoryContext(t *testing.T) {
	t.Run("AllTypes", func(t *testing.T) {
		req := &domain.RetrieveRequest{
			AgentID: "agent_test",
			UserID:  "user_test",
			Query:   "用户的职业",
		}
		c := domain.NewRecallContext(context.Background(), req)

		c.Episodes = []domain.Episode{
			{Role: domain.RoleUser, Name: "小明", Content: "我在北京做产品经理"},
		}
		c.Entities = []domain.Entity{
			{Name: "小明", Type: domain.EntityTypePerson, Description: "用户"},
			{Name: "北京", Type: domain.EntityTypePlace, Description: "工作地点"},
		}
		c.Edges = []domain.Edge{
			{Fact: "小明是产品经理"},
			{Fact: "小明在北京工作"},
		}
		c.Summaries = []domain.Summary{
			{Topic: "职业", Content: "用户是一名在北京工作的产品经理"},
		}

		result := FormatMemoryContext(c)
		t.Logf("格式化结果:\n%s", result)

		if !strings.Contains(result, "对话摘要") {
			t.Error("结果应包含对话摘要")
		}
		if !strings.Contains(result, "用户信息") {
			t.Error("结果应包含用户信息")
		}
		if !strings.Contains(result, "相关对话记录") {
			t.Error("结果应包含相关对话记录")
		}
		if !strings.Contains(result, "提及的实体") {
			t.Error("结果应包含提及的实体")
		}
	})

	t.Run("Empty", func(t *testing.T) {
		req := &domain.RetrieveRequest{
			AgentID: "agent_test",
			UserID:  "user_test",
			Query:   "不存在的信息",
		}
		c := domain.NewRecallContext(context.Background(), req)

		result := FormatMemoryContext(c)

		if !strings.Contains(result, "没有找到") {
			t.Error("空结果应提示未找到")
		}
	})

	t.Run("OnlyEpisodes", func(t *testing.T) {
		req := &domain.RetrieveRequest{
			AgentID: "agent_test",
			UserID:  "user_test",
			Query:   "test",
		}
		c := domain.NewRecallContext(context.Background(), req)
		c.Episodes = []domain.Episode{
			{Role: domain.RoleUser, Name: "用户", Content: "测试内容"},
		}

		result := FormatMemoryContext(c)

		if !strings.Contains(result, "相关对话记录") {
			t.Error("应包含对话记录")
		}
		if strings.Contains(result, "对话摘要") {
			t.Error("不应包含对话摘要（无 Summary）")
		}
	})

	t.Run("OnlySummaries", func(t *testing.T) {
		req := &domain.RetrieveRequest{
			AgentID: "agent_test",
			UserID:  "user_test",
			Query:   "test",
		}
		c := domain.NewRecallContext(context.Background(), req)
		c.Summaries = []domain.Summary{
			{Topic: "测试", Content: "这是测试摘要"},
		}

		result := FormatMemoryContext(c)

		if !strings.Contains(result, "对话摘要") {
			t.Error("应包含对话摘要")
		}
		if !strings.Contains(result, "[测试]") {
			t.Error("应包含 topic 标签")
		}
	})

	t.Run("SummaryWithoutTopic", func(t *testing.T) {
		req := &domain.RetrieveRequest{
			AgentID: "agent_test",
			UserID:  "user_test",
			Query:   "test",
		}
		c := domain.NewRecallContext(context.Background(), req)
		c.Summaries = []domain.Summary{
			{Content: "没有主题的摘要"},
		}

		result := FormatMemoryContext(c)

		if !strings.Contains(result, "没有主题的摘要") {
			t.Error("应包含摘要内容")
		}
		if strings.Contains(result, "[]") {
			t.Error("无主题时不应显示空括号")
		}
	})

	t.Run("EpisodeWithoutName", func(t *testing.T) {
		req := &domain.RetrieveRequest{
			AgentID: "agent_test",
			UserID:  "user_test",
			Query:   "test",
		}
		c := domain.NewRecallContext(context.Background(), req)
		c.Episodes = []domain.Episode{
			{Role: domain.RoleUser, Content: "没有名字的消息"},
		}

		result := FormatMemoryContext(c)

		if !strings.Contains(result, "[user]") {
			t.Error("无名字时应使用 role")
		}
	})

	t.Run("EntityWithDescription", func(t *testing.T) {
		req := &domain.RetrieveRequest{
			AgentID: "agent_test",
			UserID:  "user_test",
			Query:   "test",
		}
		c := domain.NewRecallContext(context.Background(), req)
		c.Entities = []domain.Entity{
			{Name: "小明", Type: domain.EntityTypePerson, Description: "产品经理"},
		}

		result := FormatMemoryContext(c)

		if !strings.Contains(result, "小明: 产品经理") {
			t.Error("有描述时应显示 name: description")
		}
	})

	t.Run("EntityWithoutDescription", func(t *testing.T) {
		req := &domain.RetrieveRequest{
			AgentID: "agent_test",
			UserID:  "user_test",
			Query:   "test",
		}
		c := domain.NewRecallContext(context.Background(), req)
		c.Entities = []domain.Entity{
			{Name: "北京", Type: domain.EntityTypePlace},
		}

		result := FormatMemoryContext(c)

		if !strings.Contains(result, "北京 (place)") {
			t.Error("无描述时应显示 name (type)")
		}
	})
}
