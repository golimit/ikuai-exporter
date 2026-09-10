# iKuai Exporter

![GitHub Release](https://img.shields.io/github/v/release/golimit/ikuai-exporter?include_prereleases)

一个用于采集爱快（iKuai）路由统计数据，并导出为 Prometheus 格式的 Exporter。

本仓库基于 [jakeslee/ikuai-exporter](https://github.com/jakeslee/ikuai-exporter) 二次开发，镜像由 GitHub Actions 自动构建并推送到 GHCR。

## 本版本能力（fork）

| 能力 | 说明 |
|:---|:---|
| 基础监控 | CPU / 内存 / 温度 / 版本 / 运行时间 / WAN·LAN·终端流量 / 连接数 |
| 版本兼容 | **自动识别 iKuai 3.x / 4.x**，同一镜像同一套指标 |
| 端口映射 `dnat` | 规则清单、启用/禁用计数、每条规则当前连接数（3.x / 4.x） |
| 连接会话 `session` | 会话总数；可选明细（**仅 4.x**；3.x 标记 `collector_status=1`） |
| 关联分析 | DNAT 规则 × 当前会话 → `ikuai_dnat_connections` |
| 部署 | 预构建镜像 / 本地 `docker compose` + `.env` |
| CI | `master` 推送自动构建并更新 GHCR `latest` |

### 采集架构

```text
iKuai 设备
   │
   ↓  启动时探测 verinfo.version
3.x ──→ v3 Client ──┐
4.x ──→ v4 Client ──┴──→ 统一 Source ──→ Collector ──→ Prometheus
```

- Collector 只依赖统一 `Source` 接口，不直接绑定某一版 SDK。
- 会话列表在同一次 scrape 内共享，避免 `dnat` + `session` 双重拉取。

## 版本兼容

| 本仓库 | 爱快版本 | 描述 |
|:---|:---|:---|
| `>= v0.4.0`（fork） | **3.x / 4.0+** | 自动识别版本；DNAT 两端可用；Session 仅 4.x |
| 上游 `>= v0.3.0` | 4.0+ | 仅 4.0，不保证兼容 3.0 |
| 上游 `v0.2.x` | 3.x | 仅 3.0 |

### 功能矩阵

| 功能 | iKuai 3.x | iKuai 4.x |
|:---|:---:|:---:|
| 系统状态 / CPU / 内存 | 是 | 是 |
| 接口流量 | 是 | 是 |
| LAN 设备 | 是 | 是 |
| 连接数（host/iface/device） | 是 | 是 |
| DNAT 端口映射清单 | 是 | 是 |
| 每条 DNAT 当前连接数 | 是（无会话数据时为 0） | 是 |
| 当前会话总数 / 明细 | 否（优雅降级） | 是 |
| Grafana 共用 Dashboard | 是 | 是 |

> 3.x 端口映射支持 `tcp+udp`、端口区间（如 `32080-32443`）；`interface` 为接口名（可能是 `all`）。4.x 的 `interface` 多为 WAN IP 或 `wan1,wan2,...`。

## 部署

### 1. 预构建镜像

```shell
docker pull ghcr.io/golimit/ikuai-exporter:latest
```

```yaml
services:
    ikuai-exporter:
        image: ghcr.io/golimit/ikuai-exporter:latest
        restart: always
        environment:
            IKUAI_URL: "http://10.0.1.253"
            IKUAI_USERNAME: "test"
            IKUAI_PASSWORD: "test123"
        ports:
            - "9090:9090"
```

### 2. 本地源码构建

项目根目录提供 `docker-compose.yaml`，可一次部署多个实例（示例：`ikuai-exporter-01` → `:9401`，`ikuai-exporter-02` → `:9402`）：

```shell
cp .env.example .env
cp .env.example.instance-01 .env.instance-01
cp .env.example.instance-02 .env.instance-02
# 编辑 .env 填入共享凭据，按需修改各实例 URL
docker compose up -d --build
```

凭据放在 `.env`，各实例 URL 放在 `.env.instance-01` / `.env.instance-02`（均已加入 `.gitignore`）。可参考 `.env.example*`。

修改代码后需加 `--build`，否则会复用旧镜像。

### 3. 多台 iKuai

同一镜像部署多个实例，各自指向一台设备，Grafana 共用 Dashboard：

```yaml
services:
    ikuai-exporter-01:
        image: ghcr.io/golimit/ikuai-exporter:latest
        environment:
            IKUAI_URL: "http://10.0.1.253"       # 4.x 示例
            IKUAI_USERNAME: "test"
            IKUAI_PASSWORD: "test123"
        ports: ["9090:9090"]
    ikuai-exporter-02:
        image: ghcr.io/golimit/ikuai-exporter:latest
        environment:
            IKUAI_URL: "http://10.0.2.253"       # 3.x 示例
            IKUAI_USERNAME: "test"
            IKUAI_PASSWORD: "test123"
        ports: ["9091:9090"]
```

### 验证

访问 `http://IP:9090/metrics`。检查：

- `ikuai_version{version="..."}` 版本是否正确
- `ikuai_exporter_metrics_collector_status` 各模块是否为 `0`（3.x 的 `session` 为 `1` 属预期）
- `ikuai_dnat_info` 是否出现已有端口映射

## 参数说明

登录帐号建议使用只读用户。

```bash
Usage:
  ikuai-exporter server [flags]

Flags:
    -h, --help                        help for server
        --insecure-skip               Skip iKuai certificate verification (default true)
    -l, --level string                Log level (default "info")
        --modules strings             The modules to be collected. (default [sysStat,lanDevice,interfaceInfo,dnat,session])
    -p, --password string             The password for the user on iKuai (default "test123")
        --session-detail              Export per-session detail metrics (high cardinality) (default false)
        --session-detail-limit int    Max number of session detail series (default 200)
        --timeout int                 The timeout (seconds) for a request to iKuai API. (default 2)
        --url string                  iKuai URL (default "http://10.0.1.253")
    -u, --username string             iKuai username (default "test")
```

| 参数 | 环境变量 | 说明 | 默认值 |
|:---|:---|:---|:---|
| url | `IKUAI_URL` | 爱快地址 | `http://10.0.1.253` |
| username | `IKUAI_USERNAME` | 登录用户名 | `test` |
| password | `IKUAI_PASSWORD` | 登录密码 | `test123` |
| modules | `IKUAI_MODULES` | 采集模块（逗号分隔） | `sysStat,lanDevice,interfaceInfo,dnat,session` |
| insecure-skip | `IKUAI_INSECURE_SKIP` | 跳过 HTTPS 证书验证 | `true` |
| timeout | `IKUAI_TIMEOUT` | 请求超时（秒） | `2` |
| session-detail | `IKUAI_SESSION_DETAIL` | 导出会话明细指标 | `false` |
| session-detail-limit | `IKUAI_SESSION_DETAIL_LIMIT` | 明细最大条数 | `200` |

环境变量格式为 `IKUAI_XXX`（flag 中的 `-` 对应 `_`）。

旧变量名仍可用，**将在以后版本中弃用**：

| 变量名 | 说明 |
|:---|:---|
| `IK_URL` | 爱快地址 |
| `IK_USER` | 登录用户 |
| `IK_PWD` | 登录密码 |

## 采集模块

| 模块 | 说明 | 默认 | 版本 |
|:---|:---|:---:|:---|
| sysStat | 系统状态（CPU/内存/版本/运行时间等） | 是 | 3.x / 4.x |
| lanDevice | 内网终端及流量 | 是 | 3.x / 4.x |
| interfaceInfo | 接口流量与在线状态 | 是 | 3.x / 4.x |
| dnat | 端口映射规则与每条规则连接数 | 是 | 3.x / 4.x |
| session | 当前连接会话总数（明细可选） | 是 | **仅 4.x** |

Exporter **只读取**爱快上已有的端口映射规则，不会创建或启用 DNAT。

关闭某个模块：

```yaml
environment:
    IKUAI_MODULES: "sysStat,lanDevice,interfaceInfo"
```

## 指标说明

### 基础（沿用上游命名）

| 指标 | 说明 |
|:---|:---|
| `ikuai_version` | 设备版本信息 |
| `ikuai_cpu_usage_ratio` / `ikuai_cpu_temperature` | CPU |
| `ikuai_memory_*_bytes` | 内存 |
| `ikuai_device_info` / `ikuai_device_count` | 内网终端 |
| `ikuai_iface_info` | 接口信息 |
| `ikuai_up` / `ikuai_uptime` | 在线状态 / 运行时间 |
| `ikuai_network_send_bytes` / `recv_bytes` | 流量累计 |
| `ikuai_network_send_kbytes_per_second` / `recv_...` | 实时速率 |
| `ikuai_network_conn_count` | 连接数（host / iface / device） |
| `ikuai_exporter_metrics_collector_status` | 各模块采集状态（`0` 成功，`1` 失败） |

### 端口映射 / 会话（本 fork 新增）

| 指标 | 说明 |
|:---|:---|
| `ikuai_dnat_info` | 端口映射规则清单（含 enabled、协议、WAN/内网端口等） |
| `ikuai_dnat_total` / `ikuai_dnat_enabled_total` / `ikuai_dnat_disabled_total` | 规则计数 |
| `ikuai_dnat_connections` | 每条**启用**规则当前匹配连接数 |
| `ikuai_session_total` | 当前会话总数（仅 4.x） |
| `ikuai_session_info` | 可选会话明细（需 `IKUAI_SESSION_DETAIL=true`，高基数） |

#### DNAT 会话关联规则

对每条启用的 DNAT，按下列优先级归属会话（每条会话只计一次）：

1. **转换后**：`dst_addr:dst_port == lan_addr:lan_port`
2. **转换前**：`dst_port` 命中 `wan_port`（支持端口区间），且源地址非内网

协议兼容：规则为 `any` 或与会话协议一致；`tcp+udp` 匹配 tcp/udp。

#### 高基数控制（重要）

- 默认**不把公网 IP / 源端口**写入 Prometheus label。
- 明细 `ikuai_session_info` 默认关闭；开启后受 `session-detail-limit`（默认 200）截断。

```yaml
environment:
    IKUAI_SESSION_DETAIL: "true"
    IKUAI_SESSION_DETAIL_LIMIT: "200"
```

## Grafana

- 上游演示 Dashboard：[examples/grafana-dashboard.json](https://github.com/jakeslee/ikuai-exporter/raw/refs/heads/master/examples/grafana-dashboard.json)
- 基础区（CPU/内存/流量/连接数）可直接复用。
- 端口映射区建议查询：
  - 规则表：`ikuai_dnat_info`
  - 每条规则连接数：`ikuai_dnat_connections`
  - 会话总数：`ikuai_session_total`
- 本 fork 的 DNAT/Session 面板 JSON **尚未合入** 示例 Dashboard，需自行添加或使用 `examples/grafana-dashboard.json` 作底板扩展。

## 开发

```shell
go test ./...
go build -o ikuai-exporter .
./ikuai-exporter server --url http://10.0.1.253 -u test -p test123
```

相关设计文档见 `docs/compose/spec/`（`dnat-session-metrics.md`、`v3-compat.md`）。

## 致谢

上游项目：[jakeslee/ikuai-exporter](https://github.com/jakeslee/ikuai-exporter) 与 [jakeslee/ikuai](https://github.com/jakeslee/ikuai) SDK。
