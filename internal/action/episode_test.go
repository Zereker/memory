package action

import (
	"context"
	"testing"

	"github.com/Zereker/memory/internal/domain"
	"github.com/Zereker/memory/pkg/storage"
)

func TestEpisodeStorageAction(t *testing.T) {
	newContext := func(ctx context.Context) *domain.AddContext {
		return domain.NewAddContext(ctx, "agent_test", "user_test", "session_test")
	}

	run := func(c *domain.AddContext) {
		chain := domain.NewActionChain()
		chain.Use(NewEpisodeStorageAction())
		chain.Run(c)
	}

	t.Run("Success", func(t *testing.T) {
		c := newContext(context.Background())
		c.Messages = domain.Messages{
			{Role: domain.RoleUser, Name: "小明", Content: "我今天去了北京"},
			{Role: domain.RoleAssistant, Name: "AI助手", Content: "北京是个好地方，你去了哪些景点？"},
		}

		run(c)

		if c.Error() != nil {
			t.Fatalf("不应该有错误: %v", c.Error())
		}

		if len(c.Episodes) != 2 {
			t.Fatalf("期望 2 个 episodes，实际 %d", len(c.Episodes))
		}

		for i, ep := range c.Episodes {
			if len(ep.Embedding) == 0 {
				t.Errorf("Episode %d embedding 为空", i)
			}

			// 验证 topic 和 topic_embedding
			if ep.Topic == "" {
				t.Errorf("Episode %d topic 为空", i)
			} else {
				t.Logf("Episode %d topic: %s", i, ep.Topic)
			}

			if len(ep.TopicEmbedding) == 0 {
				t.Errorf("Episode %d topic_embedding 为空", i)
			} else {
				t.Logf("Episode %d topic_embedding 长度: %d", i, len(ep.TopicEmbedding))
			}
		}

		// 验证 OpenSearch 中的数据
		store := storage.NewStore()
		if store != nil {
			for _, ep := range c.Episodes {
				doc, err := store.Get(c.Context, ep.ID)
				if err != nil {
					t.Errorf("查询 episode 失败: id=%s, error=%v", ep.ID, err)
					continue
				}

				if doc == nil {
					t.Errorf("episode 未存入 OpenSearch: id=%s", ep.ID)
					continue
				}

				// 验证存储与查询数据一致
				if doc["topic"] != ep.Topic {
					t.Errorf("topic 不一致: 期望 %s, 实际 %v", ep.Topic, doc["topic"])
				}

				if doc["content"] != ep.Content {
					t.Errorf("content 不一致: 期望 %s, 实际 %v", ep.Content, doc["content"])
				}

				if doc["role"] != ep.Role {
					t.Errorf("role 不一致: 期望 %s, 实际 %v", ep.Role, doc["role"])
				}

				t.Logf("OpenSearch Episode: id=%s, role=%v, topic=%v", doc["id"], doc["role"], doc["topic"])
			}
		}
	})

	t.Run("EmptyMessages", func(t *testing.T) {
		c := newContext(context.Background())

		run(c)

		if c.Error() != nil {
			t.Errorf("不应该有错误: %v", c.Error())
		}

		if len(c.Episodes) != 0 {
			t.Errorf("期望 0 个 episodes，实际 %d", len(c.Episodes))
		}
	})

	t.Run("EmbeddingError", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		c := newContext(ctx)
		c.Messages = domain.Messages{{Role: domain.RoleUser, Content: "测试"}}

		run(c)

		if c.Error() == nil {
			t.Error("期望有错误")
		}

		if !c.IsAborted() {
			t.Error("链应该被中断")
		}

		if len(c.Episodes) != 0 {
			t.Errorf("期望 0 个 episodes，实际 %d", len(c.Episodes))
		}
	})
}
