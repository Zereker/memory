# Memory System

基于 Zep 三层子图模型的长期记忆系统，用于 AI Agent 的对话记忆管理。

## 特性

- **三层记忆模型**: Episode → Entity/Edge → Summary
- **向量检索**: OpenSearch k-NN 实现语义相似度搜索
- **知识图谱**: Neo4j 存储实体关系，支持图遍历
- **LLM 驱动**: 自动提取实体、关系，检测主题变化生成摘要
- **主题检测**: 基于 TopicEmbedding 相似度检测话题变化
- **双协议支持**: HTTP REST API 和 MCP Protocol

## 快速开始

### 1. 启动依赖服务

```bash
# OpenSearch
docker run -d --name opensearch \
  -p 9200:9200 \
  -e "discovery.type=single-node" \
  -e "DISABLE_SECURITY_PLUGIN=true" \
  opensearchproject/opensearch:2.11.0

# Neo4j
docker run -d --name neo4j \
  -p 7474:7474 -p 7687:7687 \
  -e NEO4J_AUTH=neo4j/YOUR_NEO4J_PASSWORD \
  neo4j:5.15.0
```

### 2. 初始化索引

```bash
python3 scripts/cli.py init
```

### 3. 编译运行

```bash
go build -o bin/memory ./cmd/memory
./bin/memory -config configs/config.toml
```

### 4. 验证服务

```bash
curl http://localhost:8080/health
```

## 文档

| 文档 | 说明 |
|------|------|
| [架构设计](docs/architecture.md) | 三层模型原理、设计决策、数据流 |
| [API 文档](docs/api.md) | 接口规范、请求响应示例 |
| [部署运维](docs/deployment.md) | 配置、初始化、测试、故障排查 |

## 项目结构

```
memory/
├── cmd/memory/        # 主程序入口
├── configs/           # 配置文件
├── internal/
│   ├── action/        # Action Chain 处理逻辑
│   ├── api/           # HTTP/MCP API Handler
│   ├── domain/        # 领域模型
│   └── server/        # 服务启动
├── pkg/               # 公共包 (genkit, graph, storage)
├── scripts/
│   ├── cli.py         # 统一 CLI 工具
│   └── lib/           # Python 库
├── data/              # 测试数据
└── docs/              # 文档
```

## 技术栈

| 组件 | 技术 |
|------|------|
| 向量存储 | OpenSearch 2.x (k-NN) |
| 图存储 | Neo4j 5.x |
| LLM/Embedding | 可配置 (支持 Ark 等多厂商) |
| 框架 | Firebase Genkit |
