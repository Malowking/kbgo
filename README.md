# KBGO - 知识库管理系统

基于 Go 开发的 RAG（检索增强生成）知识库管理系统，支持文档解析、向量检索和智能对话。

## 核心功能

### 知识库管理
- 创建、查询、更新、删除知识库
- 支持知识库分类和状态管理

### 文档处理
- 支持文件上传和 URL 导入
- 自动文档解析和分块（chunking）
- 支持文档重新索引
- 文档和分块的状态管理

### 向量检索
- 支持 Milvus 和 pgvector 向量数据库
- 三种检索模式：向量检索、Rerank、RRF（倒数排名融合）
- 支持查询重写优化

### RAG 对话
- 结合知识库的智能问答
- 支持流式和非流式输出
- 支持多模态输入（图片、音频、视频）
- 集成 MCP 工具调用

### 模型管理
- 统一的模型配置管理
- 支持 LLM、Embedding、Rerank、多模态模型
- OpenAI 风格的 API 接口
- 动态模型加载和切换

### MCP 集成
- MCP 服务注册和管理
- 工具发现和调用
- 调用日志和统计

## 技术栈

- **后端框架**: [GoFrame v2](https://goframe.org/)
- **向量数据库**: [Milvus](https://milvus.io/) / PostgreSQL + pgvector
- **关系数据库**: MySQL / PostgreSQL
- **文件存储**: RustFS (MinIO) / 本地文件系统
- **AI 模型**: OpenAI 兼容接口

## 快速启动

### 1. 环境要求

- Go 1.24+
- MySQL 5.7+ 或 PostgreSQL 9.6+
- Milvus 2.6+ 或 PostgreSQL 16+ (with pgvector)

### 2. 配置文件

复制配置模板并修改：

```bash
cp config/config_demo.yaml config/config.yaml
```

### 3. 运行项目

```bash
# 安装依赖
go mod tidy

# 直接运行
go run main.go

# 或编译后运行
go build -o kbgo
./kbgo
```

### 4. 访问服务

- API 服务: http://localhost:8000
- API 文档: http://localhost:8000/swagger/
- 调试工具: 在浏览器中打开 `debug.html`

## 主要 API 接口

### 知识库
- `POST /v1/kb` - 创建知识库
- `GET /v1/kb` - 获取知识库列表
- `GET /v1/kb/{id}` - 获取知识库详情
- `PUT /v1/kb/{id}` - 更新知识库
- `DELETE /v1/kb/{id}` - 删除知识库

### 文档
- `POST /v1/upload` - 上传文件
- `POST /v1/index` - 索引文档（分块+向量化）
- `GET /v1/documents` - 获取文档列表
- `DELETE /v1/documents` - 删除文档
- `POST /v1/documents/reindex` - 重新索引

### 分块
- `GET /v1/chunks` - 获取分块列表
- `PUT /v1/chunks` - 更新分块状态
- `DELETE /v1/chunks` - 删除分块

### 检索
- `POST /v1/retriever` - 向量检索

### 对话
- `POST /v1/chat` - 智能对话（支持流式、多模态、MCP）

### 模型管理
- `POST /v1/model/reload` - 重新加载模型配置
- `GET /v1/model/list` - 获取模型列表
- `POST /v1/model/chat` - OpenAI 风格聊天接口
- `POST /v1/model/embeddings` - Embedding 接口

### MCP
- `POST /v1/mcp/registry` - 注册 MCP 服务
- `GET /v1/mcp/registry` - 获取 MCP 服务列表
- `POST /v1/mcp/call` - 调用 MCP 工具
- `GET /v1/mcp/logs` - 查询 MCP 调用日志

## 项目结构

```
.
├── api/                  # API 接口定义
├── config/               # 配置文件
├── core/                 # 核心业务逻辑
│   ├── chat/            # 对话处理
│   ├── client/          # OpenAI 客户端
│   ├── common/          # 公共工具
│   ├── formatter/       # 消息格式化
│   ├── indexer/         # 文档索引
│   ├── model/           # 模型管理
│   ├── retriever/       # 检索器
│   └── vector_store/    # 向量存储
├── internal/            # 内部实现
│   ├── cmd/            # 命令行入口
│   ├── controller/     # 控制器
│   ├── dao/            # 数据访问层
│   ├── logic/          # 业务逻辑
│   ├── mcp/            # MCP 客户端
│   └── model/          # 数据模型
└── pkg/                 # 公共包
```

## 使用流程

1. **创建知识库**: 调用 `/v1/kb` 创建一个知识库
2. **上传文档**: 使用 `/v1/upload` 上传文档或提供 URL
3. **索引文档**: 调用 `/v1/index` 对文档进行分块和向量化
4. **智能对话**: 使用 `/v1/chat` 进行基于知识库的对话

## License

MIT License