# 前端 Docker 部署指南

## 📁 文件说明

- `Dockerfile`: 多阶段构建配置，使用 Node.js 构建 + Nginx 部署
- `nginx.conf`: Nginx 配置文件，监听 3000 端口并代理 API 请求到后端
- `docker-compose.yml`: Docker Compose 配置文件
- `.dockerignore`: Docker 构建时忽略的文件

## 🚀 快速开始

```bash
cd frontend

# 构建并启动
docker-compose up -d --build

# 查看日志
docker-compose logs -f

# 停止服务
docker-compose down
```

## ⚙️ 配置说明

### 端口配置
- **前端服务**：3000 端口
- **后端 API**：nginx 会将 `/api/*` 请求代理到后端

### 修改后端地址

编辑 `nginx.conf` 文件的第 24 行：

```nginx
proxy_pass http://host.docker.internal:8000;
```

根据实际情况修改：

| 场景 | 后端地址配置 |
|------|-------------|
| 后端在宿主机运行 | `http://host.docker.internal:8000` |
| 后端在其他服务器 | `http://192.168.1.100:8000` |
| 后端在同一 Docker 网络 | `http://backend:8000` |

修改后重新构建：
```bash
docker-compose up -d --build
```

## 📝 常用命令

```bash
# 启动服务
docker-compose up -d

# 查看日志
docker-compose logs -f

# 停止服务
docker-compose down

# 重新构建
docker-compose build

# 查看容器状态
docker-compose ps

# 健康检查
curl http://localhost:3000/health
```

## 🔧 故障排查

### 容器无法启动
```bash
docker-compose logs
docker-compose ps
```

### API 请求失败
1. 检查 `nginx.conf` 中的后端地址是否正确
2. 确认后端服务是否正常运行
3. 查看日志：`docker-compose logs -f`

### 前端页面无法访问
1. 确认容器运行：`docker-compose ps`
2. 检查端口：`docker ps`
3. 检查防火墙设置