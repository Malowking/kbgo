#!/bin/bash

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${YELLOW}停止 File Parse Service...${NC}"

# 查找进程
PIDS=$(pgrep -f "uvicorn app.main:app")

if [ -z "$PIDS" ]; then
    echo -e "${YELLOW}没有找到运行中的服务${NC}"
    exit 0
fi

# 停止进程
echo -e "找到进程: ${GREEN}${PIDS}${NC}"
pkill -f "uvicorn app.main:app"

# 等待进程结束
sleep 2

# 检查是否成功停止
REMAINING=$(pgrep -f "uvicorn app.main:app")
if [ -z "$REMAINING" ]; then
    echo -e "${GREEN}✓ 服务已成功停止${NC}"
else
    echo -e "${RED}警告: 部分进程仍在运行，尝试强制停止...${NC}"
    pkill -9 -f "uvicorn app.main:app"
    sleep 1

    FINAL_CHECK=$(pgrep -f "uvicorn app.main:app")
    if [ -z "$FINAL_CHECK" ]; then
        echo -e "${GREEN}✓ 服务已强制停止${NC}"
    else
        echo -e "${RED}✗ 无法停止服务，请手动处理${NC}"
        exit 1
    fi
fi