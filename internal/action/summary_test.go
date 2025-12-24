package action

import (
	"context"
	"testing"

	"github.com/Zereker/memory/internal/domain"
)

// TestSummaryAction 单元测试
func TestSummaryAction(t *testing.T) {
	newContext := func() *domain.AddContext {
		return domain.NewAddContext(context.Background(), "agent_test", "user_test", "session_summary_test")
	}

	run := func(c *domain.AddContext) {
		chain := domain.NewActionChain()
		chain.Use(NewSummaryAction())
		chain.Run(c)
	}

	t.Run("NoUserEpisode", func(t *testing.T) {
		c := newContext()
		c.Episodes = []domain.Episode{
			{
				ID:      "ep_test_1",
				Role:    domain.RoleAssistant,
				Name:    "AI助手",
				Content: "你好！",
			},
		}

		run(c)

		if c.Error() != nil {
			t.Fatalf("不应该有错误: %v", c.Error())
		}

		if len(c.Summaries) != 0 {
			t.Error("没有 user episode 时不应该生成 summary")
		}
	})

	t.Run("NoHistoricalEpisode", func(t *testing.T) {
		c := newContext()
		c.Episodes = []domain.Episode{
			{
				ID:             "ep_test_2",
				Role:           domain.RoleUser,
				Name:           "用户",
				Content:        "第一条消息",
				Topic:          "问候",
				TopicEmbedding: make([]float32, 4096),
			},
		}

		run(c)

		if c.Error() != nil {
			t.Fatalf("不应该有错误: %v", c.Error())
		}

		if len(c.Summaries) != 0 {
			t.Error("没有历史 episode 时不应该生成 summary")
		}
	})

	t.Run("MissingTopicEmbedding", func(t *testing.T) {
		c := newContext()
		c.Episodes = []domain.Episode{
			{
				ID:      "ep_test_3",
				Role:    domain.RoleUser,
				Name:    "用户",
				Content: "没有 topic embedding",
				Topic:   "测试",
			},
		}

		run(c)

		if c.Error() != nil {
			t.Fatalf("不应该有错误: %v", c.Error())
		}

		if len(c.Summaries) != 0 {
			t.Error("缺少 topic embedding 时不应该生成 summary")
		}
	})
}

// TestFormatEpisodes 测试 formatEpisodes 方法
func TestFormatEpisodes(t *testing.T) {
	action := NewSummaryAction()

	t.Run("WithNames", func(t *testing.T) {
		episodes := []domain.Episode{
			{Role: domain.RoleUser, Name: "小明", Content: "你好"},
			{Role: domain.RoleAssistant, Name: "AI助手", Content: "你好！有什么可以帮你的？"},
			{Role: domain.RoleUser, Name: "小明", Content: "我想了解一下产品"},
		}

		result := action.formatEpisodes(episodes)
		expected := "小明: 你好\nAI助手: 你好！有什么可以帮你的？\n小明: 我想了解一下产品"

		if result != expected {
			t.Errorf("格式化结果不匹配\n期望: %s\n实际: %s", expected, result)
		}
	})

	t.Run("WithoutNames", func(t *testing.T) {
		episodes := []domain.Episode{
			{Role: domain.RoleUser, Content: "你好"},
			{Role: domain.RoleAssistant, Content: "你好！"},
		}

		result := action.formatEpisodes(episodes)
		expected := "user: 你好\nassistant: 你好！"

		if result != expected {
			t.Errorf("格式化结果不匹配\n期望: %s\n实际: %s", expected, result)
		}
	})

	t.Run("Empty", func(t *testing.T) {
		result := action.formatEpisodes(nil)
		if result != "" {
			t.Errorf("空 episodes 应该返回空字符串，实际: %s", result)
		}
	})
}
