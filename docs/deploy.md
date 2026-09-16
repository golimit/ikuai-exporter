# 部署说明

## 默认端口

| 用途 | 端口 | 路径 |
|------|------|------|
| HTTP 服务（legacy 单目标） | **9401** | `/metrics` |
| HTTP 服务（多目标 /probe） | **9401** | `/probe`、`/healthz` |

与 Prometheus 栈中 `mysqld_exporter`（9104）、`redis_exporter`（9121）同级约定：**一台 Prometheus 主机只跑一个 `ikuai_exporter`，固定监听 `:9401`**。新增 iKuai 时只改 `config.yaml` 里的 `middleware_targets`，并同步到 Prometheus。

程序默认值：`--web.listen-address=:9401`（环境变量 `IKUAI_WEB_LISTEN_ADDRESS`）。

## 本仓库配置（`config.yaml`）

- 路径：项目根目录 **`config.yaml`**（**不入 Git**，见 `.gitignore`）
- 样板：`cp examples/config.yaml.example config.yaml`
- 内容：
  - 上半：`auths`、采集选项等 → 供 exporter `--config.file` 使用
  - `middleware_targets`：供复制到 `conf/prometheus/middleware_targets.yml`（程序不读取该段）

## 本仓库快速启动

```shell
cp examples/config.yaml.example config.yaml
# 编辑 auths、middleware_targets
docker compose up -d --build
```

### 验证 probe

```shell
curl -sG 'http://127.0.0.1:9401/probe' \
  --data-urlencode 'target=http://192.168.x.x' \
  --data-urlencode 'auth_module=default' | head
```

## Prometheus 多目标（推荐生产）

部署在 **Prometheus 项目**时，可将 `config.yaml` 中的 `auths` 同步到 `conf/ikuai_exporter/config.yml`，`middleware_targets` 同步到 `conf/prometheus/middleware_targets.yml`。

1. `prometheus.yml` 中 `job_name: ikuai` 的 relabel `replacement` 指向 `<prometheus_host>:9401`
2. `docker compose up -d ikuai_exporter`（见 `~/project/prometheus/docker-compose.yaml.env`）

详细步骤：本机 `~/project/prometheus/conf/ikuai_exporter/README.md`。

新增一台 iKuai：在 `config.yaml` 的 `middleware_targets` 追加条目，同步到 Prometheus 后 `/-/reload`。

## Legacy：单台 /metrics

仍可通过环境变量单目标启动（无需 `config.yaml`）：

```shell
IKUAI_URL=http://10.0.1.253 IKUAI_USERNAME=u IKUAI_PASSWORD=p \
  ./ikuai-exporter server --web.listen-address=:9401
```

## 已废弃

- 一机一容器（9401/9402 多 compose service）
- `.env` / `.env.instance-*` 按实例拆分 URL

清理旧容器：`./scripts/teardown-legacy-compose.sh`
