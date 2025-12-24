package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestEntityTypes(t *testing.T) {
	t.Run("entity type constants", func(t *testing.T) {
		assert.Equal(t, EntityType("person"), EntityTypePerson)
		assert.Equal(t, EntityType("place"), EntityTypePlace)
		assert.Equal(t, EntityType("thing"), EntityTypeThing)
		assert.Equal(t, EntityType("event"), EntityTypeEvent)
		assert.Equal(t, EntityType("emotion"), EntityTypeEmotion)
		assert.Equal(t, EntityType("activity"), EntityTypeActivity)
	})
}

func TestEpisode(t *testing.T) {
	t.Run("create episode", func(t *testing.T) {
		now := time.Now()
		ep := Episode{
			ID:        "ep_123",
			AgentID:   "agent_1",
			UserID:    "user_1",
			SessionID: "session_1",
			Role:      "user",
			Name:      "阿信",
			Content:   "今天天气不错",
			Timestamp: now,
			CreatedAt: now,
		}

		assert.Equal(t, "ep_123", ep.ID)
		assert.Equal(t, "agent_1", ep.AgentID)
		assert.Equal(t, "user", ep.Role)
		assert.Equal(t, "阿信", ep.Name)
		assert.Equal(t, "今天天气不错", ep.Content)
	})

	t.Run("episode with embedding", func(t *testing.T) {
		ep := Episode{
			ID:        "ep_456",
			Content:   "测试内容",
			Embedding: []float32{0.1, 0.2, 0.3},
		}

		assert.Equal(t, 3, len(ep.Embedding))
		assert.Equal(t, float32(0.1), ep.Embedding[0])
	})
}

func TestEntity(t *testing.T) {
	t.Run("create entity", func(t *testing.T) {
		now := time.Now()
		entity := Entity{
			ID:          "ent_123",
			AgentID:     "agent_1",
			UserID:      "user_1",
			Name:        "张三",
			Type:        EntityTypePerson,
			Description: "用户的朋友",
			CreatedAt:   now,
			UpdatedAt:   now,
		}

		assert.Equal(t, "ent_123", entity.ID)
		assert.Equal(t, EntityTypePerson, entity.Type)
		assert.Equal(t, "张三", entity.Name)
		assert.Equal(t, "用户的朋友", entity.Description)
	})

	t.Run("entity with embedding", func(t *testing.T) {
		entity := Entity{
			ID:        "ent_456",
			Name:      "星巴克",
			Type:      EntityTypePlace,
			Embedding: []float32{0.1, 0.2, 0.3, 0.4},
		}

		assert.Equal(t, EntityTypePlace, entity.Type)
		assert.Equal(t, 4, len(entity.Embedding))
	})
}

func TestEdge(t *testing.T) {
	t.Run("create edge", func(t *testing.T) {
		now := time.Now()
		edge := Edge{
			ID:       "edge_123",
			SourceID: "ent_1",
			TargetID: "ent_2",
			Relation: "喜欢",
			Fact:     "用户喜欢喝咖啡",
		}
		edge.CreatedAt = now

		assert.Equal(t, "edge_123", edge.ID)
		assert.Equal(t, "ent_1", edge.SourceID)
		assert.Equal(t, "ent_2", edge.TargetID)
		assert.Equal(t, "喜欢", edge.Relation)
		assert.Equal(t, "用户喜欢喝咖啡", edge.Fact)
	})

	t.Run("edge with bi-temporal", func(t *testing.T) {
		now := time.Now()
		validAt := now.AddDate(-1, 0, 0) // 一年前生效
		edge := Edge{
			ID:       "edge_456",
			SourceID: "ent_1",
			TargetID: "ent_2",
			Relation: "住在",
			Fact:     "用户住在北京",
			ValidAt:  &validAt,
		}

		assert.NotNil(t, edge.ValidAt)
		assert.Nil(t, edge.InvalidAt)
	})

	t.Run("edge with episode ids", func(t *testing.T) {
		edge := Edge{
			ID:         "edge_789",
			SourceID:   "ent_1",
			TargetID:   "ent_2",
			Relation:   "认识",
			Fact:       "用户和张三是朋友",
			EpisodeIDs: []string{"ep_1", "ep_2", "ep_3"},
		}

		assert.Equal(t, 3, len(edge.EpisodeIDs))
		assert.Contains(t, edge.EpisodeIDs, "ep_1")
	})
}

func TestEdgeIsValid(t *testing.T) {
	t.Run("edge without time bounds is always valid", func(t *testing.T) {
		edge := Edge{ID: "edge_1", Relation: "test"}

		assert.True(t, edge.IsValid(time.Now()))
		assert.True(t, edge.IsValid(time.Now().AddDate(-10, 0, 0)))
		assert.True(t, edge.IsValid(time.Now().AddDate(10, 0, 0)))
	})

	t.Run("edge with validAt", func(t *testing.T) {
		validAt := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
		edge := Edge{ID: "edge_1", Relation: "test", ValidAt: &validAt}

		// Before validAt - not valid
		assert.False(t, edge.IsValid(time.Date(2023, 12, 31, 0, 0, 0, 0, time.UTC)))
		// On validAt - valid
		assert.True(t, edge.IsValid(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)))
		// After validAt - valid
		assert.True(t, edge.IsValid(time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)))
	})

	t.Run("edge with invalidAt", func(t *testing.T) {
		invalidAt := time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC)
		edge := Edge{ID: "edge_1", Relation: "test", InvalidAt: &invalidAt}

		// Before invalidAt - valid
		assert.True(t, edge.IsValid(time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)))
		// On invalidAt - valid
		assert.True(t, edge.IsValid(time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC)))
		// After invalidAt - not valid
		assert.False(t, edge.IsValid(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)))
	})

	t.Run("edge with both validAt and invalidAt", func(t *testing.T) {
		validAt := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
		invalidAt := time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC)
		edge := Edge{ID: "edge_1", Relation: "test", ValidAt: &validAt, InvalidAt: &invalidAt}

		// Before range - not valid
		assert.False(t, edge.IsValid(time.Date(2023, 12, 31, 0, 0, 0, 0, time.UTC)))
		// In range - valid
		assert.True(t, edge.IsValid(time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)))
		// After range - not valid
		assert.False(t, edge.IsValid(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)))
	})
}

func TestCommunity(t *testing.T) {
	t.Run("create community", func(t *testing.T) {
		now := time.Now()
		community := Community{
			ID:        "comm_123",
			AgentID:   "agent_1",
			UserID:    "user_1",
			EntityIDs: []string{"ent_1", "ent_2", "ent_3"},
			Summary:   "用户的工作相关信息",
			Topics:    []string{"工作", "职业", "技能"},
			CreatedAt: now,
			UpdatedAt: now,
		}

		assert.Equal(t, "comm_123", community.ID)
		assert.Equal(t, 3, len(community.EntityIDs))
		assert.Equal(t, "用户的工作相关信息", community.Summary)
		assert.Contains(t, community.Topics, "工作")
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
				IncludeEpisodes:    true,
				IncludeEntities:    true,
				IncludeEdges:       true,
				IncludeCommunities: false,
				MaxHops:            2,
			},
		}

		assert.True(t, req.Options.IncludeEpisodes)
		assert.True(t, req.Options.IncludeEntities)
		assert.True(t, req.Options.IncludeEdges)
		assert.False(t, req.Options.IncludeCommunities)
		assert.Equal(t, 2, req.Options.MaxHops)
	})

	t.Run("minimal request", func(t *testing.T) {
		req := RetrieveRequest{
			AgentID: "agent_1",
			UserID:  "user_1",
			Query:   "用户喜欢什么",
		}

		assert.Equal(t, 0, req.Limit)
		assert.False(t, req.Options.IncludeEpisodes)
	})
}

func TestAddResponse(t *testing.T) {
	resp := AddResponse{
		Success: true,
		Episodes: []Episode{
			{ID: "ep_1", Content: "test1"},
			{ID: "ep_2", Content: "test2"},
		},
		Entities: []Entity{
			{ID: "ent_1", Name: "entity1"},
		},
		Edges: []Edge{
			{ID: "edge_1", Fact: "fact1"},
		},
	}

	assert.True(t, resp.Success)
	assert.Equal(t, 2, len(resp.Episodes))
	assert.Equal(t, 1, len(resp.Entities))
	assert.Equal(t, 1, len(resp.Edges))
}

func TestRetrieveResponse(t *testing.T) {
	resp := RetrieveResponse{
		Success: true,
		Episodes: []Episode{
			{ID: "ep_1", Content: "test1"},
		},
		Entities: []Entity{
			{ID: "ent_1", Name: "entity1"},
		},
		Edges: []Edge{
			{ID: "edge_1", Fact: "fact1"},
		},
		Total:         3,
		MemoryContext: "## 用户信息\n- fact1",
	}

	assert.True(t, resp.Success)
	assert.Equal(t, 1, len(resp.Episodes))
	assert.Equal(t, 1, len(resp.Entities))
	assert.Equal(t, 1, len(resp.Edges))
	assert.Equal(t, 3, resp.Total)
	assert.Contains(t, resp.MemoryContext, "fact1")
}

func TestExtractionResult(t *testing.T) {
	result := ExtractionResult{
		Entities: []ExtractedEntity{
			{Name: "张三", Type: "person", Description: "用户的朋友"},
			{Name: "星巴克", Type: "place", Description: "咖啡店"},
		},
		Relations: []ExtractedRelation{
			{Subject: "张三", Predicate: "去过", Object: "星巴克", Fact: "张三去过星巴克"},
		},
	}

	assert.Equal(t, 2, len(result.Entities))
	assert.Equal(t, 1, len(result.Relations))
	assert.Equal(t, "张三", result.Entities[0].Name)
	assert.Equal(t, "张三去过星巴克", result.Relations[0].Fact)
}

func TestCommunityResult(t *testing.T) {
	t.Run("detected communities", func(t *testing.T) {
		result := CommunityResult{
			Communities: []DetectedCommunity{
				{
					EntityNames: []string{"张三", "李四", "王五"},
					Summary:     "用户的朋友圈",
					Topics:      []string{"社交", "朋友"},
				},
				{
					EntityNames: []string{"星巴克", "咖啡"},
					Summary:     "用户喜欢的咖啡相关事物",
					Topics:      []string{"咖啡", "饮品"},
				},
			},
		}

		assert.Equal(t, 2, len(result.Communities))
		assert.Equal(t, 3, len(result.Communities[0].EntityNames))
		assert.Equal(t, "用户的朋友圈", result.Communities[0].Summary)
		assert.Contains(t, result.Communities[0].Topics, "社交")
	})

	t.Run("empty communities", func(t *testing.T) {
		result := CommunityResult{
			Communities: []DetectedCommunity{},
		}

		assert.Empty(t, result.Communities)
	})
}
