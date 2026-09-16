#!/usr/bin/env bash
# 停止并删除旧版「一机一容器」compose 实例（ikuai-exporter-01/02 → 9401/9402）
set -euo pipefail
cd "$(dirname "$0")/.."

legacy_containers=(
  ikuai-exporter-ikuai-exporter-01-1
  ikuai-exporter-ikuai-exporter-02-1
)

for c in "${legacy_containers[@]}"; do
  if docker ps -a --format '{{.Names}}' | grep -qx "$c"; then
    docker stop "$c"
    docker rm "$c"
    echo "removed $c"
  fi
done

docker compose down --remove-orphans 2>/dev/null || true
echo "done. 后续请在 Prometheus 主机用单进程 ikuai_exporter :9401 + middleware_targets.yml"
