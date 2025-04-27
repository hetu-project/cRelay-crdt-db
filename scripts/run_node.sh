#!/bin/bash

set -e

# 设置默认数据目录
DATA_DIR="./data/node1"
IPFS_DIR="$DATA_DIR/ipfs"
ORBITDB_DIR="$DATA_DIR/orbitdb"
SETTINGS_DIR="$DATA_DIR/settings"
LISTEN_ADDR="/ip4/0.0.0.0/tcp/4001"
IPFS_API_PORT=5001

# 可选：可通过参数自定义数据目录
if [ -n "$1" ]; then
  DATA_DIR="$1"
  IPFS_DIR="$DATA_DIR/ipfs"
  ORBITDB_DIR="$DATA_DIR/orbitdb"
  SETTINGS_DIR="$DATA_DIR/settings"
fi

# 检查 IPFS 锁文件并清理
LOCK_FILE="$IPFS_DIR/repo.lock"
API_LOCK_FILE="$IPFS_DIR/api"

if [ -f "$LOCK_FILE" ]; then
  echo "==> 检测到锁文件，清理中..."
  rm -f "$LOCK_FILE"
  echo "==> 已删除 $LOCK_FILE"
fi

if [ -f "$API_LOCK_FILE" ]; then
  echo "==> 检测到 API 锁文件，清理中..."
  rm -f "$API_LOCK_FILE"
  echo "==> 已删除 $API_LOCK_FILE"
fi

# 1. 初始化 IPFS
if [ ! -d "$IPFS_DIR" ] || [ ! -f "$IPFS_DIR/config" ]; then
  echo "==> 初始化 IPFS 仓库 ($IPFS_DIR)..."
  mkdir -p "$IPFS_DIR"
  
  # 使用环境变量设置 IPFS_PATH 来指定仓库路径
  export IPFS_PATH="$IPFS_DIR"
  ipfs init --profile server
  
  # 显式修改 config（开 pubsub 模式，增强互联）
  ipfs config --json Pubsub.Enabled true
  ipfs config --json Pubsub.Router '"gossipsub"'
fi

# 2. 启动 IPFS daemon（后台运行，pubsub enabled）
echo "==> 启动 IPFS daemon..."
IPFS_PATH="$IPFS_DIR" ipfs daemon --enable-pubsub-experiment --api /ip4/127.0.0.1/tcp/$IPFS_API_PORT > ipfs_node1.log 2>&1 &
IPFS_PID=$!

# 等待 API 启动
echo "==> 等待 IPFS API 启动..."
for i in {1..15}
do
  if curl -s http://127.0.0.1:$IPFS_API_PORT/api/v0/version > /dev/null; then
    echo "IPFS API 已启动"
    break
  fi
  sleep 1
done

# 3. 启动 OrbitDB 节点
echo "==> 启动 OrbitDB 节点..."
./orbitdb -data "$DATA_DIR" -listen "$LISTEN_ADDR" -ipfs "127.0.0.1:$IPFS_API_PORT"
# ./orbitdb -data ./data/node1 -listen /ip4/0.0.0.0/tcp/4001 -ipfs "127.0.0.1:5001"
# 4. 退出时关闭 IPFS
echo "==> 正在退出，关闭 IPFS daemon..."
kill $IPFS_PID
