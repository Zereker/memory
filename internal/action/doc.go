// Package action 实现记忆系统的核心操作。
//
// # 架构概述 (Zep 风格三层子图模型)
//
// 记忆系统采用 Zep/Graphiti 风格的三层子图模型，通过 Action Chain 模式处理记忆的写入和检索。
//
//	┌─────────────────────────────────────────────────────────────┐
//	│                        Memory                               │
//	│  统一入口，协调各 Action 的执行                              │
//	└─────────────────────────────────────────────────────────────┘
//	                           │
//	              ┌────────────┴────────────┐
//	              ▼                         ▼
//	        ┌──────────┐             ┌──────────┐
//	        │   Add    │             │ Retrieve │
//	        │   Flow   │             │   Flow   │
//	        └──────────┘             └──────────┘
//
// # 三层子图模型
//
//	┌─────────────────────────────────────────────────────────────┐
//	│                   Three-Layer Subgraph                       │
//	│                                                             │
//	│  Layer 3: Summary (主题摘要)                                 │
//	│              ▲                                              │
//	│  Layer 2: Entity + Edge (实体 + 关系)                        │
//	│              ▲                                              │
//	│  Layer 1: Episode (原始对话)                                 │
//	│                                                             │
//	└─────────────────────────────────────────────────────────────┘
//
// # 数据类型
//
//   - Episode: 原始对话记录 (Layer 1)
//   - 存储每条对话消息 (role + content)
//   - 生成 embedding 用于向量检索
//   - 生成 topic 用于主题检测
//   - 生成 topic_embedding 用于主题相似度计算
//   - 存储在 OpenSearch
//
//   - Entity: 实体节点 (Layer 2)
//   - 从对话中提取的实体 (人物、地点、事物、事件等)
//   - 存储在 Neo4j
//
//   - Edge: 关系边 (Layer 2)
//   - 实体间的关系 (如: 喜欢、认识、住在)
//   - 包含 fact 字段描述事实
//   - 支持双时间轴 (ValidAt/InvalidAt)
//   - 存储在 Neo4j
//
//   - Summary: 主题摘要 (Layer 3)
//   - 检测到主题变化时生成的对话摘要
//   - 基于 TopicEmbedding 相似度检测主题变化
//   - 存储在 OpenSearch (向量检索)
//
// # 数据流
//
// ## Add 流程 (写入)
//
//	对话消息
//	    │
//	    ▼
//	┌─────────────────────────────────────────────────────────┐
//	│  EpisodeStorageAction                                    │
//	│  - 将每条消息转换为 Episode                               │
//	│  - 生成 embedding                                       │
//	│  - 生成 topic (LLM)                                     │
//	│  - 生成 topic_embedding                                 │
//	│  - 存储到 OpenSearch                                    │
//	└─────────────────────────────────────────────────────────┘
//	    │
//	    ▼
//	┌─────────────────────────────────────────────────────────┐
//	│  ExtractionAction                                        │
//	│  - LLM 提取 Entity (实体)                                │
//	│  - LLM 提取 Edge (关系三元组)                            │
//	│  - 存储到 Neo4j                                         │
//	└─────────────────────────────────────────────────────────┘
//	    │
//	    ▼
//	┌─────────────────────────────────────────────────────────┐
//	│  SummaryAction                                           │
//	│  - 计算 TopicEmbedding 相似度                            │
//	│  - 检测主题变化 (相似度 < 阈值)                           │
//	│  - 生成对话摘要 (LLM)                                    │
//	│  - 存储到 OpenSearch                                    │
//	└─────────────────────────────────────────────────────────┘
//
// ## Retrieve 流程 (检索)
//
//	查询文本
//	    │
//	    ▼
//	┌─────────────────────────────────────────────────────────┐
//	│  RetrievalAction                                         │
//	│  - 生成查询 embedding                                    │
//	│  - 向量检索 Episodes (OpenSearch)                        │
//	│  - 向量检索 Entities (Neo4j)                             │
//	│  - 向量检索 Summaries (OpenSearch)                       │
//	│  - 图遍历扩展相关 Entities/Edges                          │
//	└─────────────────────────────────────────────────────────┘
//	    │
//	    ▼
//	格式化为 MemoryContext (用于 LLM Prompt)
//
// # Action 类型
//
// ## EpisodeStorageAction
//
// 将原始对话存储为 Episode：
//   - 输入：AddContext.Messages (对话消息列表)
//   - 输出：AddContext.Episodes (Episode 列表)
//   - 功能：
//   - 为每条消息生成 ID、embedding、时间戳
//   - 调用 LLM 生成 topic (2-4 字主题标签)
//   - 生成 topic_embedding 用于主题相似度计算
//
// ## ExtractionAction
//
// 从对话中提取实体和关系：
//   - 输入：AddContext.Messages (对话消息)
//   - 输出：AddContext.Entities, AddContext.Edges
//   - 功能：调用 LLM 提取实体 (Entity) 和关系 (Edge)
//   - 实体类型：person, place, thing, event, emotion, activity
//
// ## SummaryAction
//
// 检测主题变化并生成摘要：
//   - 输入：AddContext.Episodes
//   - 输出：AddContext.Summaries
//   - 功能：
//   - 加载历史 user Episode
//   - 计算 TopicEmbedding 余弦相似度
//   - 相似度 < 阈值时触发摘要生成
//   - 调用 LLM 生成对话摘要
//   - 存储到 OpenSearch
//
// ## RetrievalAction
//
// 混合检索相关记忆：
//   - 输入：RecallContext.Query (查询文本)
//   - 输出：RecallContext.Episodes, Entities, Edges, Summaries
//   - 检索策略：
//   - Episodes: 向量相似度检索
//   - Entities: 名称匹配 + 图遍历
//   - Summaries: 向量相似度检索
//   - Edges: 通过关联实体获取
//   - 可选：MaxHops 参数控制图遍历深度
//
// # BaseAction 公共能力
//
// BaseAction 提供所有 Action 的公共方法：
//   - Generate: 调用 LLM 生成内容，自动记录 token 使用量
//   - GenEmbedding: 生成文本向量表示
//   - CosineSimilarity: 计算向量余弦相似度
//   - DocToEpisode: 将存储文档转换为 Episode 结构
//
// # LLM 输出类型
//
//   - TopicResult: topic prompt 输出 (定义在 episode.go)
//   - SummaryResult: summary prompt 输出 (定义在 summary.go)
//   - ExtractionResult: extraction prompt 输出 (定义在 extraction.go)
//
// # 存储后端
//
//   - OpenSearch: Episode/Summary 向量存储，支持混合检索 (向量 + BM25)
//   - Neo4j: Entity/Edge 图存储，支持图遍历查询
//
// # 使用示例
//
//	// 创建 Memory 实例
//	mem := action.NewMemory()
//
//	// 从对话添加记忆
//	addResp, err := mem.Add(ctx, &domain.AddRequest{
//	    AgentID:   "jarvis",
//	    UserID:    "user-123",
//	    SessionID: "session-456",
//	    Messages: []domain.Message{
//	        {Role: "user", Name: "阿信", Content: "我今天去了星巴克"},
//	        {Role: "assistant", Name: "贾维斯", Content: "星巴克的咖啡怎么样？"},
//	    },
//	})
//	// addResp.Episodes: 2 条原始对话 (含 topic)
//	// addResp.Entities: 提取的实体 (阿信, 星巴克)
//	// addResp.Edges: 提取的关系 (阿信 -[去过]-> 星巴克)
//	// addResp.Summaries: 主题变化时生成的摘要
//
//	// 检索相关记忆
//	retResp, err := mem.Retrieve(ctx, &domain.RetrieveRequest{
//	    AgentID: "jarvis",
//	    UserID:  "user-123",
//	    Query:   "用户喜欢喝什么",
//	    Options: domain.RetrieveOptions{
//	        IncludeEpisodes:  true,
//	        IncludeEntities:  true,
//	        IncludeEdges:     true,
//	        IncludeSummaries: true,
//	        MaxHops:          2,
//	    },
//	})
//
//	// 获取格式化的记忆上下文 (用于 LLM Prompt)
//	memoryContext := retResp.MemoryContext
package action
