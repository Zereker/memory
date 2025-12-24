package action

import (
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/Zereker/memory/internal/domain"
	"github.com/Zereker/memory/pkg/relation"
	"github.com/Zereker/memory/pkg/vector"
)

// 确保实现 domain.AddAction 接口
var _ domain.AddAction = (*EventExtractionAction)(nil)

// EventExtractionAction 事件三元组提取 Action（Layer 3）
type EventExtractionAction struct {
	*BaseAction

	vectorStore   vector.Store
	relationStore relation.Store
}

// NewEventExtractionAction 创建 EventExtractionAction
func NewEventExtractionAction() *EventExtractionAction {
	return &EventExtractionAction{
		BaseAction:    NewBaseAction("event_extraction"),
		vectorStore:   vector.NewStore(),
		relationStore: relation.NewStore(),
	}
}

// WithStores 设置存储（用于测试注入 mock）
func (a *EventExtractionAction) WithStores(vectorStore vector.Store, relationStore relation.Store) *EventExtractionAction {
	a.vectorStore = vectorStore
	a.relationStore = relationStore
	return a
}

// Name 返回 action 名称
func (a *EventExtractionAction) Name() string {
	return "event_extraction"
}

// EventExtractResult LLM 提取结果
type EventExtractResult struct {
	Events    []ExtractedEvent    `json:"events"`
	Relations []ExtractedRelation `json:"relations"`
}

// ExtractedEvent 单条提取的事件三元组
type ExtractedEvent struct {
	TriggerWord string `json:"trigger_word"`
	Argument1   string `json:"argument1"`
	Argument2   string `json:"argument2"`
}

// ExtractedRelation 事件间的关系
type ExtractedRelation struct {
	FromIndex    int    `json:"from_index"`
	ToIndex      int    `json:"to_index"`
	RelationType string `json:"relation_type"` // causal / temporal
}

// Handle 执行事件三元组提取
func (a *EventExtractionAction) Handle(c *domain.AddContext) {
	a.logger.Info("executing", "session_id", c.SessionID)

	if len(c.Messages) == 0 {
		c.Next()
		return
	}

	// 调用 LLM 提取事件
	conversation := c.Messages.Format()
	var result EventExtractResult
	if err := a.Generate(c, "event_extract", map[string]any{
		"conversation": conversation,
		"language":     c.LanguageName(),
	}, &result); err != nil {
		a.logger.Error("event extraction failed", "error", err)
		c.Next()
		return
	}

	if len(result.Events) == 0 {
		a.logger.Debug("no events extracted")
		c.Next()
		return
	}

	now := time.Now()
	eventIDs := make([]string, len(result.Events))

	// 存储事件三元组
	for i, ev := range result.Events {
		eventID := fmt.Sprintf("evt_%s", uuid.New().String()[:8])
		eventIDs[i] = eventID

		// 生成触发词向量
		triggerText := ev.Argument1 + " " + ev.TriggerWord + " " + ev.Argument2
		embedding, err := a.GenEmbedding(c.Context, EmbedderName, triggerText)
		if err != nil {
			a.logger.Warn("failed to generate trigger embedding", "error", err)
		}

		triplet := domain.EventTriplet{
			ID:               eventID,
			AgentID:          c.AgentID,
			UserID:           c.UserID,
			TriggerWord:      ev.TriggerWord,
			Argument1:        ev.Argument1,
			Argument2:        ev.Argument2,
			TriggerEmbedding: embedding,
			AccessCount:      0,
			LastAccessedAt:   now,
			CreatedAt:        now,
		}

		// 存储到 OpenSearch（向量检索用）
		if err := a.storeEventToVector(c, triplet); err != nil {
			a.logger.Warn("failed to store event to vector", "id", eventID, "error", err)
		}

		c.AddEvents(triplet)
	}

	// 存储事件关系
	for _, rel := range result.Relations {
		if rel.FromIndex < 0 || rel.FromIndex >= len(eventIDs) ||
			rel.ToIndex < 0 || rel.ToIndex >= len(eventIDs) {
			continue
		}

		eventRelation := domain.EventRelation{
			ID:           fmt.Sprintf("rel_%s", uuid.New().String()[:8]),
			RelationType: rel.RelationType,
			FromEventID:  eventIDs[rel.FromIndex],
			ToEventID:    eventIDs[rel.ToIndex],
			CreatedAt:    now,
		}

		// 存储关系到 PostgreSQL
		if err := a.storeRelation(c, eventRelation); err != nil {
			a.logger.Warn("failed to store relation", "error", err)
			continue
		}

		c.AddEventRelations(eventRelation)
	}

	a.logger.Info("event extraction completed",
		"events", len(c.Events),
		"relations", len(c.EventRelations),
	)

	c.Next()
}

// storeEventToVector 存储事件到 OpenSearch（向量检索）
func (a *EventExtractionAction) storeEventToVector(c *domain.AddContext, e domain.EventTriplet) error {
	if a.vectorStore == nil {
		return nil
	}

	doc := map[string]any{
		"id":               e.ID,
		"type":             domain.DocTypeEvent,
		"agent_id":         e.AgentID,
		"user_id":          e.UserID,
		"trigger_word":     e.TriggerWord,
		"argument1":        e.Argument1,
		"argument2":        e.Argument2,
		"embedding":        e.TriggerEmbedding, // 使用 embedding 字段与 k-NN 查询一致
		"access_count":     e.AccessCount,
		"last_accessed_at": e.LastAccessedAt,
		"created_at":       e.CreatedAt,
	}

	return a.vectorStore.Store(c.Context, e.ID, doc)
}

// storeRelation 存储事件关系到 PostgreSQL
func (a *EventExtractionAction) storeRelation(c *domain.AddContext, rel domain.EventRelation) error {
	if a.relationStore == nil {
		return nil
	}

	return a.relationStore.CreateRelation(c.Context, relation.Relation{
		ID:           rel.ID,
		FromEventID:  rel.FromEventID,
		ToEventID:    rel.ToEventID,
		RelationType: rel.RelationType,
		CreatedAt:    rel.CreatedAt,
	})
}
