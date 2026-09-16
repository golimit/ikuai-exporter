# iKuai Exporter

![GitHub Release](https://img.shields.io/github/v/release/golimit/ikuai-exporter?include_prereleases)

采集爱快（iKuai）路由数据并导出为 Prometheus 指标。**自动识别 3.x / 4.x**，同一镜像、同一套指标名。镜像由 GitHub Actions 构建并发布至 `ghcr.io/golimit/ikuai-exporter`。

## 特性

| 能力 | 说明 |
|:---|:---|
| 基础监控 | CPU / 内存 / 温度 / 版本 / 运行时间 / WAN·LAN·终端流量 / 连接数 |
| 端口映射 `dnat` | 规则清单、启用/禁用计数、每条规则连接数；可选入站连接明细 |
| 连接会话 `session` | 会话总数（**仅 4.x**）；全量明细可选，默认关闭 |
| 关联分析 | DNAT 规则 × 当前会话 → `ikuai_dnat_connections` |
| 部署 | 多目标 `/probe`（推荐）或 legacy 单台 `/metrics`；见 [docs/deploy.md](docs/deploy.md) |
| Dashboard | [examples/grafana-dashboard.json](examples/grafana-dashboard.json)（Grafana 13+，3.x/4.x 共用） |

```text
iKuai → 探测版本 → v3/v4 Client → 统一 Source → Collector → Prometheus
```

同一次 scrape 内共享会话数据，避免 `dnat` 与 `session` 重复拉取。

### 版本与功能

| 爱快版本 | 系统 / 接口 / LAN / DNAT | Session 总数与明细 |
|:---:|:---:|:---:|
| **3.x** | 支持（DNAT 连接数经 `monitor_lanip`） | 不支持（`session` 模块状态为 `1` 属预期） |
| **4.x** | 支持（DNAT 经 `collect_conn` 匹配） | 支持 |

3.x 端口映射支持 `tcp+udp`、端口区间；`interface` 可能为 `all` 或接口名。4.x 的 `interface` 多为 WAN IP 或 `wan1,wan2,...`。

## 部署

**默认 HTTP 端口：`9401`**（`/metrics` 与 `/probe` 相同）。

### 多目标（推荐）

单进程 + Prometheus `file_sd` 维护多台设备；账号与 targets 见根目录 **`config.yaml`**（从 [examples/config.yaml.example](examples/config.yaml.example) 复制，勿提交密钥）。

- 配置样板与 Compose：见 [golimit/prometheus — `conf/ikuai_exporter`](https://github.com/golimit/prometheus)
- 抓取：`/probe?target=<ikuai_url>&auth_module=default`，`instance` 等标签由 `middleware_targets` 提供

```shell
docker pull ghcr.io/golimit/ikuai-exporter:latest
# 或在 prometheus 项目中：docker compose up -d ikuai_exporter
```

多账号时在 `config.yaml` 的 `auths` 增加模块，并在 targets 的 `labels` 中设置 `auth_module`。

### 单台 legacy

```yaml
services:
  ikuai-exporter:
    image: ghcr.io/golimit/ikuai-exporter:latest
    restart: always
    command: [ "server", "--web.listen-address=:9401" ]
    environment:
      IKUAI_URL: "http://10.0.1.253"
      IKUAI_USERNAME: "test"
      IKUAI_PASSWORD: "test123"
    ports:
      - "9401:9401"
```

### 本地构建

```shell
cp examples/config.yaml.example config.yaml
docker compose up -d --build
```

### 验证

- Legacy：`http://IP:9401/metrics`
- 多目标：`curl -sG 'http://IP:9401/probe' --data-urlencode 'target=http://10.0.1.253' | head`

关注：`ikuai_version`、`ikuai_exporter_metrics_collector_status`（`0` 为成功）、`ikuai_dnat_info` / `ikuai_dnat_connections`。

## 参数说明

建议使用只读账号。

```bash
ikuai-exporter server [flags]
```

常用项：

| 参数 | 环境变量 | 说明 | 默认值 |
|:---|:---|:---|:---|
| config.file | `IKUAI_CONFIG_FILE` | 多目标 YAML | — |
| web.listen-address | `IKUAI_WEB_LISTEN_ADDRESS` | 监听地址 | `:9401` |
| url | `IKUAI_URL` | 爱快地址（legacy） | `http://10.0.1.253` |
| username / password | `IKUAI_USERNAME` / `IKUAI_PASSWORD` | 登录 | `test` / `test123` |
| modules | `IKUAI_MODULES` | 采集模块 | `sysStat,lanDevice,interfaceInfo,dnat,session` |
| insecure-skip | `IKUAI_INSECURE_SKIP` | 跳过 TLS 校验 | `true` |
| timeout | `IKUAI_TIMEOUT` | API 超时（秒） | `2` |
| dnat-session-detail | `IKUAI_DNAT_SESSION_DETAIL` | DNAT 入站连接明细 | `true` |
| dnat-session-detail-limit | `IKUAI_DNAT_SESSION_DETAIL_LIMIT` | 明细条数上限 | `500` |
| session-detail | `IKUAI_SESSION_DETAIL` | 全量会话明细（高基数） | `false` |
| session-detail-limit | `IKUAI_SESSION_DETAIL_LIMIT` | 全量明细上限 | `200` |

完整 flags：`ikuai-exporter server -h`。环境变量中 `-` 写作 `_`（`IKUAI_*`）。

已弃用但仍可用：`IK_URL`、`IK_USER`、`IK_PWD`。

## 采集模块

| 模块 | 说明 | 默认 | 版本 |
|:---|:---|:---:|:---|
| sysStat | 系统状态 | 是 | 3.x / 4.x |
| lanDevice | 内网终端 | 是 | 3.x / 4.x |
| interfaceInfo | 接口流量 | 是 | 3.x / 4.x |
| dnat | 端口映射与连接数 | 是 | 3.x / 4.x |
| session | 当前会话 | 是 | 仅 4.x |

Exporter 只读取已有 DNAT 规则，不会创建或修改映射。

## 指标摘要

**基础（与上游指标名兼容）：** `ikuai_version`、`ikuai_cpu_*`、`ikuai_memory_*`、`ikuai_iface_info`、`ikuai_network_*`、`ikuai_up`、`ikuai_exporter_metrics_collector_status` 等。

**扩展：**

| 指标 | 说明 |
|:---|:---|
| `ikuai_dnat_info` / `ikuai_dnat_*_total` | 端口映射规则与计数 |
| `ikuai_dnat_connections` | 每条启用规则的当前连接数 |
| `ikuai_dnat_session_info` | DNAT 入站明细（默认开，见 `dnat-session-detail-limit`） |
| `ikuai_session_total` | 当前会话总数（4.x） |
| `ikuai_session_info` | 全量会话明细（默认关） |

4.x DNAT 归属：优先 `dst_addr:dst_port == lan_addr:lan_port`，否则按 `wan_port` 与非内网源匹配。3.x 经 `monitor_lanip` 按内网主机端口统计，语义接近 UI「当前连接」。

明细与基数：默认只开 `ikuai_dnat_session_info`；全量 `ikuai_session_info` 含大量内网出站，不适合只看端口映射入站。

## Grafana

导入 [examples/grafana-dashboard.json](examples/grafana-dashboard.json)（Grafana ≥ 13，Prometheus 数据源）。变量 `$instance` 对应 `label_values(ikuai_version, instance)`。公网接口面板使用 `ikuai_iface_info{parent_interface!=""}`，3.x/4.x 通用。

## 开发

```shell
go test ./...
go build -o ikuai-exporter .
./ikuai-exporter server --url http://10.0.1.253 -u test -p test123
```

设计说明：`docs/compose/spec/`（`dnat-session-metrics.md`、`v3-compat.md`）。

## 致谢

基于 [jakeslee/ikuai-exporter](https://github.com/jakeslee/ikuai-exporter) 二次开发，并依赖 [jakeslee/ikuai](https://github.com/jakeslee/ikuai) Go SDK。
