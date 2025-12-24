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

	t.Run("add summaries", func(t *testing.T) {
		ctx := NewAddContext(context.Background(), "agent_1", "user_1", "session_1")

		assert.Empty(t, ctx.Summaries)

		ctx.AddSummaries(SummaryMemory{ID: "sum_1", Content: "test1", MemoryType: MemoryTypeFact})
		ctx.AddSummaries(SummaryMemory{ID: "sum_2", Content: "test2", MemoryType: MemoryTypeWorking})
		assert.Equal(t, 2, len(ctx.Summaries))
	})

	t.Run("add events", func(t *testing.T) {
		ctx := NewAddContext(context.Background(), "agent_1", "user_1", "session_1")

		assert.Empty(t, ctx.Events)

		ctx.AddEvents(EventTriplet{ID: "evt_1", TriggerWord: "买了", Argument1: "用户", Argument2: "手机"})
		ctx.AddEvents(EventTriplet{ID: "evt_2", TriggerWord: "去了", Argument1: "用户", Argument2: "北京"})
		assert.Equal(t, 2, len(ctx.Events))
	})

	t.Run("add event relations", func(t *testing.T) {
		ctx := NewAddContext(context.Background(), "agent_1", "user_1", "session_1")

		assert.Empty(t, ctx.EventRelations)

		ctx.AddEventRelations(EventRelation{ID: "rel_1", RelationType: RelationCausal, FromEventID: "evt_1", ToEventID: "evt_2"})
		ctx.AddEventRelations(EventRelation{ID: "rel_2", RelationType: RelationTemporal, FromEventID: "evt_1", ToEventID: "evt_2"})
		assert.Equal(t, 2, len(ctx.EventRelations))
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
				MaxFacts:   500,
				MaxGraph:   200,
				MaxWorking: 300,
			},
		}

		ctx := NewRecallContext(context.Background(), req)

		assert.Equal(t, "agent_1", ctx.AgentID)
		assert.Equal(t, "user_1", ctx.UserID)
		assert.Equal(t, "session_1", ctx.SessionID)
		assert.Equal(t, "用户的职业", ctx.Query)
		assert.Equal(t, 10, ctx.Limit)
		assert.Equal(t, 500, ctx.Options.MaxFacts)
		assert.Equal(t, 200, ctx.Options.MaxGraph)
		assert.Equal(t, 300, ctx.Options.MaxWorking)
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
		ctx.Facts = []SummaryMemory{{ID: "f_1"}, {ID: "f_2"}}
		ctx.WorkingMem = []SummaryMemory{{ID: "w_1"}}
		ctx.Events = []EventTriplet{{ID: "e_1"}, {ID: "e_2"}, {ID: "e_3"}}
		ctx.ShortTerm = Messages{{Role: "user", Content: "hi"}}

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

	t.Run("handler can add summaries", func(t *testing.T) {
		chain := NewActionChain()

		chain.Use(newMockAddAction(func(c *AddContext) {
			c.AddSummaries(SummaryMemory{ID: "sum_1", Content: "from handler 1"})
			c.Next()
		}))
		chain.Use(newMockAddAction(func(c *AddContext) {
			c.AddSummaries(SummaryMemory{ID: "sum_2", Content: "from handler 2"})
		}))

		ctx := NewAddContext(context.Background(), "agent_1", "user_1", "session_1")
		chain.Run(ctx)

		assert.Equal(t, 2, len(ctx.Summaries))
		assert.Equal(t, "sum_1", ctx.Summaries[0].ID)
		assert.Equal(t, "sum_2", ctx.Summaries[1].ID)
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
			order = append(order, "facts")
			c.Facts = []SummaryMemory{{ID: "f_1"}}
			c.Next()
		}))
		chain.Use(newMockRecallAction(func(c *RecallContext) {
			order = append(order, "working")
			c.WorkingMem = []SummaryMemory{{ID: "w_1"}}
			c.Next()
		}))
		chain.Use(newMockRecallAction(func(c *RecallContext) {
			order = append(order, "events")
			c.Events = []EventTriplet{{ID: "e_1"}}
		}))

		req := &RetrieveRequest{
			AgentID: "agent_1",
			UserID:  "user_1",
			Query:   "test query",
		}
		ctx := NewRecallContext(context.Background(), req)
		chain.Run(ctx)

		assert.Equal(t, []string{"facts", "working", "events"}, order)
		assert.Equal(t, 1, len(ctx.Facts))
		assert.Equal(t, 1, len(ctx.WorkingMem))
		assert.Equal(t, 1, len(ctx.Events))
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
