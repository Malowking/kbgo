# 多阶段构建 - 构建阶段
FROM golang:1.25-alpine AS builder

# 设置工作目录
WORKDIR /build

# 安装必要的构建工具
RUN apk add --no-cache git gcc musl-dev

# 复制 go mod 文件
COPY go.mod go.sum ./

# 下载依赖
RUN go mod download

# 复制源代码
COPY . .

# 构建应用
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /build/kbgo .

# 运行阶段
FROM alpine:latest

# 安装 ca-certificates 用于 HTTPS 请求
RUN apk --no-cache add ca-certificates tzdata

# 设置时区
ENV TZ=Asia/Shanghai

# 创建非 root 用户
RUN addgroup -g 1000 app && \
    adduser -D -u 1000 -G app app

# 创建必要的目录
RUN mkdir -p /app/config /app/logs && \
    chown -R app:app /app

WORKDIR /app

# 从构建阶段复制二进制文件
COPY --from=builder /build/kbgo /app/kbgo

# 复制配置文件作为默认配置
COPY --chown=app:app config/config_demo.yaml /app/config/config.yaml

# 切换到非 root 用户
USER app

# 暴露端口
EXPOSE 8000

# 启动应用
CMD ["/app/kbgo"]