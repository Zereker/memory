# 架构设计

## 概述

Memory System 采用 Zep 三层子图模型，将 AI Agent 的对话记忆分层组织和存储。这种设计借鉴了人类记忆的层次结构：原始经历 → 提炼的知识 → 抽象的理解。

## 为什么选择三层模型？

### 问题背景

传统的记忆系统通常采用单一存储方式：
- **纯向量检索**: 只存储原始对话的 Embedding，检索时通过语义相似度匹配
- **纯知识图谱**: 只存储提取的实体和关系

这两种方式各有局限：

| 方式 | 优点 | 缺点 |
|------|------|------|
| 纯向量 | 实现简单，保留完整上下文 | 无法结构化查询，难以关联分析 |
| 纯图谱 | 关系清晰，支持推理 | 丢失原始语境，提取有损失 |

### Zep 三层模型的解决方案

```
┌────────────────────────────────────────────────────────────────┐
│                    Layer 3: Summary                            │
│            主题摘要 - "用户讨论了咖啡偏好和星巴克体验"            │
│                    (OpenSearch 向量检索)                        │
├────────────────────────────────────────────────────────────────┤
│                  Layer 2: Entity + Edge                        │
│           结构化知识 - "阿信 --[喜欢]--> 咖啡"                   │
│                    (Neo4j 图遍历)                               │
├────────────────────────────────────────────────────────────────┤
│                    Layer 1: Episode                            │
│         原始对话 - "我今天去星巴克喝了一杯拿铁"                   │
│                    (OpenSearch 向量检索)                        │
└────────────────────────────────────────────────────────────────┘
```

**设计优势：**

1. **保真性**: Layer 1 保留完整对话，不丢失任何信息
2. **结构化**: Layer 2 提取关系，支持图遍历和推理
3. **高效性**: Layer 3 提供摘要，快速理解对话主题
4. **灵活检索**: 可以按需选择检索哪些层

---

## 三层详解

### Layer 1: Episode (原始对话)

**定位**: 记忆的"原材料"，保留完整的对话上下文。

**数据结构**:
```go
type Episode struct {
    ID             string    // 唯一标识
    AgentID        string    // AI Agent ID
    UserID         string    // 用户 ID
    SessionID      string    // 会话 ID
    Role           string    // user / assistant
    Name           string    // 发言者名称
    Content        string    // 对话内容
    Topic          string    // 主题标签 (2-4字)
    TopicEmbedding []float32 // 主题向量 (用于主题变化检测)
    Embedding      []float32 // 内容向量 (4096维)
    Timestamp      time.Time // 对话时间
}
```

**存储选择**: OpenSearch
- 支持 k-NN 向量检索
- 支持全文搜索
- 支持按时间、用户等过滤

**使用场景**:
- 需要完整上下文时（如回忆具体对话）
- 需要时间线回溯时
- 需要原话引用时

### Layer 2: Entity + Edge (实体 + 关系)

**定位**: 从对话中提炼的结构化知识。

**Entity (实体)**:
```go
type Entity struct {
    ID          string     // 唯一标识
    Name        string     // 实体名称
    Type        EntityType // person, place, thing, event, emotion, activity
    Description string     // 实体描述
    Embedding   []float32  // 向量表示
}
```

**Edge (关系边)**:
```go
type Edge struct {
    ID         string     // 唯一标识
    SourceID   string     // 起始实体
    TargetID   string     // 目标实体
    Relation   string     // 关系类型
    Fact       string     // 事实描述 (如: "阿信喜欢喝咖啡")
    ValidAt    *time.Time // 事实生效时间 (双时间轴)
    InvalidAt  *time.Time // 事实失效时间
    EpisodeIDs []string   // 来源 Episode (溯源)
}
```

**存储选择**: Neo4j
- 原生图数据库，关系查询高效
- 支持 Cypher 查询语言
- 支持图遍历算法

**双时间轴设计**:

Edge 支持双时间轴（Bi-temporal），可以表达时效性：
- `ValidAt`: 事实开始生效的时间
- `InvalidAt`: 事实失效的时间

例如："阿信住在北京"这个事实，可能在某个时间点失效（搬家了）。

**使用场景**:
- 查询用户的属性和偏好
- 关联分析（如通过"咖啡"关联到"星巴克"）
- 图遍历扩展相关记忆

### Layer 3: Summary (主题摘要)

**定位**: 主题变化时生成的对话摘要，用于快速理解对话内容。

**数据结构**:
```go
type Summary struct {
    ID         string    // 唯一标识
    AgentID    string    // AI Agent ID
    UserID     string    // 用户 ID
    EpisodeIDs []string  // 包含的 Episode
    Topic      string    // 主题标签
    Content    string    // 摘要内容
    Embedding  []float32 // 向量表示
}
```

**存储选择**: OpenSearch
- 摘要需要向量检索
- 需要按主题标签过滤

**生成逻辑**:
1. 为每条 Episode 生成 Topic 和 TopicEmbedding
2. 计算相邻 Episode 的 TopicEmbedding 余弦相似度
3. 相似度 < 阈值 (默认 0.7) 时检测为主题变化
4. 对变化前的 Episodes 调用 LLM 生成摘要
5. 生成 Embedding 并存储

**使用场景**:
- 快速了解对话主题
- 高层次的语义匹配
- 作为 LLM 的上下文前缀

---

## 数据流

### 存储流程 (Add)

```
请求 → EpisodeStorageAction → ExtractionAction → SummaryAction → 响应
         │                      │                   │
         │ 1. 生成 Embedding    │ 2. LLM 提取       │ 3. 检测主题变化
         │ 2. 生成 Topic        │    实体/关系      │ 4. 生成摘要
         │ 3. 存储到 OpenSearch │ 3. 存储到 Neo4j   │ 5. 存储到 OpenSearch
         ▼                      ▼                   ▼
    ┌─────────┐           ┌─────────┐         ┌─────────┐
    │OpenSearch│           │  Neo4j  │         │OpenSearch│
    │ Episode │           │Entity/Edge│        │ Summary │
    └─────────┘           └─────────┘         └─────────┘
```

**Action Chain 设计**:

每个 Action 职责单一，通过 Chain 串联：

```go
chain := domain.NewActionChain()
chain.Use(NewEpisodeStorageAction()) // Layer 1: Episode + Topic
chain.Use(NewExtractionAction())     // Layer 2: Entity/Edge
chain.Use(NewSummaryAction())        // Layer 3: Summary
chain.Run(ctx)
```

优点：
- 职责分离，易于维护
- 可以灵活组合
- 每个 Action 可以独立测试

### 检索流程 (Retrieve)

```
查询 → 生成 Query Embedding → 并行检索 → 合并排序 → 格式化 → 响应
                                │
            ┌───────────────────┼───────────────────┐
            ▼                   ▼                   ▼
       OpenSearch           Neo4j              OpenSearch
       (Episode)         (Entity/Edge)         (Summary)
            │                   │                   │
            └───────────────────┴───────────────────┘
                                │
                          图遍历扩展
                                │
                          合并去重排序
```

**混合检索策略**:

1. **向量检索**: Episode 和 Summary 通过 Embedding 相似度匹配
2. **图遍历**: 从匹配的 Entity 出发，遍历关联的 Edge 和 Entity
3. **融合排序**: 综合向量分数和图遍历深度

---

## 存储选型

### 为什么选择 OpenSearch？

| 需求 | OpenSearch 能力 |
|------|-----------------|
| 向量检索 | k-NN 插件，支持 HNSW 算法 |
| 全文搜索 | Lucene 引擎，中文分词 |
| 过滤查询 | 支持复杂的 bool 查询 |
| 扩展性 | 分布式架构，水平扩展 |

**索引配置**:
```json
{
  "settings": {
    "index": {
      "knn": true,
      "knn.algo_param.ef_search": 100
    }
  },
  "mappings": {
    "properties": {
      "embedding": {
        "type": "knn_vector",
        "dimension": 4096,
        "method": {
          "name": "hnsw",
          "space_type": "cosinesimil",
          "engine": "nmslib"
        }
      }
    }
  }
}
```

### 为什么选择 Neo4j？

| 需求 | Neo4j 能力 |
|------|-----------|
| 图遍历 | 原生图引擎，O(1) 关系查找 |
| 关系建模 | 属性图模型，灵活表达 |
| 查询语言 | Cypher，声明式图查询 |
| 索引 | 支持节点/关系属性索引 |

**图模式**:
```cypher
(:Entity {name, type, description})
    -[:RELATION {fact, valid_at, invalid_at}]->
(:Entity)
```

---

## LLM 集成

### Topic 生成

每条 Episode 存储时生成主题标签：

**Prompt 模板**:
```
为以下内容生成一个简短的主题标签。使用{language}输出。

内容：
{content}

规则：
1. 主题标签为 2-4 字
2. 概括内容的核心主题
```

**输出结构**:
```go
type TopicResult struct {
    Topic string `json:"topic"`
}
```

### 实体/关系提取

使用 LLM 从对话中提取结构化信息：

**Prompt 模板**:
```
分析以下对话，提取实体和关系。

对话内容：
{conversation}

任务：
1. 提取对话中提到的实体（人物、地点、物品等）
2. 提取实体之间的关系

输出格式（JSON）：
{"entities": [...], "relations": [...]}
```

**输出结构**:
```go
type ExtractionResult struct {
    Entities  []ExtractedEntity   `json:"entities"`
    Relations []ExtractedRelation `json:"relations"`
}
```

### 摘要生成

主题变化时生成对话摘要：

**Prompt 模板**:
```
将以下对话压缩为简洁摘要，保留关键信息。使用{language}输出。

对话内容：
{conversation}

规则：
1. 摘要应简洁，1-2句话
2. 保留关键事实、人名、地点、时间
3. 只输出用户相关的信息，不包含 AI 助手的回复
```

**输出结构**:
```go
type SummaryResult struct {
    Content string `json:"content"`
}
```

### Embedding 模型选择

选择 `qwen3-embedding-8b` (4096 维)：
- 中文语义理解能力强
- 维度较高，区分度好
- 性能与效果平衡

---

## 扩展性设计

### 添加新的 Action

实现 `domain.AddAction` 接口：

```go
type MyAction struct {
    *BaseAction
}

func (a *MyAction) Name() string {
    return "my_action"
}

func (a *MyAction) Handle(c *domain.AddContext) {
    // 处理逻辑
    c.Next()
}
```

注册到 Chain：
```go
chain.Use(NewMyAction())
```

### BaseAction 公共能力

所有 Action 继承 BaseAction，获得以下能力：

```go
// 调用 LLM 生成内容，自动记录 token 使用量
func (b *BaseAction) Generate(c *domain.AddContext, promptName string, input map[string]any, output any) error

// 生成文本向量
func (b *BaseAction) GenEmbedding(ctx context.Context, embedderName, text string) ([]float32, error)

// 计算余弦相似度
func (b *BaseAction) CosineSimilarity(vec1, vec2 []float32) float64

// 文档转 Episode
func (b *BaseAction) DocToEpisode(doc map[string]any) *domain.Episode
```

### 支持新的存储后端

实现存储接口，替换 OpenSearch/Neo4j：
- `pkg/storage` - 向量存储接口
- `pkg/graph` - 图存储接口

### 多租户支持

通过 `agent_id` + `user_id` 实现数据隔离：
- 所有数据都带有 agent_id 和 user_id
- 查询时自动过滤
- 索引按租户分片（可选）
