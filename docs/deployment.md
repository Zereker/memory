# 部署运维

## 目录

- [环境要求](#环境要求)
- [依赖服务部署](#依赖服务部署)
- [索引初始化](#索引初始化)
- [配置说明](#配置说明)
- [服务启动](#服务启动)
- [测试验证](#测试验证)
- [运维操作](#运维操作)
- [故障排查](#故障排查)

---

## 环境要求

| 组件 | 版本要求 |
|------|----------|
| Go | >= 1.21 |
| Python | >= 3.8 |
| OpenSearch | >= 2.11 |
| Neo4j | >= 5.15 |
| Docker | >= 20.10 (可选) |

---

## 依赖服务部署

### OpenSearch

**Docker 方式**:
```bash
docker run -d --name opensearch \
  -p 9200:9200 \
  -e "discovery.type=single-node" \
  -e "DISABLE_SECURITY_PLUGIN=true" \
  -e "OPENSEARCH_JAVA_OPTS=-Xms512m -Xmx512m" \
  opensearchproject/opensearch:2.11.0
```

**验证**:
```bash
curl http://localhost:9200
```

### Neo4j

**Docker 方式**:
```bash
docker run -d --name neo4j \
  -p 7474:7474 \
  -p 7687:7687 \
  -e NEO4J_AUTH=neo4j/YOUR_NEO4J_PASSWORD \
  neo4j:5.15.0
```

**验证**:
```bash
curl http://localhost:7474
```

---

## 索引初始化

使用 CLI 工具初始化：

```bash
python3 scripts/cli.py init
```

输出示例：
```
Initializing indexes...
  OpenSearch... OK
  Neo4j... OK
Done.
```

### 索引结构

**OpenSearch memories 索引**:

| 字段 | 类型 | 说明 |
|------|------|------|
| id | keyword | 唯一标识 |
| type | keyword | episode / community |
| agent_id | keyword | Agent 标识 |
| user_id | keyword | 用户标识 |
| session_id | keyword | 会话标识 |
| content | text | 内容 |
| embedding | knn_vector | 4096 维向量 |
| timestamp | date | 时间戳 |

**Neo4j Entity 索引**:

| 索引 | 字段 | 说明 |
|------|------|------|
| entity_name | name | 实体名称查找 |
| entity_id | id | ID 查找 |
| entity_agent_user | agent_id, user_id | 多租户过滤 |

---

## 配置说明

配置文件：`configs/config.toml`

### 完整配置示例

```toml
# 服务配置
[server]
mode = "http"  # http, mcp, or both
port = 8080

# 日志配置
[log]
path = "logs"
level = "debug"  # debug, info, warn, error
format = "json"  # json, text
rotation_time = "24h"
max_age = "168h"

# ============== AI 模型配置 ==============
[genkit]
prompt_dir = "internal/action/prompts"

# Ark 厂商
[genkit.ark]
api_key = ""  # Your Ark API key
base_url = "https://ark.cn-beijing.volces.com/api/v3"

# LLM 模型
[[genkit.ark.models]]
name = "doubao-pro-32k"
type = "llm"
model = "doubao-pro-32k"

# Embedding 模型
[[genkit.ark.models]]
name = "doubao-embedding-text-240715"
type = "embedding"
model = "doubao-embedding-text-240715"
dim = 2560

# ============== 存储配置 ==============

# OpenSearch
[storage]
addresses = ["http://localhost:9200"]
username = ""
password = ""
index = "memories"
embedding_dim = 2560

# Neo4j
[neo4j]
enabled = true
uri = "bolt://localhost:7687"
username = "neo4j"
password = "YOUR_NEO4J_PASSWORD"
database = "neo4j"

# ============== 可选组件 ==============

# Redis (分布式状态)
[redis]
enabled = false
addr = "localhost:6379"

# Kafka (异步处理)
[kafka]
enabled = false
brokers = ["localhost:9092"]
```

### 配置项说明

| 配置项 | 说明 | 默认值 |
|--------|------|--------|
| server.mode | 服务模式 | http |
| server.port | 服务端口 | 8080 |
| storage.embedding_dim | Embedding 维度 | 4096 |
| neo4j.enabled | 是否启用 Neo4j | true |

---

## 服务启动

### 编译

```bash
go build -o bin/memory ./cmd/memory
```

### 启动

```bash
./bin/memory -config configs/config.toml
```

### 后台运行

```bash
nohup ./bin/memory -config configs/config.toml > /dev/null 2>&1 &
```

### Systemd 服务

创建 `/etc/systemd/system/memory.service`:

```ini
[Unit]
Description=Memory System
After=network.target

[Service]
Type=simple
User=memory
WorkingDirectory=/opt/memory
ExecStart=/opt/memory/bin/memory -config /opt/memory/configs/config.toml
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl enable memory
sudo systemctl start memory
```

---

## 测试验证

### CLI 工具

所有测试通过统一的 CLI 工具执行：

```bash
python3 scripts/cli.py <command> [mode]
```

### 命令列表

| 命令 | 说明 |
|------|------|
| `status` | 查看服务状态 |
| `init` | 初始化索引 |
| `clear` | 清空所有数据 |
| `reset` | 重置 (清空 + 初始化) |
| `test [mode]` | 运行测试 |
| `preview [mode]` | 完整流程 (重置 + 测试) |

### 测试模式

| 模式 | 说明 |
|------|------|
| `quick` | 快速冒烟测试 (2 个请求) |
| `store` | 仅存储测试 |
| `retrieve` | 仅召回测试 |
| `full` | 完整测试 (存储 + 召回) |
| `all` | 全部测试 (默认) |

### 常用命令

```bash
# 查看服务状态
python3 scripts/cli.py status

# 快速测试
python3 scripts/cli.py test quick

# 完整测试
python3 scripts/cli.py test full

# 完整流程：重置数据 + 运行测试
python3 scripts/cli.py preview

# 仅重置数据
python3 scripts/cli.py reset
```

### 测试数据

测试数据位于 `data/test_data_complete.json`：
- 33 组多轮对话
- 10 个测试问题（基础召回、时序推理、因果推理）

### 测试报告

测试完成后自动生成：
- `data/test_results.md` - 简要报告
- `data/test_results_detail.md` - 详细报告

---

## 运维操作

### 查看状态

```bash
python3 scripts/cli.py status
```

输出示例：
```
Service Status
----------------------------------------
Memory Server:  OK
OpenSearch:     OK (v2.11.0, 80 docs)
Neo4j:          OK (28 nodes)
```

### 清空数据

```bash
python3 scripts/cli.py clear
```

### 重置环境

```bash
python3 scripts/cli.py reset
```

### 查看日志

```bash
tail -f logs/memory-$(date +%Y-%m-%d).log
```

### 手动查询数据

**OpenSearch 数据量**:
```bash
curl "http://localhost:9200/memories/_count"
```

**Neo4j 节点数量**:
```bash
curl -X POST "http://localhost:7474/db/neo4j/tx/commit" \
  -H "Content-Type: application/json" \
  -u "neo4j:YOUR_NEO4J_PASSWORD" \
  -d '{"statements": [{"statement": "MATCH (n) RETURN count(n) as count"}]}'
```

---

## 故障排查

### 服务无法启动

**检查端口占用**:
```bash
lsof -i :8080
```

### OpenSearch 连接失败

**检查服务状态**:
```bash
curl http://localhost:9200/_cluster/health
```

**常见问题**:
- 端口未开放
- 安全插件未禁用
- 内存不足

### Neo4j 连接失败

**检查服务状态**:
```bash
curl http://localhost:7474
```

**检查 Bolt 端口**:
```bash
nc -zv localhost 7687
```

**常见问题**:
- 密码错误
- 数据库名称错误
- 端口未开放

### Embedding 维度不匹配

**错误信息**:
```
vector dimension mismatch: expected 4096, got 2560
```

**解决方法**:
1. 确认 Embedding 模型维度
2. 更新 `config.toml` 中的 `embedding_dim`
3. 重置索引：

```bash
python3 scripts/cli.py reset
```

### LLM 调用超时

**错误信息**:
```
context deadline exceeded
```

**解决方法**:
1. 检查 LLM 服务可用性
2. 增加超时时间
3. 检查网络连通性

### 检索结果为空

**排查步骤**:

1. 查看状态确认数据已存储：
```bash
python3 scripts/cli.py status
```

2. 检查 agent_id/user_id 是否匹配：
```bash
curl "http://localhost:9200/memories/_search" \
  -H "Content-Type: application/json" \
  -d '{"query": {"term": {"agent_id": "贾维斯"}}}'
```

3. 检查 Embedding 是否正常生成

### 性能问题

**优化建议**:

1. **OpenSearch 调优**:
   - 增加 `ef_search` 参数
   - 调整 JVM 堆内存

2. **Neo4j 调优**:
   - 增加内存配置
   - 优化 Cypher 查询

3. **服务端调优**:
   - 增加并发数
   - 启用连接池
