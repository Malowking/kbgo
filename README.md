# KBGO - 知识库管理系统

KBGO 是一个基于 Go 语言开发的知识库管理系统，集成了 Milvus 向量数据库，支持文档索引、检索增强生成（RAG）等功能。

## 功能特性

- 知识库管理：创建、查询、更新、删除知识库
- 文档管理：支持多种格式文档的上传和管理
- 向量检索：基于 Milvus 的高效相似度检索
- RAG 对话：结合知识库内容进行智能问答
- API 调试：提供完整的 HTML 调试界面

## 技术栈

- 后端框架：Go + [GoFrame](https://goframe.org/)
- 向量数据库：[Milvus](https://milvus.io/)
- 关系数据库：MySQL
- 对象存储：RustFS
- 前端技术：HTML/CSS/JavaScript(后段开发测试使用)

## 快速开始

### 环境要求

- Go 1.24+
- MySQL 5.7+
- Milvus 2.4+
- RustFS

### 配置文件

1. 复制配置文件模板：
   ```bash
   cp config/config_demo.yaml config/config.yaml
   ```

2. 修改 `config/config.yaml` 中的配置项：
   ```yaml
   # 数据库配置
   database:
     default:
       host: "localhost" # MySQL地址
       port: "3306"      # MySQL端口
       user: "root"      # 用户名
       pass: "password"  # 密码
       name: "kbgo"      # 数据库名
   
   # Milvus向量数据库配置
   milvus:
     address: "http://localhost:19530"
     database: "kbgo"
   
   # 其他配置...
   ```

### 启动项目

1. 安装依赖：
   ```bash
   go mod tidy
   ```

2. 运行项目：
   ```bash
   go run main.go
   ```

   或者编译后运行：
   ```bash
   go build -o kbgo main.go
   ./kbgo
   ```

3. 访问服务：
   - API 地址: http://localhost:8000
   - API 文档: http://localhost:8000/swagger/

## API 调试工具

项目提供了一个可视化的 API 调试工具 `debug.html`，可以直接通过浏览器打开该文件来调试各种 API 接口：

1. 在浏览器中打开 `debug.html` 文件
2. 确保后端服务正在运行
3. 设置正确的 API 地址（默认为 http://localhost:8000）
4. 选择相应的 API 接口进行测试

支持的调试功能包括：
- 知识库管理（创建、查询、更新、删除）
- 文档管理（上传、删除）
- Chunk 管理
- 索引操作
- 检索功能
- 对话接口（普通和流式）

## 主要 API 接口

### 知识库相关
- `POST /api/v1/knowledge-base` - 创建知识库
- `GET /api/v1/knowledge-base/list` - 获取知识库列表
- `PUT /api/v1/knowledge-base/{id}` - 更新知识库
- `DELETE /api/v1/knowledge-base/{id}` - 删除知识库

### 文档相关
- `POST /api/v1/indexer/upload` - 上传文档并建立索引
- `POST /api/v1/indexer/url` - 通过 URL 添加文档并建立索引
- `GET /api/v1/documents/list` - 获取文档列表
- `DELETE /api/v1/documents/{id}` - 删除文档

### 检索相关
- `POST /api/v1/retriever/search` - 标准检索

### 对话相关
- `POST /api/v1/chat/completion` - 普通对话
- `POST /api/v1/chat/completion/stream` - 流式对话

## 项目结构

```
.
├── api                 # API 接口定义
├── core                # 核心业务逻辑
├── internal            # 内部实现
│   ├── cmd             # 命令行入口
│   ├── controller      # 控制器
│   ├── dao             # 数据访问对象
│   ├── logic           # 业务逻辑层
│   ├── mcp             # MCP 协议实现
│   └── model           # 数据模型
├── manifest            # 配置文件
└── milvus_new          # Milvus 相关封装
```

## 许可证

MIT License