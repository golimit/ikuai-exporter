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
| 每条 DNAT 当前连接数 | 是（经 `monitor_lanip` 按内网端口统计） | 是（经 `collect_conn` 会话匹配） |
| DNAT 入站连接明细（入站 IP / Dest IP） | 是（经 `monitor_lanip`，尽力展示） | 是（经 `collect_conn` DNAT 匹配） |
| 当前会话总数 / 全量明细 | 否（优雅降级） | 是 |
| Grafana 共用 Dashboard | 是（[v13 Dashboard](examples/grafana-dashboard.json)） | 是 |

> 3.x 端口映射支持 `tcp+udp`、端口区间（如 `32080-32443`）；`interface` 为接口名（可能是 `all`）。4.x 的 `interface` 多为 WAN IP 或 `wan1,wan2,...`。

## 最近更新

| 变更 | 说明 |
|:---|:---|
| **Grafana v13 Dashboard** | 新增完整 [examples/grafana-dashboard.json](examples/grafana-dashboard.json)（Dynamic Dashboard v2 schema，24 面板），覆盖健康、系统、网络、DNAT、会话 |
| **3.x DNAT 连接数** | 经 `monitor_lanip` 按内网主机 `lan_addr` + `src_port` 匹配 `lan_port` 统计，不再恒为 0 |
| **公网接口面板** | WAN 流量/连接数通过 `ikuai_iface_info.parent_interface!=""` 筛选，v3/v4 通用，不依赖接口 `ikuai_uptime` |
| **健康面板** | 版本（`ikuai_version`）、设备在线（`ikuai_up`）、各模块采集状态（`ikuai_exporter_metrics_collector_status`） |
| **速率单位** | 实时速率面板使用 `KiBs`，与 `*_kbytes_per_second` 指标一致 |

> Dashboard 以 `examples/grafana-dashboard.json` 为准；下文 Grafana 章节为其摘要。

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
- `ikuai_dnat_session_info` 是否在活跃 DNAT 连接时出现入站 IP / Dest IP

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
        --dnat-session-detail         Export per-connection DNAT inbound session detail metrics (default true)
        --dnat-session-detail-limit int Max number of DNAT session detail series (default 500)
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
| dnat-session-detail | `IKUAI_DNAT_SESSION_DETAIL` | 导出 DNAT 入站连接明细 | `true` |
| dnat-session-detail-limit | `IKUAI_DNAT_SESSION_DETAIL_LIMIT` | DNAT 明细最大条数 | `500` |
| session-detail | `IKUAI_SESSION_DETAIL` | 导出全量会话明细指标 | `false` |
| session-detail-limit | `IKUAI_SESSION_DETAIL_LIMIT` | 全量明细最大条数 | `200` |

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
| `ikuai_dnat_session_info` | 端口映射入站连接明细（入站 IP、Dest IP 等；**默认开启**） |
| `ikuai_session_total` | 当前会话总数（仅 4.x） |
| `ikuai_session_info` | 可选全量会话明细（需 `IKUAI_SESSION_DETAIL=true`，高基数，非 DNAT 专用） |

#### DNAT 连接数统计方式

| 版本 | 数据来源 | 匹配规则 |
|:---|:---|:---|
| **3.x** | `monitor_lanip`（按 `lan_addr` 查询内网主机连接） | `src_port` 命中 `lan_port`（支持端口区间），协议兼容 |
| **4.x** | `collect_conn` 全量会话表 | 见下方关联规则 |

> 3.x 统计的是内网主机上该服务端口的连接数（与 iKuai UI「当前连接」语义接近）；若同一端口也被内网直连访问，会一并计入。

#### DNAT 入站连接明细

`ikuai_dnat_session_info` 仅导出归属到 DNAT 规则的连接，适合回答「谁通过端口映射连进来了」：

| Label | 含义 |
|:---|:---|
| `tagname` / `interface` / `wan_port` | 端口映射规则信息 |
| `src_addr` / `src_port` | 入站 IP / 端口（外部客户端） |
| `dst_addr` / `dst_port` | Dest IP / 端口（内网目标） |

| 版本 | 数据来源 | 说明 |
|:---|:---|:---|
| **4.x** | `collect_conn` + DNAT 双规则匹配 | post-DNAT / pre-DNAT 地址自动归一化 |
| **3.x** | `monitor_lanip`（按 `lan_addr` 查内网主机连接） | 入站 IP 为内网主机视角的对端地址；若网关 SNAT，可能显示网关内网 IP 而非真实公网客户端 |

默认开启，受 `dnat-session-detail-limit`（默认 500）截断。可用 `IKUAI_DNAT_SESSION_DETAIL=false` 关闭。

#### DNAT 会话关联规则（4.x）

对每条启用的 DNAT，按下列优先级归属会话（每条会话只计一次）：

1. **转换后**：`dst_addr:dst_port == lan_addr:lan_port`
2. **转换前**：`dst_port` 命中 `wan_port`（支持端口区间），且源地址非内网

协议兼容：规则为 `any` 或与会话协议一致；`tcp+udp` 匹配 tcp/udp。

#### 高基数控制（重要）

- `ikuai_dnat_session_info` 默认开启，基数与 DNAT 活跃连接数成正比（通常远小于全量会话），受 `dnat-session-detail-limit`（默认 500）截断。
- 全量 `ikuai_session_info` 默认关闭；开启后受 `session-detail-limit`（默认 200）截断，且包含大量内网出站连接，**不适合**查看端口映射入站。

```yaml
environment:
    # DNAT 入站明细（默认已开启，一般无需配置）
    IKUAI_DNAT_SESSION_DETAIL: "true"
    IKUAI_DNAT_SESSION_DETAIL_LIMIT: "500"
    # 全量会话明细（非 DNAT 专用，默认关闭）
    IKUAI_SESSION_DETAIL: "false"
    IKUAI_SESSION_DETAIL_LIMIT: "200"
```

## Grafana

**权威配置**：[examples/grafana-dashboard.json](examples/grafana-dashboard.json)（`爱快网关监控 - Enhanced Overview v3+v4 Fixed`）

| 项 | 值 |
|:---|:---|
| 格式 | Grafana **v13+** Dynamic Dashboard（v2 schema，`elements` + `GridLayout`） |
| 面板数 | 25 |
| 要求 | Grafana ≥ 13.0，Prometheus 数据源 |
| 导入 | Dashboard → New → Import → Upload JSON file |

### 面板布局

| 区域 | 面板 | 主要指标 / 查询要点 |
|:---|:---|:---|
| 健康 | iKuai 版本 / 在线状态 / 采集器状态 | `ikuai_version`、`ikuai_up`、`ikuai_exporter_metrics_collector_status` |
| 系统 | CPU / 内存 / Uptime / 温度 / 连接数 / 在线终端 | 顶部 Stat 行 |
| 网络 · 主机 | Host Network IO/s | `ikuai_network_*_kbytes_per_second{id="host"}`，单位 KiBs |
| 网络 · 公网 | 公网接口 Network IO/s、公网接口统计 | `ikuai_iface_info{parent_interface!=""}` 筛选 WAN；不依赖 `ikuai_uptime` |
| 网络 · 全部接口 | Interface Network IO/s | 全部 `iface/*` 接口 |
| 网络 · 终端 | 在线终端表、Device IO/s、流量统计 | `ikuai_device_info` join；终端表含 IP 版本与连接数 |
| DNAT | 规则数 / 启用 / 禁用 / 映射连接 | `ikuai_dnat_*` 系列 |
| DNAT · 明细 | 端口映射规则、连接数、连接明细 | `ikuai_dnat_info`；`ikuai_dnat_connections`（每规则连接数）；`ikuai_dnat_session_info`（入站 IP / Dest IP） |
| 会话 | 当前会话 / 全量会话明细（可选） | `ikuai_session_total`（**仅 4.x**）；全量明细需 `IKUAI_SESSION_DETAIL=true` |

### 变量

- `$instance`：`label_values(ikuai_version, instance)`，多 exporter 部署时切换设备。

### 与上游 Dashboard 差异

上游 [jakeslee/ikuai-exporter](https://github.com/jakeslee/ikuai-exporter) 为 Grafana 9 旧 schema（`panels` + `schemaVersion`），无 DNAT/Session/健康面板，公网接口依赖 `ikuai_uptime > 0`。本 fork Dashboard 已迁移 v13 v2 并针对 3.x/4.x 统一布局。

## 开发

```shell
go test ./...
go build -o ikuai-exporter .
./ikuai-exporter server --url http://10.0.1.253 -u test -p test123
```

相关设计文档见 `docs/compose/spec/`（`dnat-session-metrics.md`、`v3-compat.md`）。

## 致谢

上游项目：[jakeslee/ikuai-exporter](https://github.com/jakeslee/ikuai-exporter) 与 [jakeslee/ikuai](https://github.com/jakeslee/ikuai) SDK。
