package domain

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

// mockAddAction 用于测试的 AddAction 实现
type mockAddAction struct {
	name    string
	handler func(c *AddContext)
}

func (m *mockAddAction) Name() string {
	return m.name
}

func (m *mockAddAction) Handle(c *AddContext) {
	if m.handler != nil {
		m.handler(c)
	}
}

// newMockAddAction 创建测试用的 AddAction
func newMockAddAction(handler func(c *AddContext)) *mockAddAction {
	return &mockAddAction{
		name:    "mock",
		handler: handler,
	}
}

// mockRecallAction 用于测试的 RecallAction 实现
type mockRecallAction struct {
	name    string
	handler func(c *RecallContext)
}

func (m *mockRecallAction) Name() string {
	return m.name
}

func (m *mockRecallAction) HandleRecall(c *RecallContext) {
	if m.handler != nil {
		m.handler(c)
	}
}

// newMockRecallAction 创建测试用的 RecallAction
func newMockRecallAction(handler func(c *RecallContext)) *mockRecallAction {
	return &mockRecallAction{
		name:    "mock",
		handler: handler,
	}
}

func TestBaseContext(t *testing.T) {
	t.Run("set and get metadata", func(t *testing.T) {
		ctx := NewAddContext(context.Background(), "agent_1", "user_1", "session_1")

		ctx.Set("key1", "value1")
		ctx.Set("key2", 123)

		val1, ok1 := ctx.Get("key1")
		assert.True(t, ok1)
		assert.Equal(t, "value1", val1)

		val2, ok2 := ctx.Get("key2")
		assert.True(t, ok2)
		assert.Equal(t, 123, val2)

		_, ok3 := ctx.Get("nonexistent")
		assert.False(t, ok3)
	})

	t.Run("abort functionality", func(t *testing.T) {
		ctx := NewAddContext(context.Background(), "agent_1", "user_1", "session_1")

		assert.False(t, ctx.IsAborted())
		ctx.Abort()
		assert.True(t, ctx.IsAborted())
	})

	t.Run("add episodes", func(t *testing.T) {
		ctx := NewAddContext(context.Background(), "agent_1", "user_1", "session_1")

		assert.Empty(t, ctx.Episodes)

		ctx.AddEpisodes(Episode{ID: "ep_1", Content: "test1"})
		ctx.AddEpisodes(Episode{ID: "ep_2", Content: "test2"})
		assert.Equal(t, 2, len(ctx.Episodes))
	})

	t.Run("add entities", func(t *testing.T) {
		ctx := NewAddContext(context.Background(), "agent_1", "user_1", "session_1")

		assert.Empty(t, ctx.Entities)

		ctx.AddEntities(Entity{ID: "ent_1", Name: "test1"})
		ctx.AddEntities(Entity{ID: "ent_2", Name: "test2"})
		assert.Equal(t, 2, len(ctx.Entities))
	})

	t.Run("add edges", func(t *testing.T) {
		ctx := NewAddContext(context.Background(), "agent_1", "user_1", "session_1")

		assert.Empty(t, ctx.Edges)

		ctx.AddEdges(Edge{ID: "edge_1", Fact: "test1"})
		ctx.AddEdges(Edge{ID: "edge_2", Fact: "test2"})
		assert.Equal(t, 2, len(ctx.Edges))
	})

	t.Run("add communities", func(t *testing.T) {
		ctx := NewAddContext(context.Background(), "agent_1", "user_1", "session_1")

		assert.Empty(t, ctx.Communities)

		ctx.AddCommunities(Community{ID: "comm_1", Summary: "test1"})
		ctx.AddCommunities(Community{ID: "comm_2", Summary: "test2"})
		assert.Equal(t, 2, len(ctx.Communities))
	})
}

func TestAddContext(t *testing.T) {
	t.Run("creation", func(t *testing.T) {
		ctx := NewAddContext(context.Background(), "agent_1", "user_1", "session_1")

		assert.Equal(t, "agent_1", ctx.AgentID)
		assert.Equal(t, "user_1", ctx.UserID)
		assert.Equal(t, "session_1", ctx.SessionID)
		assert.Equal(t, "zh_CN", ctx.Language)
		assert.NotNil(t, ctx.Metadata)
	})

	t.Run("set messages and use helper methods", func(t *testing.T) {
		ctx := NewAddContext(context.Background(), "agent_1", "user_1", "session_1")
		ctx.Messages = Messages{
			{Role: "user", Content: "Hello", Name: "张三"},
			{Role: "assistant", Content: "Hi there", Name: "贾维斯"},
		}

		assert.Equal(t, "张三", ctx.Messages.UserName())
		assert.Equal(t, "贾维斯", ctx.Messages.AssistantName())
		assert.Contains(t, ctx.Messages.Format(), "张三: Hello")
	})

	t.Run("LanguageName with zh_CN", func(t *testing.T) {
		ctx := NewAddContext(context.Background(), "agent_1", "user_1", "session_1")
		assert.Equal(t, "中文", ctx.LanguageName())
	})

	t.Run("LanguageName with en_US", func(t *testing.T) {
		ctx := NewAddContext(context.Background(), "agent_1", "user_1", "session_1")
		ctx.Language = "en_US"
		assert.Equal(t, "English", ctx.LanguageName())
	})

	t.Run("LanguageName with unknown code defaults to Chinese", func(t *testing.T) {
		ctx := NewAddContext(context.Background(), "agent_1", "user_1", "session_1")
		ctx.Language = "fr_FR"
		assert.Equal(t, "中文", ctx.LanguageName())
	})
}

func TestRecallContext(t *testing.T) {
	t.Run("creation from retrieve request", func(t *testing.T) {
		req := &RetrieveRequest{
			AgentID:   "agent_1",
			UserID:    "user_1",
			SessionID: "session_1",
			Query:     "用户的职业",
			Limit:     10,
			Options: RetrieveOptions{
				IncludeEpisodes: true,
				IncludeEntities: true,
				MaxHops:         2,
			},
		}

		ctx := NewRecallContext(context.Background(), req)

		assert.Equal(t, "agent_1", ctx.AgentID)
		assert.Equal(t, "user_1", ctx.UserID)
		assert.Equal(t, "session_1", ctx.SessionID)
		assert.Equal(t, "用户的职业", ctx.Query)
		assert.Equal(t, 10, ctx.Limit)
		assert.True(t, ctx.Options.IncludeEpisodes)
		assert.True(t, ctx.Options.IncludeEntities)
		assert.Equal(t, 2, ctx.Options.MaxHops)
	})

	t.Run("default limit", func(t *testing.T) {
		req := &RetrieveRequest{
			AgentID: "agent_1",
			UserID:  "user_1",
			Query:   "test",
		}

		ctx := NewRecallContext(context.Background(), req)

		assert.Equal(t, 10, ctx.Limit) // default limit
	})

	t.Run("total results", func(t *testing.T) {
		req := &RetrieveRequest{
			AgentID: "agent_1",
			UserID:  "user_1",
			Query:   "test",
		}

		ctx := NewRecallContext(context.Background(), req)
		ctx.Episodes = []Episode{{ID: "ep_1"}, {ID: "ep_2"}}
		ctx.Entities = []Entity{{ID: "ent_1"}}
		ctx.Edges = []Edge{{ID: "edge_1"}, {ID: "edge_2"}, {ID: "edge_3"}}
		ctx.Communities = []Community{{ID: "comm_1"}}

		assert.Equal(t, 7, ctx.TotalResults())
	})
}

func TestActionChain(t *testing.T) {
	t.Run("empty chain", func(t *testing.T) {
		chain := NewActionChain()
		ctx := NewAddContext(context.Background(), "agent_1", "user_1", "session_1")

		chain.Run(ctx)

		// Should complete without panic
		assert.False(t, ctx.IsAborted())
	})

	t.Run("single handler", func(t *testing.T) {
		chain := NewActionChain()
		executed := false

		chain.Use(newMockAddAction(func(c *AddContext) {
			executed = true
			c.Set("executed", true)
		}))

		ctx := NewAddContext(context.Background(), "agent_1", "user_1", "session_1")
		chain.Run(ctx)

		assert.True(t, executed)
		val, ok := ctx.Get("executed")
		assert.True(t, ok)
		assert.True(t, val.(bool))
	})

	t.Run("multiple handlers in order", func(t *testing.T) {
		chain := NewActionChain()
		order := []int{}

		chain.Use(newMockAddAction(func(c *AddContext) {
			order = append(order, 1)
			c.Next()
		}))
		chain.Use(newMockAddAction(func(c *AddContext) {
			order = append(order, 2)
			c.Next()
		}))
		chain.Use(newMockAddAction(func(c *AddContext) {
			order = append(order, 3)
		}))

		ctx := NewAddContext(context.Background(), "agent_1", "user_1", "session_1")
		chain.Run(ctx)

		assert.Equal(t, []int{1, 2, 3}, order)
	})

	t.Run("abort stops chain", func(t *testing.T) {
		chain := NewActionChain()
		order := []int{}

		chain.Use(newMockAddAction(func(c *AddContext) {
			order = append(order, 1)
			c.Next()
		}))
		chain.Use(newMockAddAction(func(c *AddContext) {
			order = append(order, 2)
			c.Abort() // Abort here
			c.Next()
		}))
		chain.Use(newMockAddAction(func(c *AddContext) {
			order = append(order, 3) // Should not execute
		}))

		ctx := NewAddContext(context.Background(), "agent_1", "user_1", "session_1")
		chain.Run(ctx)

		assert.Equal(t, []int{1, 2}, order)
		assert.True(t, ctx.IsAborted())
	})

	t.Run("handler can add episodes", func(t *testing.T) {
		chain := NewActionChain()

		chain.Use(newMockAddAction(func(c *AddContext) {
			c.AddEpisodes(Episode{ID: "ep_1", Content: "from handler 1"})
			c.Next()
		}))
		chain.Use(newMockAddAction(func(c *AddContext) {
			c.AddEpisodes(Episode{ID: "ep_2", Content: "from handler 2"})
		}))

		ctx := NewAddContext(context.Background(), "agent_1", "user_1", "session_1")
		chain.Run(ctx)

		assert.Equal(t, 2, len(ctx.Episodes))
		assert.Equal(t, "ep_1", ctx.Episodes[0].ID)
		assert.Equal(t, "ep_2", ctx.Episodes[1].ID)
	})
}

func TestRecallChain(t *testing.T) {
	t.Run("empty chain", func(t *testing.T) {
		chain := NewRecallChain()
		req := &RetrieveRequest{
			AgentID: "agent_1",
			UserID:  "user_1",
			Query:   "test query",
		}
		ctx := NewRecallContext(context.Background(), req)

		chain.Run(ctx)

		assert.False(t, ctx.IsAborted())
	})

	t.Run("multiple handlers in order", func(t *testing.T) {
		chain := NewRecallChain()
		order := []string{}

		chain.Use(newMockRecallAction(func(c *RecallContext) {
			order = append(order, "episodes")
			c.Episodes = []Episode{{ID: "ep_1"}}
			c.Next()
		}))
		chain.Use(newMockRecallAction(func(c *RecallContext) {
			order = append(order, "entities")
			c.Entities = []Entity{{ID: "ent_1"}}
			c.Next()
		}))
		chain.Use(newMockRecallAction(func(c *RecallContext) {
			order = append(order, "edges")
			c.Edges = []Edge{{ID: "edge_1"}}
		}))

		req := &RetrieveRequest{
			AgentID: "agent_1",
			UserID:  "user_1",
			Query:   "test query",
		}
		ctx := NewRecallContext(context.Background(), req)
		chain.Run(ctx)

		assert.Equal(t, []string{"episodes", "entities", "edges"}, order)
		assert.Equal(t, 1, len(ctx.Episodes))
		assert.Equal(t, 1, len(ctx.Entities))
		assert.Equal(t, 1, len(ctx.Edges))
	})

	t.Run("abort stops chain", func(t *testing.T) {
		chain := NewRecallChain()
		order := []string{}

		chain.Use(newMockRecallAction(func(c *RecallContext) {
			order = append(order, "first")
			c.Abort()
			c.Next()
		}))
		chain.Use(newMockRecallAction(func(c *RecallContext) {
			order = append(order, "second") // Should not execute
		}))

		req := &RetrieveRequest{
			AgentID: "agent_1",
			UserID:  "user_1",
			Query:   "test query",
		}
		ctx := NewRecallContext(context.Background(), req)
		chain.Run(ctx)

		assert.Equal(t, []string{"first"}, order)
		assert.True(t, ctx.IsAborted())
	})
}

func TestChainUseMethod(t *testing.T) {
	t.Run("action chain Use returns self for chaining", func(t *testing.T) {
		chain := NewActionChain()

		result := chain.Use(newMockAddAction(func(c *AddContext) {}))

		assert.Same(t, chain, result)
	})

	t.Run("recall chain Use returns self for chaining", func(t *testing.T) {
		chain := NewRecallChain()

		result := chain.Use(newMockRecallAction(func(c *RecallContext) {}))

		assert.Same(t, chain, result)
	})

	t.Run("action chain fluent API", func(t *testing.T) {
		order := []int{}

		chain := NewActionChain().
			Use(newMockAddAction(func(c *AddContext) {
				order = append(order, 1)
				c.Next()
			})).
			Use(newMockAddAction(func(c *AddContext) {
				order = append(order, 2)
				c.Next()
			})).
			Use(newMockAddAction(func(c *AddContext) {
				order = append(order, 3)
			}))

		ctx := NewAddContext(context.Background(), "agent_1", "user_1", "session_1")
		chain.Run(ctx)

		assert.Equal(t, []int{1, 2, 3}, order)
	})
}

func TestTokenUsage(t *testing.T) {
	t.Run("add and get token usage", func(t *testing.T) {
		ctx := NewAddContext(context.Background(), "agent_1", "user_1", "session_1")

		ctx.AddTokenUsage("extraction", 100, 50)
		ctx.AddTokenUsage("extraction", 50, 25) // add more

		usage := ctx.GetTokenUsage("extraction")
		assert.Equal(t, 150, usage.InputTokens)
		assert.Equal(t, 75, usage.OutputTokens)
	})

	t.Run("total token usage", func(t *testing.T) {
		ctx := NewAddContext(context.Background(), "agent_1", "user_1", "session_1")

		ctx.AddTokenUsage("action1", 100, 50)
		ctx.AddTokenUsage("action2", 200, 100)

		total := ctx.TotalTokenUsage()
		assert.Equal(t, 300, total.InputTokens)
		assert.Equal(t, 150, total.OutputTokens)
	})
}
