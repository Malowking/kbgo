# Docker 部署指南

本文档说明如何使用 Docker 和 Docker Compose 部署 kbgo 项目。

## 前置要求

- Docker 20.10+
- Docker Compose 2.0+

## 快速开始

### 启动所有服务

项目使用 Docker Compose 一键启动所有依赖服务：

```bash
# 进入 docker 目录
cd docker

# 启动所有服务（PostgreSQL + file-parse + kbgo）
docker-compose up -d

# 查看服务状态
docker-compose ps

# 查看日志
docker-compose logs -f kbgo
```

**包含的服务：**
- **PostgreSQL** - 数据库 + pgvector 向量存储
- **file-parse** - Python 文档解析服务
- **kbgo** - 主应用服务

## 服务架构

Docker Compose 启动以下服务：

| 服务 | 容器名 | 端口 | 说明 |
|------|--------|------|------|
| postgres | kbgo-postgres | 5432 | PostgreSQL 数据库（含 pgvector 扩展）|
| file-parse | kbgo-file-parse | 8002 | Python 文档解析服务 |
| kbgo | kbgo-app | 8000 | 主应用服务 |

**服务间通信：**
- kbgo 通过容器名 `kbgo-postgres` 连接数据库
- kbgo 通过容器名 `kbgo-file-parse` 调用文件解析服务
- 所有服务在同一个 Docker 网络 `kbgo-network` 中

## 配置说明

### 配置文件

项目使用 `config/config_demo.yaml` 作为 Docker 环境的配置文件，已针对容器环境进行优化：

- 数据库地址：`kbgo-postgres:5432`
- 向量数据库：`kbgo-postgres:5432`（使用 pgvector）
- 文件解析服务：`http://kbgo-file-parse:8002`

### 自定义配置

如需修改配置，有两种方式：

**方式 1：修改配置文件（推荐）**

直接编辑 `config/config_demo.yaml`，然后重新构建镜像：

```bash
cd docker
docker-compose build kbgo
docker-compose up -d
```

**方式 2：挂载自定义配置文件**

取消 `docker-compose.yml` 中的注释：
```yaml
volumes:
  - ../config/config.yaml:/app/config/config.yaml:ro
```

然后创建 `config/config.yaml` 文件并配置。

## 常用命令

### 服务管理

```bash
# 启动所有服务
docker-compose up -d

# 停止所有服务
docker-compose down

# 重启服务
docker-compose restart kbgo

# 查看服务状态
docker-compose ps

# 查看所有服务日志
docker-compose logs -f

# 查看特定服务日志
docker-compose logs -f kbgo
docker-compose logs -f postgres
docker-compose logs -f file-parse
```

### 容器操作

```bash
# 进入 kbgo 容器
docker-compose exec kbgo sh

# 进入 PostgreSQL 容器
docker-compose exec postgres psql -U kbgo -d kbgo

# 重新构建镜像
docker-compose build --no-cache kbgo

# 重新构建并启动
docker-compose up -d --build
```

## 数据持久化

Docker Compose 配置了以下数据卷：

- `postgres_data` - PostgreSQL 数据（Docker volume）
- `../logs` - 应用日志（映射到项目根目录）
- `../upload` - 上传文件存储（映射到项目根目录）

## 初始化数据库

首次启动时：
1. PostgreSQL 容器会自动创建 `kbgo` 数据库
2. kbgo 应用启动时会自动：
   - 创建所需的表结构
   - 安装 pgvector 扩展
   - 创建 vectors schema

## 文件存储

项目使用本地文件系统存储上传的文件：
- 文件保存在项目根目录的 `upload/` 目录
- 通过 Docker volume 挂载到容器的 `/app/upload`

## 故障排查

### 容器无法启动

```bash
# 查看详细日志
docker-compose logs kbgo

# 检查容器状态
docker-compose ps

# 查看所有容器日志
docker-compose logs
```

### 数据库连接失败

1. 确认 PostgreSQL 容器已启动并健康：
   ```bash
   docker-compose ps postgres
   docker-compose exec postgres pg_isready -U kbgo
   ```

2. 检查网络连接：
   ```bash
   docker network inspect docker_kbgo-network
   ```

3. 查看数据库日志：
   ```bash
   docker-compose logs postgres
   ```

### 端口冲突

如果端口被占用，修改 `docker-compose.yml` 中的端口映射：

```yaml
ports:
  - "8080:8000"  # 将主机端口改为 8080
```

### 镜像构建失败

如果遇到网络问题，配置 Docker 镜像加速器：

Docker Desktop → Settings → Docker Engine，添加：
```json
{
  "registry-mirrors": [
    "https://dockerproxy.com",
    "https://docker.m.daocloud.io"
  ]
}
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
# 停止并删除所有容器和网络
docker-compose down

# 停止并删除所有容器、网络和数据卷（包括数据库数据）
docker-compose down -v

# 删除构建的镜像
docker rmi docker-kbgo docker-file-parse

# 清理未使用的 Docker 资源
docker system prune -a
```