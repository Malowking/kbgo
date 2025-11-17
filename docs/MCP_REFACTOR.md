# MCP 注册服务重构说明

## 概述

本次重构完全取消了内置的 MCP 服务器，改为支持用户自行启动 MCP 服务并注册到系统中。系统作为 MCP 客户端，通过 SSE (Server-Sent Events) 协议与外部 MCP 服务通信。

## 架构变更

### 旧架构
- 系统内置 MCP 服务器
- 通过 `/mcp` 端点暴露 MCP 协议
- 提供内置的知识库查询、检索等工具

### 新架构
- 系统作为 MCP 客户端
- 用户注册外部 MCP 服务（SSE 格式）
- 通过 RESTful API 调用注册的 MCP 工具
- 完整的调用日志记录和统计

## 数据库表结构

### 1. mcp_registry - MCP 服务注册表

存储已注册的外部 MCP 服务信息。

| 字段 | 类型 | 说明 |
|------|------|------|
| id | varchar(64) | 主键，格式：mcp_uuid |
| name | varchar(100) | 服务名称（唯一） |
| description | varchar(500) | 服务描述 |
| endpoint | varchar(500) | SSE 端点 URL |
| api_key | varchar(500) | 认证密钥 |
| headers | text | 自定义请求头（JSON） |
| timeout | int | 超时时间（秒），默认 30 |
| status | tinyint | 状态：1-启用，0-禁用 |
| create_time | datetime | 创建时间 |
| update_time | datetime | 更新时间 |

### 2. mcp_call_log - MCP 调用日志表

记录每次 MCP 工具调用的详细信息。

| 字段 | 类型 | 说明 |
|------|------|------|
| id | varchar(64) | 主键 |
| conversation_id | varchar(255) | 对话 ID（关联 chat-history） |
| mcp_registry_id | varchar(64) | MCP 服务 ID |
| mcp_service_name | varchar(100) | MCP 服务名称快照 |
| tool_name | varchar(100) | 调用的工具名称 |
| request_payload | text | 请求参数（JSON） |
| response_payload | text | 响应结果（JSON） |
| status | tinyint | 状态：1-成功，0-失败，2-超时 |
| error_message | text | 错误信息 |
| duration | int | 调用耗时（毫秒） |
| create_time | datetime | 创建时间 |

## API 接口

### MCP 服务注册管理

#### 1. 创建 MCP 服务注册
```http
POST /api/v1/mcp/registry
Content-Type: application/json

{
  "name": "my-mcp-service",
  "description": "我的 MCP 服务",
  "endpoint": "http://localhost:3000/sse",
  "api_key": "optional-api-key",
  "headers": "{\"X-Custom-Header\": \"value\"}",
  "timeout": 30
}
```

#### 2. 更新 MCP 服务
```http
PUT /api/v1/mcp/registry/{id}
Content-Type: application/json

{
  "name": "updated-service-name",
  "description": "更新后的描述",
  "status": 1
}
```

#### 3. 删除 MCP 服务
```http
DELETE /api/v1/mcp/registry/{id}
```

#### 4. 获取 MCP 服务列表
```http
GET /api/v1/mcp/registry?status=1&page=1&page_size=10
```

#### 5. 获取单个 MCP 服务
```http
GET /api/v1/mcp/registry/{id}
```

#### 6. 更新服务状态
```http
PATCH /api/v1/mcp/registry/{id}/status
Content-Type: application/json

{
  "status": 1
}
```

#### 7. 测试服务连通性
```http
POST /api/v1/mcp/registry/{id}/test
```

### MCP 工具调用

#### 8. 列出服务的所有工具
```http
GET /api/v1/mcp/registry/{id}/tools
```

响应示例：
```json
{
  "tools": [
    {
      "name": "search",
      "description": "搜索工具",
      "inputSchema": {
        "type": "object",
        "properties": {
          "query": {
            "type": "string",
            "description": "搜索关键词"
          }
        },
        "required": ["query"]
      }
    }
  ]
}
```

#### 9. 调用 MCP 工具
```http
POST /api/v1/mcp/call
Content-Type: application/json

{
  "registry_id": "mcp_xxx",  // 或服务名称
  "tool_name": "search",
  "arguments": {
    "query": "test query"
  },
  "conversation_id": "conv_123"  // 可选，用于日志关联
}
```

响应示例：
```json
{
  "content": [
    {
      "type": "text",
      "text": "搜索结果内容..."
    }
  ],
  "is_error": false,
  "log_id": "log_xxx"
}
```

### MCP 调用日志查询

#### 10. 查询调用日志列表
```http
GET /api/v1/mcp/logs?conversation_id=conv_123&page=1&page_size=10
```

支持的过滤参数：
- `conversation_id`: 对话 ID
- `registry_id`: MCP 服务 ID
- `service_name`: MCP 服务名称
- `tool_name`: 工具名称
- `status`: 状态（1-成功，0-失败，2-超时）
- `start_time`: 开始时间（RFC3339 格式）
- `end_time`: 结束时间（RFC3339 格式）

#### 11. 根据对话 ID 查询日志
```http
GET /api/v1/mcp/logs/conversation/{conversation_id}?page=1&page_size=10
```

#### 12. 获取服务统计信息
```http
GET /api/v1/mcp/registry/{id}/stats
```

响应示例：
```json
{
  "total_calls": 1000,
  "success_calls": 950,
  "failed_calls": 50,
  "avg_duration": 234.5
}
```

## 使用流程

### 1. 启动外部 MCP 服务

用户需要自行启动一个 MCP 服务，该服务需要：
- 支持 MCP 协议（JSON-RPC 2.0）
- 通过 SSE (Server-Sent Events) 格式返回响应
- 实现必要的 MCP 方法：
  - `initialize`: 初始化连接
  - `tools/list`: 列出可用工具
  - `tools/call`: 调用工具

### 2. 注册 MCP 服务

通过 API 将 MCP 服务注册到系统：

```bash
curl -X POST http://localhost:8000/api/v1/mcp/registry \
  -H "Content-Type: application/json" \
  -d '{
    "name": "my-knowledge-service",
    "description": "知识库查询服务",
    "endpoint": "http://localhost:3000/sse",
    "timeout": 30
  }'
```

### 3. 测试连通性

```bash
curl -X POST http://localhost:8000/api/v1/mcp/registry/{id}/test
```

### 4. 查看可用工具

```bash
curl http://localhost:8000/api/v1/mcp/registry/{id}/tools
```

### 5. 调用工具

```bash
curl -X POST http://localhost:8000/api/v1/mcp/call \
  -H "Content-Type: application/json" \
  -d '{
    "registry_id": "my-knowledge-service",
    "tool_name": "search",
    "arguments": {
      "query": "测试查询"
    },
    "conversation_id": "conv_123"
  }'
```

### 6. 查看调用日志

```bash
curl "http://localhost:8000/api/v1/mcp/logs?conversation_id=conv_123"
```

## MCP 客户端实现

系统实现了完整的 MCP SSE 客户端（`internal/mcp/client/mcp_client.go`），支持：

- **连接初始化**: `Initialize()`
- **工具列表查询**: `ListTools()`
- **工具调用**: `CallTool()`
- **连通性测试**: `Ping()`
- **SSE 协议支持**: 完整的 Server-Sent Events 解析
- **自定义请求头**: 支持自定义 HTTP 头
- **认证支持**: Bearer Token 认证
- **超时控制**: 可配置的超时时间

## 代码结构

```
internal/
├── model/gorm/
│   ├── mcp_registry.go        # MCP 注册表模型
│   ├── mcp_call_log.go        # MCP 调用日志模型
│   └── migrate.go             # 数据库迁移（已更新）
├── dao/
│   ├── mcp_registry.go        # MCP 注册表 DAO
│   └── mcp_call_log.go        # MCP 调用日志 DAO
├── mcp/
│   ├── client/
│   │   └── mcp_client.go      # MCP SSE 客户端
│   └── old/                   # 旧的 MCP 服务代码（已归档）
└── controller/rag/
    └── rag_v1_mcp.go          # MCP API 控制器

api/rag/v1/
└── mcp.go                     # MCP API 定义
```

## 迁移说明

### 旧代码处理

旧的 MCP 服务器代码已移动到 `internal/mcp/old/` 目录，包括：
- `mcp.go`: MCP 主模块
- `knowledgebase.go`: 知识库工具
- `retriever.go`: 检索工具
- `indexer.go`: 索引工具

这些文件已不再使用，仅供参考。

### 兼容性

- **不兼容**: 旧的 `/mcp` 端点已移除
- **新方案**: 需要用户自行启动并注册 MCP 服务
- **数据库**: 自动创建新表，不影响现有数据

## 注意事项

1. **SSE 格式要求**: 外部 MCP 服务必须使用 SSE 格式返回响应
2. **MCP 协议版本**: 支持 MCP 协议版本 `2024-11-05`
3. **认证**: 支持 Bearer Token 认证和自定义请求头
4. **超时**: 默认 30 秒超时，可配置
5. **日志记录**: 所有调用自动记录到 `mcp_call_log` 表
6. **错误处理**: 完整的错误信息和状态码支持

## 示例 MCP 服务

如需实现一个简单的 MCP 服务，可以参考旧代码（`internal/mcp/old/`）并按照 SSE 格式改造。

关键要点：
- 使用 `Content-Type: text/event-stream`
- 响应格式：`data: {json}\n\n`
- 支持 JSON-RPC 2.0 格式
- 实现必要的 MCP 方法

## 后续计划

- [ ] 添加 MCP 服务健康检查
- [ ] 支持 MCP 服务版本管理
- [ ] 实现工具调用缓存
- [ ] 添加 MCP 调用限流
- [ ] 支持批量工具调用
- [ ] 实现 MCP 服务自动发现

## 参考文档

- [MCP 协议规范](https://spec.modelcontextprotocol.io/)
- [Server-Sent Events](https://developer.mozilla.org/en-US/docs/Web/API/Server-sent_events)
- [JSON-RPC 2.0](https://www.jsonrpc.org/specification)