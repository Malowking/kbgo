# Docker 部署指南

本文档说明如何使用 Docker 和 Docker Compose 部署 kbgo 项目。

## 前置要求

- Docker 20.10+
- Docker Compose 2.0+

## 快速开始

项目支持两种 Docker 部署模式：

### 模式 1：完整启动（推荐用于新环境）

包含所有依赖服务（PostgreSQL、文件解析服务、kbgo 应用）

```bash
# 启动所有服务
docker-compose --profile full up -d

# 查看服务状态
docker-compose --profile full ps

# 查看日志
docker-compose logs -f kbgo
```

**包含的服务：**
- PostgreSQL（数据库 + pgvector 向量存储）
- file-parse（Python 文档解析服务）
- kbgo（主应用）

### 模式 2：默认启动（适合已有本地数据库）

仅启动应用容器和文件解析服务，连接本地已有的 PostgreSQL 数据库。

**前提条件：**
- 本地已启动 PostgreSQL（带 pgvector 扩展）
- 本地数据库已创建 `kbgo` 数据库

**步骤：**

1. （可选）配置环境变量：

```bash
# 复制环境变量示例文件
cp .env.example .env

# 编辑 .env 文件，配置本地数据库连接信息
# 或者挂载本地 config.yaml 文件（见下文）
```

2. 启动服务：

```bash
# 默认启动（file-parse + kbgo）
docker-compose up -d

# 查看服务状态
docker-compose ps

# 查看日志
docker-compose logs -f kbgo
```

**包含的服务：**
- file-parse（Python 文档解析服务）
- kbgo（主应用，连接 host.docker.internal 的 PostgreSQL）

3. 容器会通过 `host.docker.internal` 访问宿主机服务

## 服务架构

Docker Compose 启动以下服务：

| 服务 | 端口 | 说明 | 完整模式 | 默认模式 |
|------|------|------|---------|---------|
| kbgo | 8000 | 主应用服务 | ✓ | ✓ |
| file-parse | 8002 | Python 文档解析服务 | ✓ | ✓ |
| postgres | 5432 | PostgreSQL 数据库（含 pgvector 扩展）| ✓ | ✗ |

## 配置说明

### 配置文件说明

- `config/config_demo.yaml` - 示例配置文件（容器内默认使用）
- `config/config.yaml` - 本地开发配置（需自行创建，被 git 忽略）
- `.env.example` - 环境变量示例文件
- `.env` - 实际环境变量配置（需自行创建，被 git 忽略）

### 配置方式

有三种方式配置数据库连接：

**方式 1：使用环境变量（推荐）**

```bash
# 复制示例文件
cp .env.example .env

# 编辑 .env 文件，配置数据库连接
# 默认模式会连接 host.docker.internal（本地服务）
# 完整模式会自动覆盖为容器服务地址
```

**方式 2：挂载配置文件**

取消 `docker-compose.yml` 中的注释：
```yaml
volumes:
  - ./config/config.yaml:/app/config/config.yaml:ro
```

**方式 3：使用默认配置**

不做任何配置，使用 `config_demo.yaml` 的默认值，通过 `host.docker.internal` 连接本地服务。

### 环境变量说明

GoFrame 支持通过环境变量覆盖配置，格式为：`GF_` + 配置路径（点号替换为下划线，全大写）

```bash
# 数据库配置
GF_DATABASE_DEFAULT_HOST=localhost
GF_DATABASE_DEFAULT_PORT=5432
GF_DATABASE_DEFAULT_USER=kbgo
GF_DATABASE_DEFAULT_PASS=password

# PostgreSQL 向量数据库配置
GF_POSTGRES_HOST=localhost
GF_POSTGRES_PORT=5432
GF_POSTGRES_USER=kbgo
GF_POSTGRES_PASSWORD=password
```

## 常用命令

**完整模式（带数据库）：**

```bash
# 启动所有服务
docker-compose --profile full up -d

# 停止所有服务
docker-compose --profile full down

# 查看服务状态
docker-compose --profile full ps

# 重启服务
docker-compose --profile full restart kbgo

# 查看数据库日志
docker-compose --profile full logs -f postgres
```

**默认模式（连接本地数据库）：**

```bash
# 启动 kbgo 应用
docker-compose up -d

# 停止服务
docker-compose down

# 查看日志
docker-compose logs -f kbgo

# 进入容器
docker-compose exec kbgo sh

# 重新构建镜像
docker-compose build --no-cache kbgo
```

**通用命令：**

## 数据持久化

Docker Compose 配置了以下数据卷：

- `postgres_data` - PostgreSQL 数据（仅完整模式）
- `./logs` - 应用日志（映射到主机目录）
- `./upload` - 上传文件存储（映射到主机目录，使用本地存储）

## 初始化数据库

首次启动时，PostgreSQL 会自动创建数据库。kbgo 应用会在启动时自动创建所需的表结构和 pgvector 扩展。

## 文件存储

项目使用本地文件系统存储上传的文件，文件保存在 `./upload` 目录中，已通过 volumes 挂载到容器。

## 故障排查

### 容器无法启动

```bash
# 查看详细日志
docker-compose logs kbgo

# 检查容器状态
docker-compose ps
```

### 数据库连接失败

**完整模式：**
1. 确认 PostgreSQL 容器已启动：`docker-compose --profile full ps postgres`
2. 检查健康状态：`docker-compose exec postgres pg_isready -U kbgo`
3. 查看网络连接：`docker network inspect kbgo_kbgo-network`

**默认模式：**
1. 确认本地 PostgreSQL 已启动
2. 检查 `.env` 文件中的数据库配置是否正确
3. 测试本地连接：`psql -h localhost -U your_user -d kbgo`

### 端口冲突

如果端口被占用，修改 `docker-compose.yml` 中的端口映射：

```yaml
ports:
  - "8080:8000"  # 将主机端口改为 8080
```

## 生产环境建议

1. **安全性**
   - 修改默认密码
   - 使用 Docker secrets 管理敏感信息
   - 启用 TLS/SSL

2. **性能优化**
   - 根据需要调整容器资源限制
   - 配置日志轮转
   - 使用外部数据库和存储（生产环境）

3. **监控**
   - 添加健康检查
   - 集成监控工具（如 Prometheus）
   - 配置日志聚合

## API 访问

服务启动后，可以通过以下地址访问：

- API 端点：http://localhost:8000/api
- Swagger 文档：http://localhost:8000/swagger

## 卸载

```bash
# 默认模式：停止并删除容器
docker-compose down

# 完整模式：停止并删除所有容器、网络、数据卷
docker-compose --profile full down -v

# 删除镜像
docker rmi kbgo:latest
docker rmi kbgo-file-parse:latest
```