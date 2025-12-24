package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestConstants(t *testing.T) {
	t.Run("document type constants", func(t *testing.T) {
		assert.Equal(t, "summary", DocTypeSummary)
		assert.Equal(t, "event", DocTypeEvent)
	})

	t.Run("memory type constants", func(t *testing.T) {
		assert.Equal(t, "fact", MemoryTypeFact)
		assert.Equal(t, "working", MemoryTypeWorking)
	})

	t.Run("relation type constants", func(t *testing.T) {
		assert.Equal(t, "causal", RelationCausal)
		assert.Equal(t, "temporal", RelationTemporal)
	})

	t.Run("role constants", func(t *testing.T) {
		assert.Equal(t, "user", RoleUser)
		assert.Equal(t, "assistant", RoleAssistant)
		assert.Equal(t, "system", RoleSystem)
	})
}

func TestShortTermMemory(t *testing.T) {
	t.Run("create short term memory", func(t *testing.T) {
		now := time.Now()
		stm := ShortTermMemory{
			AgentID:   "agent_1",
			UserID:    "user_1",
			SessionID: "session_1",
			Messages: Messages{
				{Role: "user", Content: "Hello", Name: "张三"},
			},
			UpdatedAt: now,
		}

		assert.Equal(t, "agent_1", stm.AgentID)
		assert.Equal(t, "user_1", stm.UserID)
		assert.Equal(t, "session_1", stm.SessionID)
		assert.Equal(t, 1, len(stm.Messages))
		assert.Equal(t, now, stm.UpdatedAt)
	})
}

func TestSummaryMemory(t *testing.T) {
	t.Run("create fact memory", func(t *testing.T) {
		now := time.Now()
		sm := SummaryMemory{
			ID:          "sum_123",
			AgentID:     "agent_1",
			UserID:      "user_1",
			Content:     "用户是一名程序员",
			MemoryType:  MemoryTypeFact,
			Importance:  0.9,
			Keywords:    []string{"职业", "程序员"},
			IsProtected: true,
			CreatedAt:   now,
			UpdatedAt:   now,
		}

		assert.Equal(t, "sum_123", sm.ID)
		assert.Equal(t, MemoryTypeFact, sm.MemoryType)
		assert.Equal(t, 0.9, sm.Importance)
		assert.True(t, sm.IsProtected)
		assert.Equal(t, 2, len(sm.Keywords))
		assert.Nil(t, sm.ExpiredAt)
	})

	t.Run("create working memory", func(t *testing.T) {
		sm := SummaryMemory{
			ID:         "sum_456",
			Content:    "用户正在讨论旅行计划",
			MemoryType: MemoryTypeWorking,
			Importance: 0.5,
		}

		assert.Equal(t, MemoryTypeWorking, sm.MemoryType)
		assert.False(t, sm.IsProtected)
	})

	t.Run("summary with embedding", func(t *testing.T) {
		sm := SummaryMemory{
			ID:        "sum_789",
			Content:   "test",
			Embedding: []float32{0.1, 0.2, 0.3},
		}

		assert.Equal(t, 3, len(sm.Embedding))
		assert.Equal(t, float32(0.1), sm.Embedding[0])
	})

	t.Run("summary with expired_at", func(t *testing.T) {
		now := time.Now()
		sm := SummaryMemory{
			ID:        "sum_expired",
			Content:   "旧的事实",
			ExpiredAt: &now,
		}

		assert.NotNil(t, sm.ExpiredAt)
	})

	t.Run("summary with access stats", func(t *testing.T) {
		now := time.Now()
		sm := SummaryMemory{
			ID:             "sum_stats",
			Content:        "frequently accessed",
			AccessCount:    10,
			LastAccessedAt: now,
		}

		assert.Equal(t, 10, sm.AccessCount)
		assert.Equal(t, now, sm.LastAccessedAt)
	})
}

func TestEventTriplet(t *testing.T) {
	t.Run("create event triplet", func(t *testing.T) {
		now := time.Now()
		et := EventTriplet{
			ID:          "evt_123",
			AgentID:     "agent_1",
			UserID:      "user_1",
			TriggerWord: "买了",
			Argument1:   "用户",
			Argument2:   "手机",
			CreatedAt:   now,
		}

		assert.Equal(t, "evt_123", et.ID)
		assert.Equal(t, "买了", et.TriggerWord)
		assert.Equal(t, "用户", et.Argument1)
		assert.Equal(t, "手机", et.Argument2)
	})

	t.Run("event with trigger embedding", func(t *testing.T) {
		et := EventTriplet{
			ID:               "evt_456",
			TriggerWord:      "去了",
			TriggerEmbedding: []float32{0.1, 0.2, 0.3, 0.4},
		}

		assert.Equal(t, 4, len(et.TriggerEmbedding))
	})

	t.Run("event with access stats", func(t *testing.T) {
		now := time.Now()
		et := EventTriplet{
			ID:             "evt_789",
			TriggerWord:    "见了",
			AccessCount:    5,
			LastAccessedAt: now,
		}

		assert.Equal(t, 5, et.AccessCount)
		assert.Equal(t, now, et.LastAccessedAt)
	})
}

func TestEventRelation(t *testing.T) {
	t.Run("create causal relation", func(t *testing.T) {
		now := time.Now()
		rel := EventRelation{
			ID:           "rel_123",
			RelationType: RelationCausal,
			FromEventID:  "evt_1",
			ToEventID:    "evt_2",
			CreatedAt:    now,
		}

		assert.Equal(t, RelationCausal, rel.RelationType)
		assert.Equal(t, "evt_1", rel.FromEventID)
		assert.Equal(t, "evt_2", rel.ToEventID)
	})

	t.Run("create temporal relation", func(t *testing.T) {
		rel := EventRelation{
			ID:           "rel_456",
			RelationType: RelationTemporal,
			FromEventID:  "evt_3",
			ToEventID:    "evt_4",
		}

		assert.Equal(t, RelationTemporal, rel.RelationType)
	})
}

func TestMessage(t *testing.T) {
	t.Run("user message", func(t *testing.T) {
		msg := Message{
			Role:    "user",
			Content: "今天心情不错",
			Name:    "阿信",
		}

		assert.Equal(t, "user", msg.Role)
		assert.Equal(t, "今天心情不错", msg.Content)
		assert.Equal(t, "阿信", msg.Name)
	})

	t.Run("assistant message", func(t *testing.T) {
		msg := Message{
			Role:    "assistant",
			Content: "很高兴听到这个消息！",
			Name:    "贾维斯",
		}

		assert.Equal(t, "assistant", msg.Role)
	})

	t.Run("message without name", func(t *testing.T) {
		msg := Message{
			Role:    "user",
			Content: "Hello",
		}

		assert.Empty(t, msg.Name)
	})
}

func TestMessages(t *testing.T) {
	t.Run("UserName with name", func(t *testing.T) {
		msgs := Messages{
			{Role: "user", Content: "Hello", Name: "阿信"},
			{Role: "assistant", Content: "Hi!", Name: "贾维斯"},
		}
		assert.Equal(t, "阿信", msgs.UserName())
	})

	t.Run("UserName without name", func(t *testing.T) {
		msgs := Messages{
			{Role: "user", Content: "Hello"},
			{Role: "assistant", Content: "Hi!"},
		}
		assert.Equal(t, "user", msgs.UserName())
	})

	t.Run("UserName no user message", func(t *testing.T) {
		msgs := Messages{
			{Role: "assistant", Content: "Hi!", Name: "贾维斯"},
		}
		assert.Equal(t, "user", msgs.UserName())
	})

	t.Run("AssistantName with name", func(t *testing.T) {
		msgs := Messages{
			{Role: "user", Content: "Hello", Name: "阿信"},
			{Role: "assistant", Content: "Hi!", Name: "贾维斯"},
		}
		assert.Equal(t, "贾维斯", msgs.AssistantName())
	})

	t.Run("AssistantName without name", func(t *testing.T) {
		msgs := Messages{
			{Role: "user", Content: "Hello"},
			{Role: "assistant", Content: "Hi!"},
		}
		assert.Equal(t, "assistant", msgs.AssistantName())
	})

	t.Run("AssistantName no assistant message", func(t *testing.T) {
		msgs := Messages{
			{Role: "user", Content: "Hello", Name: "阿信"},
		}
		assert.Equal(t, "assistant", msgs.AssistantName())
	})

	t.Run("Format with names", func(t *testing.T) {
		msgs := Messages{
			{Role: "user", Content: "Hello", Name: "阿信"},
			{Role: "assistant", Content: "Hi!", Name: "贾维斯"},
		}
		formatted := msgs.Format()
		assert.Contains(t, formatted, "阿信: Hello")
		assert.Contains(t, formatted, "贾维斯: Hi!")
	})

	t.Run("Format without names", func(t *testing.T) {
		msgs := Messages{
			{Role: "user", Content: "Hello"},
			{Role: "assistant", Content: "Hi!"},
		}
		formatted := msgs.Format()
		assert.Contains(t, formatted, "user: Hello")
		assert.Contains(t, formatted, "assistant: Hi!")
	})
}

func TestAddRequest(t *testing.T) {
	req := AddRequest{
		AgentID:   "agent_1",
		UserID:    "user_1",
		SessionID: "session_1",
		Messages: []Message{
			{Role: "user", Content: "我叫张三", Name: "张三"},
			{Role: "assistant", Content: "你好张三！", Name: "贾维斯"},
		},
	}

	assert.Equal(t, "agent_1", req.AgentID)
	assert.Equal(t, "session_1", req.SessionID)
	assert.Equal(t, 2, len(req.Messages))
	assert.Equal(t, "张三", req.Messages[0].Name)
	assert.Equal(t, "贾维斯", req.Messages[1].Name)
}

func TestRetrieveRequest(t *testing.T) {
	t.Run("with all options", func(t *testing.T) {
		req := RetrieveRequest{
			AgentID:   "agent_1",
			UserID:    "user_1",
			SessionID: "session_1",
			Query:     "用户的职业是什么",
			Limit:     5,
			Options: RetrieveOptions{
				MaxTokens:  3000,
				MaxFacts:   1500,
				MaxGraph:   600,
				MaxWorking: 900,
			},
		}

		assert.Equal(t, 3000, req.Options.MaxTokens)
		assert.Equal(t, 1500, req.Options.MaxFacts)
		assert.Equal(t, 600, req.Options.MaxGraph)
		assert.Equal(t, 900, req.Options.MaxWorking)
	})

	t.Run("minimal request", func(t *testing.T) {
		req := RetrieveRequest{
			AgentID: "agent_1",
			UserID:  "user_1",
			Query:   "用户喜欢什么",
		}

		assert.Equal(t, 0, req.Limit)
		assert.Equal(t, 0, req.Options.MaxTokens)
	})
}

func TestAddResponse(t *testing.T) {
	resp := AddResponse{
		Success: true,
		Summaries: []SummaryMemory{
			{ID: "sum_1", Content: "用户是程序员", MemoryType: MemoryTypeFact},
			{ID: "sum_2", Content: "正在讨论工作", MemoryType: MemoryTypeWorking},
		},
		Events: []EventTriplet{
			{ID: "evt_1", TriggerWord: "做", Argument1: "用户", Argument2: "后端开发"},
		},
		EventRelations: []EventRelation{
			{ID: "rel_1", RelationType: RelationCausal, FromEventID: "evt_1", ToEventID: "evt_2"},
		},
	}

	assert.True(t, resp.Success)
	assert.Equal(t, 2, len(resp.Summaries))
	assert.Equal(t, 1, len(resp.Events))
	assert.Equal(t, 1, len(resp.EventRelations))
}

func TestRetrieveResponse(t *testing.T) {
	resp := RetrieveResponse{
		Success: true,
		Facts: []SummaryMemory{
			{ID: "f_1", Content: "用户喜欢咖啡"},
		},
		WorkingMem: []SummaryMemory{
			{ID: "w_1", Content: "正在讨论旅行"},
		},
		Events: []EventTriplet{
			{ID: "e_1", TriggerWord: "去了", Argument1: "用户", Argument2: "北京"},
		},
		ShortTerm: Messages{
			{Role: "user", Content: "hello"},
		},
		Total:         4,
		MemoryContext: "## 用户事实\n- 用户喜欢咖啡",
	}

	assert.True(t, resp.Success)
	assert.Equal(t, 1, len(resp.Facts))
	assert.Equal(t, 1, len(resp.WorkingMem))
	assert.Equal(t, 1, len(resp.Events))
	assert.Equal(t, 1, len(resp.ShortTerm))
	assert.Equal(t, 4, resp.Total)
	assert.Contains(t, resp.MemoryContext, "用户喜欢咖啡")
}

func TestForgetRequest(t *testing.T) {
	req := ForgetRequest{
		AgentID: "agent_1",
		UserID:  "user_1",
	}

	assert.Equal(t, "agent_1", req.AgentID)
	assert.Equal(t, "user_1", req.UserID)
}

func TestForgetResponse(t *testing.T) {
	resp := ForgetResponse{
		Success:       true,
		WorkingForgot: 5,
		EventsForgot:  3,
		FactsExpired:  2,
	}

	assert.True(t, resp.Success)
	assert.Equal(t, 5, resp.WorkingForgot)
	assert.Equal(t, 3, resp.EventsForgot)
	assert.Equal(t, 2, resp.FactsExpired)
}
