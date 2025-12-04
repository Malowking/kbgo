# KBGO - 知识库管理系统

KBGO 是一个基于 Go 语言开发的知识库管理系统，支持 RAG（检索增强生成）对话、文档索引、向量检索等功能。集成了 Milvus 和 PostgreSQL pgvector 两种向量数据库方案，支持 MCP 协议工具调用。

## 项目简介

本系统提供完整的知识库管理能力，支持从文档上传、解析、分块、向量化到智能检索的全流程。采用分层架构设计，核心功能模块可复用，支持多种数据库和存储方案。

## 核心功能

### 1. 知识库管理
- 创建、查询、更新、删除知识库
- 知识库状态管理（启用/禁用）
- 支持按名称、分类、状态过滤

### 2. 文档管理
- 支持本地文件上传和 URL 导入
- 支持多种文件格式（PDF、HTML、XLSX、Markdown 等）
- 文档列表分页查询
- 文档删除（级联删除 chunks 和向量）
- 文档重新索引

### 3. 文档索引
- 自动化索引流程（加载 → 解析 → 分块 → 嵌入 → 存储）
- 支持递归分块和 Markdown 分块策略
- 可配置 chunk 大小和重叠大小
- 批量文档索引

### 4. 向量检索
- 三种检索模式：milvus（向量搜索）、rerank（向量+重排）、rrf（倒数排名融合）
- 支持查询重写（Query Rewriting）
- Top-K 检索和相似度分数过滤
- 返回文档片段和元数据

### 5. 智能对话（RAG）
- 支持知识库检索增强对话
- 流式/非流式输出
- MCP 工具集成调用
- 多模态文件支持（图片、音频、视频）
- 对话历史管理
- JSON 格式化输出

### 6. 模型管理
- 支持多种模型类型（LLM、Embedding、Rerank、多模态）
- 模型注册、更新、删除
- 模型热重载（无需重启）
- 从数据库动态加载模型配置

### 7. MCP 协议支持
- MCP 服务注册和管理
- 工具发现和调用
- 调用结果日志记录
- 支持多服务工具集成

### 8. 数据块（Chunks）管理
- 查询文档的所有 chunks
- 删除和更新 chunks
- 支持启用/禁用 chunks

## 技术栈

- **后端框架**: Go 1.24+ + [GoFrame v2.9.5](https://goframe.org/)
- **向量数据库**: [Milvus 2.6+](https://milvus.io/) / PostgreSQL + pgvector
- **关系数据库**: MySQL 5.7+ / PostgreSQL 9.6+
- **对象存储**: RustFS/MinIO（可选）或本地文件系统
- **文件解析服务**: Python FastAPI（file_parse）
- **AI 框架**: cloudwego/eino、go-openai

## 环境要求

### 必需环境
- **Go**: 1.24+
- **关系数据库**（二选一）:
  - MySQL 5.7+ （需要 utf8mb4 字符集）
  - PostgreSQL 9.6+ （推荐 14+，支持 pgvector 扩展）
- **向量数据库**（二选一）:
  - Milvus 2.6.x+
  - PostgreSQL + pgvector 扩展

### 可选服务
- **文件解析服务**: Python FastAPI（file_parse，位于 `file_parse/` 目录）
  - 用于解析 PDF、HTML、XLSX 等文件格式
  - 默认端口: 8002
- **对象存储**: RustFS/MinIO（可选，默认使用本地存储）
- **MCP 服务**: 用于工具调用扩展（可选）

## 快速开始

### 1. 配置文件

复制配置文件模板：
```bash
cp config/config_demo.yaml config/config.yaml
```

编辑 `config/config.yaml`，主要配置项说明：

#### 服务器配置
```yaml
server:
  address: ":8000"           # 监听端口
  openapiPath: "/api.json"   # OpenAPI 规范路径
  swaggerPath: "/swagger"    # Swagger UI 路径
```

#### 关系数据库配置（必需）
```yaml
database:
  default:
    host: "localhost"        # 数据库地址
    port: "5432"             # 端口（MySQL: 3306, PostgreSQL: 5432）
    user: "your_user"        # 用户名
    pass: "your_password"    # 密码
    name: "kbgo"             # 数据库名
    type: "pgsql"            # 数据库类型: mysql 或 pgsql
    charset: "utf8mb4"       # 字符集
    maxLifeTime: 3600        # 连接最长存活时间（秒）
```

#### 向量数据库配置（必需）
```yaml
vectorStore:
  type: "pgvector"           # 向量库类型: milvus 或 pgvector

# 方案1：使用 Milvus
milvus:
  address: "http://localhost:19530"
  database: "kbgo"
  dim: 1024                  # 向量维度
  metricType: "COSINE"       # 相似度度量: COSINE/L2/IP

# 方案2：使用 PostgreSQL pgvector（需要先安装 pgvector 扩展）
postgres:
  host: "localhost"
  port: "5432"
  user: "your_user"
  password: "your_password"
  database: "kbgo"
  sslmode: "disable"         # SSL 模式: disable/require/verify-ca/verify-full
  dim: 1024
  metricType: "COSINE"
```

#### 文件存储配置（可选）
```yaml
storage:
  type: "local"              # 存储类型: local 或 rustfs

# 使用 RustFS/MinIO 对象存储（可选）
rustfs:
  endpoint: "localhost:9000"
  accessKey: "your_access_key"
  secretKey: "your_secret_key"
  bucketName: "kbfiles"
  ssl: false
```

#### 检索配置（可选）
```yaml
retriever:
  enableRewrite: false       # 是否启用查询重写
  rewriteAttempts: 3         # 查询重写尝试次数
  retrieveMode: "rerank"     # 检索模式: milvus/rerank/rrf
```

#### 文件解析服务配置（可选）
```yaml
fileParse:
  url: "http://localhost:8002"  # file_parse 服务地址
  timeout: 120                   # 请求超时（秒）
```

### 2. 启动项目

#### 安装依赖
```bash
go mod tidy
```

#### 运行方式

**开发模式（直接运行）**:
```bash
go run main.go
```

**生产模式（编译后运行）**:
```bash
go build -o kbgo main.go
./kbgo
```

#### 访问服务
- **API 地址**: http://localhost:8000
- **Swagger 文档**: http://localhost:8000/swagger/
- **Debug 调试页面**: http://localhost:8000/debug.html（或直接打开本地 debug.html 文件）

## Debug 调试页面

项目提供了一个可视化的 API 调试工具 `debug.html`，可以直接在浏览器中调试所有 API 接口。

**访问方式**:
1. 启动后端服务后，访问: http://localhost:8000/debug.html
2. 或直接用浏览器打开项目根目录的 `debug.html` 文件

**功能特性**:
- 支持所有 API 接口的可视化调试
- 实时显示请求和响应数据
- 支持流式输出调试
- 支持文件上传测试
- 可保存和加载测试配置

---

## API 接口文档

所有接口统一返回格式：
```json
{
  "code": "0",
  "message": "ok",
  "data": { }
}
```

### 1. 知识库管理 API

#### 1.1 创建知识库
**请求**:
```http
POST /api/v1/kb
Content-Type: application/json

{
  "name": "测试知识库",           // 必需，长度 3-50
  "description": "知识库描述",    // 必需，长度 3-200
  "category": "技术文档"          // 可选，长度 3-50
}
```

**响应**:
```json
{
  "code": "0",
  "message": "ok",
  "data": {
    "id": "uuid-string"
  }
}
```

#### 1.2 获取知识库列表
**请求**:
```http
GET /api/v1/kb?name=测试&status=1&category=技术文档
```

**参数说明**:
- `name`: 可选，按名称过滤
- `status`: 可选，状态过滤（1=启用，2=禁用）
- `category`: 可选，按分类过滤

**响应**:
```json
{
  "code": "0",
  "message": "ok",
  "data": {
    "list": [
      {
        "id": "uuid",
        "name": "测试知识库",
        "description": "知识库描述",
        "category": "技术文档",
        "status": 1,
        "created_at": "2024-01-01T00:00:00Z",
        "updated_at": "2024-01-01T00:00:00Z"
      }
    ]
  }
}
```

#### 1.3 获取单个知识库
**请求**:
```http
GET /api/v1/kb/{id}
```

#### 1.4 更新知识库
**请求**:
```http
PUT /api/v1/kb/{id}
Content-Type: application/json

{
  "name": "新名称",              // 可选
  "description": "新描述",       // 可选
  "category": "新分类",          // 可选
  "status": 1                    // 可选，1=启用，2=禁用
}
```

#### 1.5 删除知识库
**请求**:
```http
DELETE /api/v1/kb/{id}
```

#### 1.6 更新知识库状态
**请求**:
```http
PATCH /api/v1/kb/{id}/status
Content-Type: application/json

{
  "status": 1   // 必需，1=启用，2=禁用
}
```

---

### 2. 文档管理 API

#### 2.1 上传文件
**请求**:
```http
POST /api/v1/upload
Content-Type: multipart/form-data

file: [文件]              // 必需（本地文件或 URL 二选一）
url: "https://..."        // 必需（本地文件或 URL 二选一）
knowledge_id: "uuid"      // 必需，知识库 ID
```

**响应**:
```json
{
  "code": "0",
  "message": "ok",
  "data": {
    "document_id": "uuid",
    "status": "pending",
    "message": "文件上传成功"
  }
}
```

#### 2.2 索引文档
**请求**:
```http
POST /api/v1/index
Content-Type: application/json

{
  "document_ids": ["uuid1", "uuid2"],     // 必需，文档 ID 列表
  "embedding_model_id": "uuid",           // 必需，Embedding 模型 UUID
  "chunk_size": 1000,                     // 可选，默认 1000
  "overlap_size": 100,                    // 可选，默认 100
  "separator": "\n\n"                     // 可选，自定义分隔符
}
```

**响应**:
```json
{
  "code": "0",
  "message": "ok",
  "data": {
    "message": "索引任务已启动"
  }
}
```

#### 2.3 获取文档列表
**请求**:
```http
GET /api/v1/documents?knowledge_id=uuid&page=1&size=10
```

**参数说明**:
- `knowledge_id`: 必需，知识库 ID
- `page`: 必需，页码（从 1 开始）
- `size`: 必需，每页大小（1-100）

**响应**:
```json
{
  "code": "0",
  "message": "ok",
  "data": {
    "data": [
      {
        "id": "uuid",
        "knowledge_id": "uuid",
        "file_name": "document.pdf",
        "file_url": "path/to/file",
        "status": 2,
        "chunk_count": 10,
        "created_at": "2024-01-01T00:00:00Z"
      }
    ],
    "total": 100,
    "page": 1,
    "size": 10
  }
}
```

#### 2.4 删除文档
**请求**:
```http
DELETE /api/v1/documents?document_id=uuid
```

**说明**: 会级联删除文档关联的所有 chunks 和向量数据

#### 2.5 重新索引文档
**请求**:
```http
POST /api/v1/documents/reindex
Content-Type: application/json

{
  "document_id": "uuid",        // 必需
  "chunk_size": 1000,           // 可选，默认 1000
  "overlap_size": 100           // 可选，默认 100
}
```

---

### 3. 数据块（Chunks）管理 API

#### 3.1 获取 Chunks 列表
**请求**:
```http
GET /api/v1/chunks?knowledge_doc_id=uuid&page=1&size=10
```

**参数说明**:
- `knowledge_doc_id`: 必需，文档 ID
- `page`: 必需，页码
- `size`: 必需，每页大小

#### 3.2 删除 Chunk
**请求**:
```http
DELETE /api/v1/chunks?id=uuid
```

#### 3.3 更新 Chunks 状态
**请求**:
```http
PUT /api/v1/chunks
Content-Type: application/json

{
  "ids": ["uuid1", "uuid2"],    // 必需，Chunk ID 列表
  "status": 1                    // 必需，0=禁用，1=启用
}
```

---

### 4. 检索 API

#### 4.1 向量检索
**请求**:
```http
POST /api/v1/retriever
Content-Type: application/json

{
  "question": "查询问题",               // 必需
  "knowledge_id": "uuid",              // 必需，知识库 ID
  "embedding_model_id": "uuid",        // 必需，Embedding 模型 UUID
  "rerank_model_id": "uuid",           // 可选，Rerank 模型 UUID（rerank/rrf 模式需要）
  "top_k": 5,                          // 可选，默认 5
  "score": 0.2,                        // 可选，默认 0.2
  "enable_rewrite": false,             // 可选，是否启用查询重写
  "rewrite_attempts": 3,               // 可选，查询重写次数
  "retrieve_mode": "rerank"            // 可选，检索模式: milvus/rerank/rrf
}
```

**响应**:
```json
{
  "code": "0",
  "message": "ok",
  "data": {
    "document": [
      {
        "content": "检索到的文本片段",
        "metadata": {
          "source": "document.pdf",
          "page": 1
        },
        "score": 0.95
      }
    ]
  }
}
```

---

### 5. 聊天对话 API

#### 5.1 智能对话（统一接口）
**请求**:
```http
POST /api/v1/chat
Content-Type: multipart/form-data

conv_id: "uuid"                          // 必需，会话 ID
question: "你的问题"                      // 必需
model_id: "uuid"                         // 必需，LLM 模型 UUID
embedding_model_id: "uuid"               // 可选，Embedding 模型 UUID（启用检索时需要）
rerank_model_id: "uuid"                  // 可选，Rerank 模型 UUID（rerank/rrf 模式需要）
knowledge_id: "uuid"                     // 可选，知识库 ID
enable_retriever: true                   // 可选，是否启用知识库检索
top_k: 5                                 // 可选，检索 Top-K
score: 0.2                               // 可选，相似度阈值
retrieve_mode: "rerank"                  // 可选，检索模式
use_mcp: true                            // 可选，是否使用 MCP
mcp_service_tools: {"service1": ["tool1", "tool2"]}  // 可选，MCP 工具配置
stream: true                             // 可选，是否流式输出
jsonformat: false                        // 可选，是否 JSON 格式化输出
files: [文件1, 文件2]                    // 可选，多模态文件（图片/音频/视频）
```

**响应（非流式）**:
```json
{
  "code": "0",
  "message": "ok",
  "data": {
    "answer": "AI 回答内容",
    "references": [
      {
        "content": "参考文本",
        "metadata": {"source": "doc.pdf"},
        "score": 0.95
      }
    ],
    "mcp_results": [
      {
        "service_name": "服务名",
        "tool_name": "工具名",
        "content": "工具调用结果"
      }
    ]
  }
}
```

**响应（流式）**:
```
Content-Type: text/event-stream

data: {"content": "部"}
data: {"content": "分"}
data: {"content": "回"}
data: {"content": "答"}
data: [DONE]
```

---

### 6. 模型管理 API

#### 6.1 获取模型列表
**请求**:
```http
GET /api/v1/model/list?model_type=llm
```

**参数说明**:
- `model_type`: 可选，按类型过滤（llm, embedding, reranker, multimodal, image, video, audio）

#### 6.2 注册模型
**请求**:
```http
POST /api/v1/model/register
Content-Type: application/json

{
  "model_name": "gpt-4",                   // 必需
  "model_type": "llm",                     // 必需
  "provider": "openai",                    // 可选
  "base_url": "https://api.openai.com/v1", // 可选
  "api_key": "sk-xxx",                     // 可选
  "capabilities": ["chat", "function_calling"], // 可选
  "context_window": 8192,                  // 可选
  "max_completion_tokens": 4096,           // 可选
  "dimension": 1536,                       // 可选（embedding 模型专用）
  "config": {},                            // 可选，其他配置
  "enabled": true,                         // 可选，默认 true
  "description": "模型描述"                 // 可选
}
```

#### 6.3 更新模型
**请求**:
```http
PUT /api/v1/model/{model_id}
Content-Type: application/json

{
  "model_name": "新名称",     // 可选
  "base_url": "新地址",       // 可选
  "api_key": "新密钥",        // 可选
  "enabled": false            // 可选
  // ... 其他可选字段
}
```

#### 6.4 删除模型
**请求**:
```http
DELETE /api/v1/model/{model_id}
```

#### 6.5 重新加载模型配置
**请求**:
```http
POST /api/v1/model/reload
```

**说明**: 从数据库重新加载所有模型配置，无需重启服务

---

### 7. MCP 协议 API

#### 7.1 注册 MCP 服务
**请求**:
```http
POST /api/v1/mcp/registry
Content-Type: application/json

{
  "name": "服务名称",                // 必需，唯一
  "description": "服务描述",         // 可选
  "endpoint": "http://localhost:3000/sse", // 必需，SSE 端点
  "api_key": "密钥",                 // 可选
  "headers": "{\"key\": \"value\"}", // 可选，JSON 格式
  "timeout": 30                      // 可选，超时时间（秒）
}
```

#### 7.2 获取 MCP 服务列表
**请求**:
```http
GET /api/v1/mcp/registry?status=1&page=1&page_size=10
```

#### 7.3 获取服务的工具列表
**请求**:
```http
GET /api/v1/mcp/registry/{id}/tools?cached=true&cache_ttl=300
```

#### 7.4 调用 MCP 工具
**请求**:
```http
POST /api/v1/mcp/call
Content-Type: application/json

{
  "registry_id": "uuid",           // 必需，MCP 服务 ID 或名称
  "tool_name": "工具名称",         // 必需
  "arguments": {                   // 可选，工具参数
    "param1": "value1"
  },
  "conversation_id": "uuid"        // 可选，用于日志关联
}
```

#### 7.5 获取调用日志
**请求**:
```http
GET /api/v1/mcp/logs?conversation_id=uuid&status=1&page=1&page_size=10
```

#### 7.6 获取服务统计
**请求**:
```http
GET /api/v1/mcp/registry/{id}/stats
```

**响应**:
```json
{
  "code": "0",
  "message": "ok",
  "data": {
    "total_calls": 100,
    "success_calls": 95,
    "failed_calls": 5,
    "avg_duration": 123.45
  }
}
```

## 项目结构

```
.
├── api                 # API 接口定义
├── config              # 配置文件
├── core                # 核心业务逻辑
├── internal            # 内部实现
│   ├── cmd             # 命令行入口
│   ├── controller      # 控制器
│   ├── dao             # 数据访问对象
│   ├── logic           # 业务逻辑层
│   ├── mcp             # MCP 协议实现
│   └── model           # 数据模型
```

## 许可证

MIT License