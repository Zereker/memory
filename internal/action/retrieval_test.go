package action

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/Zereker/memory/internal/domain"
)

// TestRetrievalAction 单元测试
func TestRetrievalAction(t *testing.T) {
	t.Run("Name", func(t *testing.T) {
		action := NewRetrievalAction()
		assert.Equal(t, "retrieval", action.Name())
	})

	t.Run("DefaultLimit", func(t *testing.T) {
		req := &domain.RetrieveRequest{
			AgentID: "agent_test",
			UserID:  "user_test",
			Query:   "test",
			Limit:   0, // 未设置
		}
		c := domain.NewRecallContext(context.Background(), req)

		// 默认 limit 应该是 10
		assert.Equal(t, 10, c.Limit)
	})

	t.Run("CustomLimit", func(t *testing.T) {
		req := &domain.RetrieveRequest{
			AgentID: "agent_test",
			UserID:  "user_test",
			Query:   "test",
			Limit:   20,
		}
		c := domain.NewRecallContext(context.Background(), req)

		assert.Equal(t, 20, c.Limit)
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

		assert.Contains(t, result, "对话摘要")
		assert.Contains(t, result, "用户信息")
		assert.Contains(t, result, "相关对话记录")
		assert.Contains(t, result, "提及的实体")
	})

	t.Run("Empty", func(t *testing.T) {
		req := &domain.RetrieveRequest{
			AgentID: "agent_test",
			UserID:  "user_test",
			Query:   "不存在的信息",
		}
		c := domain.NewRecallContext(context.Background(), req)

		result := FormatMemoryContext(c)

		assert.Contains(t, result, "没有找到")
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

		assert.Contains(t, result, "相关对话记录")
		assert.NotContains(t, result, "对话摘要") // 无 Summary
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

		assert.Contains(t, result, "对话摘要")
		assert.Contains(t, result, "[测试]")
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

		assert.Contains(t, result, "没有主题的摘要")
		assert.NotContains(t, result, "[]")
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

		assert.Contains(t, result, "[user]")
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

		assert.Contains(t, result, "小明: 产品经理")
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

		assert.Contains(t, result, "北京 (place)")
	})
}

// TestBudgetCalculation 测试 token 预算计算
func TestBudgetCalculation(t *testing.T) {
	t.Run("EstimateTokens", func(t *testing.T) {
		// 中文约 1.5 字符/token
		text := "这是一个测试字符串"
		chars := len([]rune(text))
		expectedTokens := float64(chars) / CharsPerToken

		// 验证常量
		assert.Equal(t, 1.5, CharsPerToken)
		assert.Greater(t, expectedTokens, 0.0)
	})

	t.Run("DefaultMaxTokens", func(t *testing.T) {
		assert.Equal(t, 2000, DefaultMaxTokens)
		assert.Equal(t, 3, DefaultMaxSummaries)
		assert.Equal(t, 10, DefaultMaxEdges)
		assert.Equal(t, 5, DefaultMaxEntities)
		assert.Equal(t, 5, DefaultMaxEpisodes)
	})
}

// TestFormatMemoryContextIntegration 测试完整格式化场景
func TestFormatMemoryContextIntegration(t *testing.T) {
	req := &domain.RetrieveRequest{
		AgentID: "agent_test",
		UserID:  "user_test",
		Query:   "工作和爱好",
	}
	c := domain.NewRecallContext(context.Background(), req)

	// 添加多种类型的数据
	c.Summaries = []domain.Summary{
		{Topic: "工作", Content: "用户在科技公司担任工程师"},
		{Topic: "爱好", Content: "用户喜欢跑步和阅读"},
	}
	c.Entities = []domain.Entity{
		{Name: "张三", Type: domain.EntityTypePerson, Description: "科技公司工程师"},
		{Name: "科技公司", Type: domain.EntityTypeThing},
	}
	c.Edges = []domain.Edge{
		{Fact: "张三在科技公司工作"},
	}
	c.Episodes = []domain.Episode{
		{Role: domain.RoleUser, Name: "张三", Content: "我每天早上去跑步"},
	}

	result := FormatMemoryContext(c)

	// 验证所有部分都存在
	assert.Contains(t, result, "对话摘要")
	assert.Contains(t, result, "[工作]")
	assert.Contains(t, result, "[爱好]")
	assert.Contains(t, result, "用户信息")
	assert.Contains(t, result, "张三在科技公司工作")
	assert.Contains(t, result, "提及的实体")
	assert.Contains(t, result, "相关对话记录")
	assert.Contains(t, result, "[张三]")

	// 验证格式正确
	lines := strings.Split(result, "\n")
	assert.Greater(t, len(lines), 5) // 应该有多行输出
}
