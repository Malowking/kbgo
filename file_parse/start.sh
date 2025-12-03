#!/bin/bash

set -e
cd "$(dirname "$0")"

# 启动模式: dev 或 production (默认)
MODE="${1:-production}"

# 安装依赖
poetry install --no-interaction

# 停止已运行的服务
pkill -f "uvicorn app.main:app" 2>/dev/null || true

# 启动服务
if [ "$MODE" = "dev" ]; then
    echo "启动开发模式..."
    poetry run uvicorn app.main:app --host 127.0.0.1 --port 8002 --reload
else
    echo "启动生产模式..."
    mkdir -p logs
    nohup poetry run uvicorn app.main:app --host 127.0.0.1 --port 8002 > logs/server.log 2>&1 &
    echo "服务已启动: http://127.0.0.1:8002"
    echo "查看日志: tail -f logs/server.log"
fi