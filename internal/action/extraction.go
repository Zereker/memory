package action

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/pkg/errors"

	"github.com/Zereker/memory/internal/domain"
	"github.com/Zereker/memory/pkg/graph"
	"github.com/Zereker/memory/pkg/storage"
)

const (
	LabelEntity = "Entity"
)

// 确保实现 domain.AddAction 接口
var _ domain.AddAction = (*ExtractionAction)(nil)

// ExtractionAction 从对话中提取 Entity 和 Edge
type ExtractionAction struct {
	*BaseAction

	vectorStore *storage.OpenSearchStore
	graphStore  *graph.Neo4jStore
}

// NewExtractionAction 创建 ExtractionAction
func NewExtractionAction() *ExtractionAction {
	return &ExtractionAction{
		BaseAction:  NewBaseAction("extraction"),
		vectorStore: storage.NewStore(),
		graphStore:  graph.NewStore(),
	}
}

// Name 返回 action 名称
func (a *ExtractionAction) Name() string {
	return "extraction"
}

// Handle 执行实体和关系提取
func (a *ExtractionAction) Handle(c *domain.AddContext) {
	a.logger.Info("executing", "session_id", c.SessionID)

	if len(c.Messages) == 0 {
		a.logger.Debug("no messages, skipping")
		c.Next()
		return
	}

	// 调用 LLM 提取实体和关系
	conversation := c.Messages.Format()
	var extracted ExtractionResult
	if err := a.Generate(c, "extraction", map[string]any{
		"conversation": conversation,
		"language":     c.LanguageName(),
	}, &extracted); err != nil {
		a.logger.Error("extraction failed", "error", err)
		c.SetError(errors.WithMessage(err, "extraction failed"))
		return
	}

	// 构建实体并持久化到 Neo4j + OpenSearch
	entities := a.buildEntities(c, extracted.Entities)
	entityByID := make(map[string]domain.Entity, len(entities))
	for _, entity := range entities {
		// 存储到 Neo4j（图结构）
		if err := a.storeEntity(c, entity); err != nil {
			a.logger.Warn("failed to store entity to graph", "id", entity.ID, "error", err)
			continue
		}
		// 存储到 OpenSearch（向量检索）
		if err := a.storeEntityToVector(c, entity); err != nil {
			a.logger.Warn("failed to store entity to vector", "id", entity.ID, "error", err)
		}
		c.AddEntities(entity)
		entityByID[entity.ID] = entity
	}

	// 构建边并持久化到 Neo4j + OpenSearch（使用成功存储的实体）
	edges := a.buildEdges(c, extracted.Relations, c.Entities)
	for _, edge := range edges {
		source, ok1 := entityByID[edge.SourceID]
		target, ok2 := entityByID[edge.TargetID]
		if !ok1 || !ok2 {
			a.logger.Warn("edge references unstored entity", "source", edge.SourceID, "target", edge.TargetID)
			continue
		}
		// 存储到 Neo4j（图结构）
		if err := a.storeEdge(c, edge, source.Name, target.Name); err != nil {
			a.logger.Warn("failed to store edge to graph", "id", edge.ID, "error", err)
			continue
		}
		// 存储到 OpenSearch（向量检索）
		if err := a.storeEdgeToVector(c, edge); err != nil {
			a.logger.Warn("failed to store edge to vector", "id", edge.ID, "error", err)
		}
		c.AddEdges(edge)
	}

	a.logger.Info("extraction completed",
		"entities", len(c.Entities),
		"edges", len(c.Edges),
	)

	c.Next()
}

// ExtractedEntity LLM 提取的实体
type ExtractedEntity struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
}

// ExtractedRelation LLM 提取的关系
type ExtractedRelation struct {
	Subject   string `json:"subject"`   // 主体实体名
	Predicate string `json:"predicate"` // 关系
	Object    string `json:"object"`    // 客体实体名
	Fact      string `json:"fact"`      // 事实描述
}

// ExtractionResult LLM 提取结果
type ExtractionResult struct {
	Entities  []ExtractedEntity   `json:"entities"`
	Relations []ExtractedRelation `json:"relations"`
}

// buildEntities 将提取结果转换为 Entity 列表
func (a *ExtractionAction) buildEntities(c *domain.AddContext, extracted []ExtractedEntity) []domain.Entity {
	now := time.Now()
	entities := make([]domain.Entity, 0, len(extracted))

	for _, e := range extracted {
		entity := domain.Entity{
			ID:          fmt.Sprintf("ent_%s", uuid.New().String()[:8]),
			AgentID:     c.AgentID,
			UserID:      c.UserID,
			Name:        e.Name,
			Type:        domain.EntityType(e.Type),
			Description: e.Description,
			CreatedAt:   now,
			UpdatedAt:   now,
		}

		// 生成 embedding
		if embedding, err := a.GenEmbedding(c.Context, EmbedderName, e.Name+" "+e.Description); err != nil {
			a.logger.Warn("failed to generate entity embedding", "name", e.Name, "error", err)
		} else {
			entity.Embedding = embedding
		}

		entities = append(entities, entity)
	}

	return entities
}

// buildEdges 将提取结果转换为 Edge 列表
func (a *ExtractionAction) buildEdges(c *domain.AddContext, relations []ExtractedRelation, entities []domain.Entity) []domain.Edge {
	now := time.Now()
	edges := make([]domain.Edge, 0, len(relations))

	// 构建 entityMap
	entityMap := make(map[string]*domain.Entity, len(entities))
	for i := range entities {
		entityMap[entities[i].Name] = &entities[i]
	}

	// 收集 Episode IDs
	episodeIDs := make([]string, 0, len(c.Episodes))
	for _, ep := range c.Episodes {
		episodeIDs = append(episodeIDs, ep.ID)
	}

	for _, rel := range relations {
		source, target := entityMap[rel.Subject], entityMap[rel.Object]
		if source == nil || target == nil {
			a.logger.Warn("relation references unknown entity", "subject", rel.Subject, "object", rel.Object)
			continue
		}

		edge := domain.Edge{
			ID:         fmt.Sprintf("edge_%s", uuid.New().String()[:8]),
			SourceID:   source.ID,
			TargetID:   target.ID,
			Relation:   rel.Predicate,
			Fact:       rel.Fact,
			EpisodeIDs: episodeIDs,
			CreatedAt:  now,
		}

		// 生成 embedding
		if embedding, err := a.GenEmbedding(c.Context, EmbedderName, rel.Fact); err != nil {
			a.logger.Warn("failed to generate edge embedding", "fact", rel.Fact, "error", err)
		} else {
			edge.Embedding = embedding
		}

		edges = append(edges, edge)
	}

	return edges
}

// storeEntity 存储 Entity 到 Neo4j
func (a *ExtractionAction) storeEntity(c *domain.AddContext, entity domain.Entity) error {
	if a.graphStore == nil {
		return errors.New("graph store not initialized")
	}

	labels := []string{LabelEntity, string(entity.Type)}
	properties := map[string]any{
		"id":          entity.ID,
		"type":        string(entity.Type),
		"name":        entity.Name,
		"description": entity.Description,
		"agent_id":    entity.AgentID,
		"user_id":     entity.UserID,
		"session_id":  c.SessionID,
		"created_at":  entity.CreatedAt.Unix(),
		"updated_at":  entity.UpdatedAt.Unix(),
	}

	return a.graphStore.MergeNode(c.Context, labels, "name", entity.Name, properties)
}

// storeEntityToVector 存储 Entity 到 OpenSearch（用于向量检索锚定）
func (a *ExtractionAction) storeEntityToVector(c *domain.AddContext, entity domain.Entity) error {
	if a.vectorStore == nil {
		return nil // 向量存储不可用时静默跳过
	}

	doc := map[string]any{
		"id":          entity.ID,
		"type":        domain.DocTypeEntity,
		"entity_type": string(entity.Type),
		"name":        entity.Name,
		"description": entity.Description,
		"agent_id":    entity.AgentID,
		"user_id":     entity.UserID,
		"session_id":  c.SessionID,
		"embedding":   entity.Embedding, // 使用 embedding 字段与 k-NN 查询一致
		"created_at":  entity.CreatedAt,
		"updated_at":  entity.UpdatedAt,
	}

	return a.vectorStore.Store(c.Context, entity.ID, doc)
}

// storeEdge 存储 Edge 到 Neo4j
func (a *ExtractionAction) storeEdge(c *domain.AddContext, edge domain.Edge, sourceName, targetName string) error {
	if a.graphStore == nil {
		return errors.New("graph store not initialized")
	}

	properties := map[string]any{
		"id":          edge.ID,
		"fact":        edge.Fact,
		"episode_ids": edge.EpisodeIDs,
		"session_id":  c.SessionID,
		"created_at":  edge.CreatedAt.Unix(),
	}

	return a.graphStore.CreateRelationship(
		c.Context,
		LabelEntity, "name", sourceName,
		LabelEntity, "name", targetName,
		edge.Relation,
		properties,
	)
}

// storeEdgeToVector 存储 Edge 到 OpenSearch（用于向量检索）
func (a *ExtractionAction) storeEdgeToVector(c *domain.AddContext, edge domain.Edge) error {
	if a.vectorStore == nil {
		return nil // 向量存储不可用时静默跳过
	}

	doc := map[string]any{
		"id":          edge.ID,
		"type":        domain.DocTypeEdge,
		"source_id":   edge.SourceID,
		"target_id":   edge.TargetID,
		"relation":    edge.Relation,
		"fact":        edge.Fact,
		"agent_id":    c.AgentID,
		"user_id":     c.UserID,
		"session_id":  c.SessionID,
		"episode_ids": edge.EpisodeIDs,
		"embedding":   edge.Embedding,
		"created_at":  edge.CreatedAt,
	}

	return a.vectorStore.Store(c.Context, edge.ID, doc)
}
