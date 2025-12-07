#!/bin/bash

set -e
cd "$(dirname "$0")"

# 停止已运行的服务
pkill -f "uvicorn app.main:app" 2>/dev/null || true
mkdir -p logs
nohup poetry run uvicorn app.main:app --host 127.0.0.1 --port 8002 > logs/file_parse.log 2>&1 &
echo "服务已启动: http://127.0.0.1:8002"
echo "查看日志: tail -f logs/file_parse.log"