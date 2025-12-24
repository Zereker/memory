# API 文档

Memory System 提供 RESTful HTTP API，用于存储和检索 AI Agent 的对话记忆。

## 基础信息

- **Base URL**: `http://localhost:8080/api/v1`
- **Content-Type**: `application/json`
- **字符编码**: UTF-8

## 响应格式

所有接口统一返回格式：

```json
{
  "success": true,
  "data": { ... },
  "error": "错误信息（仅失败时）"
}
```

---

## 接口列表

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | /api/v1/memories/add | 添加记忆 |
| POST | /api/v1/memories/retrieve | 检索记忆 |
| DELETE | /api/v1/memories/{id} | 删除记忆 |
| GET | /health | 健康检查 |

---

## 添加记忆

**POST /api/v1/memories/add**

存储对话消息，自动提取实体、关系并生成摘要。

### 请求参数

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| agent_id | string | 否 | AI Agent 标识，可从 messages 推断 |
| user_id | string | 否 | 用户标识，可从 messages 推断 |
| session_id | string | 是 | 会话 ID |
| messages | array | 是 | 对话消息列表 |

**Message 结构**:

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| role | string | 是 | 角色：user / assistant / system |
| content | string | 是 | 消息内容 |
| name | string | 否 | 发言者名称 |

### 请求示例

```bash
curl -X POST "http://localhost:8080/api/v1/memories/add" \
  -H "Content-Type: application/json" \
  -d '{
    "agent_id": "贾维斯",
    "user_id": "阿信",
    "session_id": "session-2024-01-01",
    "messages": [
      {
        "role": "user",
        "name": "阿信",
        "content": "我今天去了星巴克喝咖啡，点了一杯拿铁"
      },
      {
        "role": "assistant",
        "name": "贾维斯",
        "content": "拿铁是个不错的选择！你喜欢加糖还是原味？"
      },
      {
        "role": "user",
        "name": "阿信",
        "content": "我喜欢原味的，不加糖"
      }
    ]
  }'
```

### 响应示例

```json
{
  "success": true,
  "data": {
    "success": true,
    "episodes": [
      {
        "id": "ep_a1b2c3d4",
        "agent_id": "贾维斯",
        "user_id": "阿信",
        "session_id": "session-2024-01-01",
        "role": "user",
        "name": "阿信",
        "content": "我今天去了星巴克喝咖啡，点了一杯拿铁",
        "timestamp": "2024-01-01T10:30:00Z"
      },
      {
        "id": "ep_e5f6g7h8",
        "role": "assistant",
        "name": "贾维斯",
        "content": "拿铁是个不错的选择！你喜欢加糖还是原味？",
        "timestamp": "2024-01-01T10:30:01Z"
      },
      {
        "id": "ep_i9j0k1l2",
        "role": "user",
        "name": "阿信",
        "content": "我喜欢原味的，不加糖",
        "timestamp": "2024-01-01T10:30:02Z"
      }
    ],
    "entities": [
      {
        "id": "ent_m3n4o5p6",
        "name": "阿信",
        "type": "person",
        "description": "用户"
      },
      {
        "id": "ent_q7r8s9t0",
        "name": "星巴克",
        "type": "place",
        "description": "咖啡店"
      },
      {
        "id": "ent_u1v2w3x4",
        "name": "拿铁",
        "type": "thing",
        "description": "咖啡饮品"
      }
    ],
    "edges": [
      {
        "id": "edge_y5z6a7b8",
        "source_id": "ent_m3n4o5p6",
        "target_id": "ent_q7r8s9t0",
        "relation": "去",
        "fact": "阿信今天去了星巴克"
      },
      {
        "id": "edge_c9d0e1f2",
        "source_id": "ent_m3n4o5p6",
        "target_id": "ent_u1v2w3x4",
        "relation": "喜欢",
        "fact": "阿信喜欢喝原味拿铁，不加糖"
      }
    ],
    "summaries": [
      {
        "id": "sum_g3h4i5j6",
        "topic": "咖啡偏好",
        "content": "用户阿信喜欢去星巴克喝咖啡，偏好原味拿铁不加糖"
      }
    ]
  }
}
```

### 错误响应

```json
{
  "success": false,
  "error": "session_id is required"
}
```

---

## 检索记忆

**POST /api/v1/memories/retrieve**

根据查询语句检索相关记忆，支持多层混合检索。

### 请求参数

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| agent_id | string | 是 | AI Agent 标识 |
| user_id | string | 是 | 用户标识 |
| session_id | string | 否 | 会话 ID（可选过滤） |
| query | string | 是 | 查询语句 |
| limit | int | 否 | 返回数量限制，默认 10 |
| options | object | 否 | 检索选项 |

**Options 结构**:

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| include_episodes | bool | true | 是否检索 Episode |
| include_entities | bool | true | 是否检索 Entity |
| include_edges | bool | true | 是否检索 Edge |
| include_summaries | bool | false | 是否检索 Summary |
| max_hops | int | 0 | 图遍历最大跳数 |

### 请求示例

```bash
curl -X POST "http://localhost:8080/api/v1/memories/retrieve" \
  -H "Content-Type: application/json" \
  -d '{
    "agent_id": "贾维斯",
    "user_id": "阿信",
    "query": "用户喜欢喝什么饮料",
    "limit": 10,
    "options": {
      "include_episodes": true,
      "include_entities": true,
      "include_edges": true,
      "include_summaries": true,
      "max_hops": 2
    }
  }'
```

### 响应示例

```json
{
  "success": true,
  "data": {
    "success": true,
    "episodes": [
      {
        "id": "ep_a1b2c3d4",
        "content": "我今天去了星巴克喝咖啡，点了一杯拿铁",
        "role": "user",
        "name": "阿信",
        "score": 0.89,
        "timestamp": "2024-01-01T10:30:00Z"
      },
      {
        "id": "ep_i9j0k1l2",
        "content": "我喜欢原味的，不加糖",
        "role": "user",
        "name": "阿信",
        "score": 0.85,
        "timestamp": "2024-01-01T10:30:02Z"
      }
    ],
    "entities": [
      {
        "id": "ent_u1v2w3x4",
        "name": "拿铁",
        "type": "thing",
        "description": "咖啡饮品",
        "score": 0.92
      },
      {
        "id": "ent_q7r8s9t0",
        "name": "星巴克",
        "type": "place",
        "description": "咖啡店",
        "score": 0.78
      }
    ],
    "edges": [
      {
        "id": "edge_c9d0e1f2",
        "fact": "阿信喜欢喝原味拿铁，不加糖",
        "relation": "喜欢",
        "score": 0.95
      }
    ],
    "summaries": [
      {
        "id": "sum_g3h4i5j6",
        "topic": "咖啡偏好",
        "content": "用户阿信喜欢去星巴克喝咖啡，偏好原味拿铁不加糖",
        "score": 0.88
      }
    ],
    "total": 6,
    "memory_context": "## 主题摘要\n- [咖啡偏好] 用户阿信喜欢去星巴克喝咖啡，偏好原味拿铁不加糖\n\n## 用户信息\n- 阿信喜欢喝原味拿铁，不加糖\n\n## 相关对话记录\n- [阿信] 我今天去了星巴克喝咖啡，点了一杯拿铁\n- [阿信] 我喜欢原味的，不加糖"
  }
}
```

### 响应字段说明

| 字段 | 说明 |
|------|------|
| episodes | 匹配的原始对话，按相关度排序 |
| entities | 匹配的实体 |
| edges | 匹配的关系/事实 |
| summaries | 匹配的主题摘要 |
| total | 结果总数 |
| memory_context | 格式化的记忆上下文，可直接用于 LLM prompt |

### memory_context 使用

`memory_context` 字段提供格式化的记忆文本，可以直接拼接到 LLM 的 system prompt 中：

```
你是贾维斯，一个智能助手。

以下是关于用户的记忆信息：
{memory_context}

请基于以上信息回答用户的问题。
```

---

## 删除记忆

**DELETE /api/v1/memories/{id}**

删除指定的记忆。

### 路径参数

| 参数 | 说明 |
|------|------|
| id | 记忆 ID（Episode/Entity/Edge/Community） |

### 请求示例

```bash
curl -X DELETE "http://localhost:8080/api/v1/memories/ep_a1b2c3d4"
```

### 响应示例

```json
{
  "success": true,
  "data": {
    "deleted": "ep_a1b2c3d4"
  }
}
```

---

## 健康检查

**GET /health**

检查服务健康状态。

### 请求示例

```bash
curl "http://localhost:8080/health"
```

### 响应示例

```json
{
  "success": true,
  "data": {
    "status": "healthy"
  }
}
```

---

## 错误码

| HTTP 状态码 | 说明 |
|------------|------|
| 200 | 成功 |
| 400 | 请求参数错误 |
| 500 | 服务器内部错误 |

---

## 使用建议

### 1. 分批存储

对于长对话，建议分批存储，每批 5-10 条消息：

```python
# 不推荐：一次存储整个对话
add_memory(messages=all_100_messages)

# 推荐：分批存储
for batch in chunks(messages, size=10):
    add_memory(messages=batch)
```

### 2. 检索选项优化

根据场景选择合适的检索层：

```python
# 场景1：需要完整上下文
options = {"include_episodes": True, "include_summaries": False}

# 场景2：快速了解主题摘要
options = {"include_episodes": False, "include_summaries": True}

# 场景3：需要关联推理
options = {"include_edges": True, "max_hops": 2}
```

### 3. 利用 memory_context

直接使用 `memory_context` 作为 LLM 上下文，无需自行拼接：

```python
response = retrieve_memory(query="用户喜欢什么")
context = response["data"]["memory_context"]

llm_prompt = f"""
{context}

用户问题：{user_question}
"""
```

### 4. 会话 ID 管理

使用有意义的会话 ID，便于追踪和调试：

```python
# 推荐：包含日期和用户信息
session_id = f"session-{user_id}-{date}"

# 不推荐：随机 ID
session_id = str(uuid.uuid4())
```
