# File Parse Service

一个强大的文件解析服务，能够将各种格式的文档转换为 Markdown 格式，支持文本分块和图片处理。

## 功能特性

- 📄 **多格式支持**: 支持 PDF, DOCX, XLSX, PPTX, HTML, TXT, MD, CSV, JSON 等多种格式
- 🔍 **智能分块**: 根据配置自动将长文本分割成多个块，支持自定义分块大小和重叠
- 🖼️ **图片处理**: 自动提取文档中的图片，转换为 URL，支持图片去重
- 🚀 **高性能**: 异步处理，支持并发请求
- 🔧 **易于配置**: 通过环境变量或配置文件灵活配置
- 📊 **完整日志**: 详细的日志记录，便于调试和监控
- 🌐 **RESTful API**: 标准的 RESTful API 接口，易于集成

## 项目结构

- `app/` - 应用代码（API 路由、核心业务逻辑、配置管理）
- `logs/` - 日志目录
- `tests/` - 测试文件
- `start.sh` / `stop.sh` - 启动/停止脚本

## 快速开始

### 前置要求

- Python 3.9+
- Poetry（依赖管理工具）
- LibreOffice（可选，用于处理 EMF/WMF 格式图片）

### 安装 LibreOffice（可选）

LibreOffice 用于将 Word 文档中的 EMF/WMF 矢量图片转换为 JPEG 格式。如果你需要处理包含这类图片的文档，建议安装。

- **macOS**: `brew install --cask libreoffice`
- **Ubuntu/Debian**: `sudo apt-get install libreoffice`
- **CentOS/RHEL**: `sudo yum install libreoffice`

### 安装 Poetry 和依赖

1. 安装 Poetry: `curl -sSL https://install.python-poetry.org | python3 -`（或访问 [Poetry 官方文档](https://python-poetry.org/docs/#installation)）
2. 安装依赖: `poetry install` （生产环境: `poetry install --only main`）

### 配置

创建 `.env` 文件（**必须配置**）：

```bash
cp .env.example .env
```

编辑 `.env` 文件，**必须设置 IMAGE_DIR**：

```bash
# 服务配置
HOST=127.0.0.1
PORT=8002
DEBUG=False

# 路径配置（必须配置）
IMAGE_DIR=/path/to/your/image/directory  # 图片存储目录，绝对路径

# 日志级别
LOG_LEVEL=INFO

# 分块配置
DEFAULT_CHUNK_SIZE=1000
DEFAULT_CHUNK_OVERLAP=100

# 图片配置
MAX_IMAGE_SIZE=(1024, 1024)
```

**重要说明：**
- `IMAGE_DIR` 必须配置为绝对路径，例如 `/Users/wing/kbgo/upload/images` 或 `/var/www/file_parse/images`
- 日志文件会自动存储在项目的 `logs/` 目录下
- 服务启动时会自动创建 `IMAGE_DIR` 目录（如果不存在）

### 启动服务

**开发模式**（支持热重载）: `./start.sh dev`

**生产模式**（后台运行）: `./start.sh prod`

**停止服务**: `./stop.sh`

或使用 Poetry 直接运行:
- 开发: `poetry run uvicorn app.main:app --reload`
- 生产: `poetry run uvicorn app.main:app`

服务启动后访问:
- API 文档: http://127.0.0.1:8002/docs
- 健康检查: http://127.0.0.1:8002/health

## API 接口

### 1. 解析文件

**POST** `/parse`

将文件解析为分块的 Markdown 文本。

**请求体：**

```json
{
  "file_path": "/path/to/document.pdf",
  "chunk_size": 1000,
  "chunk_overlap": 100,
  "separators": ["\n\n", "\n", " "]
}
```

**响应：**

```json
{
  "success": true,
  "result": [
    {
      "chunk_index": 0,
      "text": "文本内容...",
      "image_urls": ["http://127.0.0.1:8002/images/abc123.jpg"]
    }
  ],
  "image_urls": ["http://127.0.0.1:8002/images/abc123.jpg"],
  "total_chunks": 5,
  "total_images": 3,
  "file_info": {
    "name": "document.pdf",
    "size": 102400,
    "extension": ".pdf",
    "path": "/path/to/document.pdf"
  }
}
```

### 2. 健康检查

**GET** `/health`

检查服务状态。

**响应：**

```json
{
  "status": "healthy",
  "message": "File Parse Service is running",
  "version": "1.0.0"
}
```

### 3. 获取支持的文件格式

**GET** `/supported-formats`

获取支持的文件格式列表。

**响应：**

```json
{
  "supported_formats": [".txt", ".md", ".pdf", ".docx", "..."],
  "description": "List of supported file formats for parsing"
}
```

### 4. 获取配置信息

**GET** `/config`

获取当前服务配置。

**响应：**

```json
{
  "chunk_size_range": {
    "min": 100,
    "max": 100000,
    "default": 1000
  },
  "default_chunk_overlap": 100,
  "default_separators": ["\n\n", "\n", " ", ""],
  "max_image_size": [1024, 1024],
  "supported_formats": ["..."]
}
```

## 使用示例

服务支持通过 HTTP API 调用，可以使用任何支持 HTTP 请求的语言或工具。

**基本调用方式:**
- POST `/parse` - 解析文件（需提供 file_path, chunk_size, chunk_overlap 等参数）
- 返回分块的 Markdown 文本和图片 URL 列表

**支持的客户端:**
- Python: 使用 `requests` 库发送 POST 请求
- cURL: 使用 `curl -X POST` 命令
- Go: 使用 `http.Post` 方法与 kbgo 项目集成

详细的 API 接口说明请访问 http://127.0.0.1:8002/docs

## 配置说明

### 环境变量

| 变量名 | 说明 | 默认值 | 是否必填 |
|--------|------|--------|---------|
| HOST | 服务监听地址 | 127.0.0.1 | 否 |
| PORT | 服务监听端口 | 8002 | 否 |
| DEBUG | 调试模式 | False | 否 |
| IMAGE_DIR | 图片存储目录（绝对路径） | - | **是** |
| LOG_LEVEL | 日志级别 | INFO | 否 |
| DEFAULT_CHUNK_SIZE | 默认分块大小 | 1000 | 否 |
| DEFAULT_CHUNK_OVERLAP | 默认重叠大小 | 100 | 否 |

### 分块策略

服务支持智能分块，会尝试在以下位置切分文本：

1. 段落边界 (`\n\n`)
2. 行边界 (`\n`)
3. 空格 (` `)
4. 任意位置（如果找不到更好的切分点）

同时会避免切断图片 URL，确保图片引用完整。

## 开发

- **运行测试**: `pytest tests/`
- **代码格式化**: `black app/`
- **类型检查**: `mypy app/`

## 日志

日志文件位于 `logs/` 目录：

- `file_parse.log`: 主日志文件
- `parser.log`: 解析器日志
- `chunker.log`: 分块器日志
- `image_handler.log`: 图片处理日志
- `api.log`: API 日志

## 性能优化建议

1. **调整分块大小**: 根据实际需求调整 `chunk_size`，更大的值会减少块数量但增加每块的大小
2. **图片压缩**: 服务会自动将图片缩放到 1024x1024，可以通过配置调整
3. **并发处理**: 服务使用异步处理，可以同时处理多个请求
4. **缓存**: 建议在生产环境中使用 Redis 等缓存常用文件的解析结果

## 故障排除

### 服务无法启动

1. **IMAGE_DIR 未配置**：
   - 错误信息：`IMAGE_DIR must be configured`
   - 解决方法：在 `.env` 文件中设置 `IMAGE_DIR` 环境变量为绝对路径

2. 检查端口是否被占用：`lsof -i :8002`
3. 检查依赖是否安装完整：`poetry show`
4. 查看日志文件：`tail -f logs/file_parse.log`

### 文件解析失败

1. 确认文件格式是否支持：访问 `/supported-formats`
2. 检查文件路径是否正确
3. 确认文件没有损坏
4. 查看详细错误日志

### 图片无法显示

1. 确认图片目录权限：`ls -la upload/images/`
2. 检查静态文件服务是否正常：访问 `/images/`
3. 确认防火墙设置

## 许可证

MIT License

## 贡献

欢迎提交 Issue 和 Pull Request！

## 联系方式

- 项目地址: https://github.com/your-repo/file_parse
- 问题反馈: https://github.com/your-repo/file_parse/issues