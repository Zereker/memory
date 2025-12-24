package action

import (
	"context"
	"testing"

	"github.com/Zereker/memory/internal/domain"
	"github.com/Zereker/memory/pkg/graph"
)

func TestExtractionAction(t *testing.T) {
	// 公共辅助函数
	newContext := func() *domain.AddContext {
		return domain.NewAddContext(context.Background(), "agent_test", "user_test", "session_test")
	}

	run := func(c *domain.AddContext) {
		chain := domain.NewActionChain()
		chain.Use(NewExtractionAction())
		chain.Run(c)
	}

	t.Run("Success", func(t *testing.T) {
		c := newContext()
		c.Messages = domain.Messages{
			{Role: "user", Name: "小明", Content: "我叫小明，在北京做产品经理，我女朋友叫小红"},
			{Role: "assistant", Name: "AI助手", Content: "你好小明！产品经理是个很有挑战的职业，小红也在北京吗？"},
		}

		run(c)

		if c.Error() != nil {
			t.Fatalf("执行失败: %v", c.Error())
		}

		t.Logf("提取到 %d 个实体，%d 条关系", len(c.Entities), len(c.Edges))

		if len(c.Entities) == 0 {
			t.Fatal("没有提取到实体")
		}

		for _, entity := range c.Entities {
			t.Logf("Entity: name=%s, type=%s, desc=%s", entity.Name, entity.Type, entity.Description)
		}

		for _, edge := range c.Edges {
			t.Logf("Edge: %s -[%s]-> %s, fact=%s", edge.SourceID, edge.Relation, edge.TargetID, edge.Fact)
		}

		// 验证 Neo4j 中的数据
		store := graph.NewStore()
		if store != nil {
			// 验证实体是否存入 Neo4j
			for _, entity := range c.Entities {
				node, err := store.GetNode(c.Context, LabelEntity, "name", entity.Name)
				if err != nil {
					t.Errorf("查询实体失败: name=%s, error=%v", entity.Name, err)
					continue
				}

				if node == nil {
					t.Errorf("实体未存入 Neo4j: name=%s", entity.Name)
					continue
				}

				t.Logf("Neo4j Entity: name=%s, type=%v", node["name"], node["type"])
			}

			// 验证关系是否存入 Neo4j
			for _, entity := range c.Entities {
				rels, err := store.FindRelationships(c.Context, LabelEntity, "name", entity.Name, "", 10)
				if err != nil {
					t.Errorf("查询关系失败: name=%s, error=%v", entity.Name, err)
					continue
				}

				for _, rel := range rels {
					t.Logf("Neo4j Relation: %v", rel)
				}
			}
		}

		usage := c.TotalTokenUsage()
		t.Logf("Token 使用量: input=%d, output=%d", usage.InputTokens, usage.OutputTokens)
	})

	t.Run("EmptyMessages", func(t *testing.T) {
		c := newContext()

		run(c)

		if c.Error() != nil {
			t.Fatalf("执行失败: %v", c.Error())
		}

		if len(c.Entities) != 0 || len(c.Edges) != 0 {
			t.Error("空消息应该不产生实体和关系")
		}
	})
}
