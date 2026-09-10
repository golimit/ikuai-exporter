---
feature: dnat-session-metrics
status: delivered
updated: 2026-09-11
branch: feature/dnat-session-metrics
commits: 967ea61..55532ec
---

# DNAT + Session Metrics

## Report

**What was built** — 在现有 iKuai v4 exporter 上新增 `dnat` 与 `session` 两个采集模块。`dnat` 通过 `Action/call` 调用 `func_name=dnat` 导出端口映射清单与计数；`session` 调用 `collect_conn` 导出会话总数。Exporter 在同一次 scrape 内共享一次连接列表，按 post-DNAT（内网目标）与 pre-DNAT（公网源 + WAN 端口）双规则把会话归属到启用的 DNAT 规则，输出 `ikuai_dnat_connections`。公网 IP/源端口默认不进 label；`--session-detail` 可选开启明细并受 `--session-detail-limit`（默认 200）截断。不 fork SDK、不改既有指标名。

**Verification** — `go build ./...` / `go test ./...` / `go vet ./...` 均通过。对 live iKuai 4.x 设备抓取 `/metrics`：出现 `ikuai_dnat_info`、`ikuai_dnat_total/enabled_total`、`ikuai_dnat_connections`、`ikuai_session_total`，`collector_status{dnat,session}` 均为 0。首轮 code review 发现 RFC1918 `172.16/12` 字符串比较与 env 绑定问题，已修复并通过 focused re-review（APPROVE）。

**Journey log** —
- SDK 无 DNAT/Session show API；live 探测确认 `func_name=dnat` 与 `collect_conn`（TYPE=all）可用。
- `collect_conn` 当前样本多为出站；入站 DNAT 归属依赖双匹配规则，尚无入站流量实测。
- `dnat.interface` 字段在 v4 上可能是 WAN IP 或 `wan1,wan2,wan3` 字符串，按 label 原样导出。
- `git worktree add` 被沙箱拦截，改用仓库外独立 clone 隔离开发。
- Viper `AutomaticEnv` 对带 dash 的 flag 需要 `SetEnvKeyReplacer` 与显式 `BindEnv`。

## [S1] Problem

现有 `ikuai-exporter` 只采集 `sysStat / lanDevice / interfaceInfo`。用户需要在 Grafana 中回答：

1. 每个端口映射（DNAT）现在有多少连接？
2. 现在是谁在连接哪些映射端口？

上游 SDK 未封装 DNAT 列表与连接会话列表；v4 设备上这些能力需 exporter 自行调用 `Action/call`。

## [S2] Design

### 2.1 范围决策

- **仅支持 iKuai 4.x**（与当前 upstream `>= v0.3.0` 一致），不做 v3 双栈。
- 不做 `client/` 大重构；沿用现有 `pkg/exporter.go` 的 modules 机制扩展。
- 不修改既有 metric 名称与现有三个 collector 的行为。

### 2.2 API 契约（已在 live v4 上验证）

| 用途 | action | func_name | param | 结果路径 |
| ---- | ------ | --------- | ----- | -------- |
| 端口映射列表 | `show` | `dnat` | `TYPE=data,total,enabled_total,disabled_total`, `limit=0,10000` | `results.data[]`, `results.total`, `results.enabled_total`, `results.disabled_total` |
| 当前连接列表 | `show` | `collect_conn` | `TYPE=all` | `results.conn[]` |

**DNAT 字段**（API 原样）：

```text
id, enabled("yes"/"no"), tagname, comment,
interface   # 外网地址（WAN IP 字符串），不是 wan1/wan2 接口名
lan_addr, lan_port, wan_port,
protocol    # "tcp" / "udp" / "any"
src_addr    # object，可选源地址限制
```

**Session 字段**（`collect_conn`）：

```text
protocol, status, starttime,
src_addr, src_port,     # 连接一侧（出站时为内网终端；入站 DNAT 时为公网客户端）
dst_addr, dst_port,     # 连接另一侧（出站时为远端；入站 DNAT 转换后为目标内网）
terminal_addr, terminal_port,
src_interface, dst_interface,
app_name, domain, total_up, total_down
```

实现中通过 `base.IKuaiBase.Run(action, result)` 直接调用，不 fork SDK。

### 2.3 内部数据模型

```go
type DNATRule struct {
    ID        int64
    Enabled   bool
    Tagname   string
    Comment   string
    Interface string // WAN IP
    LANAddr   string
    LANPort   string
    WANPort   string
    Protocol  string // tcp|udp|any
}

type Session struct {
    Protocol      string
    SrcAddr       string
    SrcPort       string
    DSTAddr       string
    DSTPort       string
    TerminalAddr  string
    TerminalPort  string
    SrcInterface  string
    DSTInterface  string
    AppName       string
    Domain        string
    Starttime     int64
    TotalUp       float64
    TotalDown     float64
}
```

### 2.4 DNAT ↔ Session 匹配规则

对每条 **enabled** DNAT 规则，统计匹配会话数：

按优先级尝试（第一条命中即计数，避免重复计）：

1. **DNAT 后（内网目标）**：`dst_addr == lan_addr && dst_port == lan_port`，且 protocol 兼容（规则 `any` 或与会话 protocol 相等，忽略大小写）
2. **DNAT 前（外网入口）**：`dst_port == wan_port` 且 `src_addr` 非内网（非 RFC1918 / 非本机接口 IP），且 protocol 兼容

同时导出：

- `ikuai_dnat_connections{tagname,interface,protocol,wan_port,lan_addr,lan_port}` = 匹配数
- 对未匹配到任何规则的会话不单独打点

### 2.5 Prometheus Metrics

| Metric | Type | Labels | 含义 |
| ------ | ---- | ------ | ---- |
| `ikuai_dnat_info` | Gauge=1 | `id,tagname,comment,interface,protocol,wan_port,lan_addr,lan_port,enabled` | 规则清单 |
| `ikuai_dnat_total` | Gauge | （无） | 规则总数 |
| `ikuai_dnat_enabled_total` | Gauge | （无） | 启用规则数 |
| `ikuai_dnat_disabled_total` | Gauge | （无） | 停用规则数 |
| `ikuai_dnat_connections` | Gauge | `tagname,interface,protocol,wan_port,lan_addr,lan_port` | 每条启用规则当前匹配连接数 |
| `ikuai_session_total` | Gauge | （无） | `collect_conn` 返回的会话总数 |
| `ikuai_session_info` | Gauge=1 | `protocol,src_addr,src_port,dst_addr,dst_port,terminal_addr,app_name` | 可选会话明细 |
| `ikuai_exporter_metrics_collector_status{type="dnat"}` | Gauge | 复用现有 | 采集是否成功 |
| `ikuai_exporter_metrics_collector_status{type="session"}` | Gauge | 复用现有 | 采集是否成功 |

**高基数控制**（遵守 Plan.md §10）：

- 默认 **不把** 公网 IP / 源端口写入 Prometheus label。
- 会话明细通过 `--session-detail` 开启；开启后最多导出 `--session-detail-limit`（默认 200）条 `ikuai_session_info`，超出丢弃并记日志。
- 默认 modules：`sysStat,lanDevice,interfaceInfo,dnat,session`（`session` 只做聚合计数，不做明细）。

### 2.6 模块与配置

扩展 `supported_modules`：

```text
sysStat, lanDevice, interfaceInfo, dnat, session
```

- `dnat`：拉 DNAT 规则 + 在同一次 Collect 内拉 `collect_conn` 做关联（避免双次全量连接拉取）
- `session`：单独拉 `collect_conn`，导出 `ikuai_session_total`；若配置了明细开关则导出明细
- 两个模块都启用时，session 列表在 Collect 内共享一次请求（内存缓存 per-scrape）

新增 flags / env（沿用 `IKUAI_` 前缀）：

| Flag | Env | Default | 说明 |
| ---- | --- | ------- | ---- |
| `--session-detail` | `IKUAI_SESSION_DETAIL` | `false` | 是否导出会话明细 metrics |
| `--session-detail-limit` | `IKUAI_SESSION_DETAIL_LIMIT` | `200` | 明细最大条数 |

### 2.7 错误行为

- DNAT 或 session API 失败：对应 `collector_status` 置 1，记 error 日志，不影响其他 module。
- `collect_conn` 返回空：`ikuai_session_total=0`，`ikuai_dnat_connections` 各规则为 0（启用规则仍输出）。
- 字段解析失败（端口非数字等）：该条会话/规则跳过并 debug 日志，不 panic。

### 2.8 测试边界

- 单元测试：DNAT/session JSON 解析、匹配算法、enabled 判断、协议兼容、明细截断。
- 集成验收：对 live iKuai 4.x 设备抓 `/metrics`，至少看到 `ikuai_dnat_info` 与已有 DNAT 规则。

## [S3] Out of Scope

- iKuai 3.x 双栈与版本自动识别
- `client/common|v3|v4` 包结构重构
- Grafana Dashboard JSON 完整改版（可另开任务）
- UPnP leases（`upnpd_leases`）纳入
- 修改 upstream 既有 metric 命名或采集逻辑
- 推送 GitHub / 发布 Docker 镜像

## Tasks

- [x] T1: 实现 API 客户端扩展（dnat/collect_conn 的 Action 与结果类型）— acceptance: 包内可编译，单测覆盖解析 (covers: S2.2, S2.3)
- [x] T2: 实现 DNAT Collector 与 metrics — acceptance: `/metrics` 含 `ikuai_dnat_*`，status 可观察 (covers: S2.4, S2.5; depends: T1)
- [x] T3: 实现 Session Collector 与 `ikuai_session_total` / 可选明细 — acceptance: modules 含 session 时有 total；detail flag 生效 (covers: S2.5, S2.6, S2.7; depends: T1)
- [x] T4: 实现 DNAT-Session 关联 `ikuai_dnat_connections` — acceptance: 启用规则均输出连接数；匹配单测通过 (covers: S2.4; depends: T2, T3)
- [x] T5: 接入 modules/flags/README — acceptance: 默认 modules 含 dnat,session；flag/env 文档更新 (covers: S2.6)
- [x] T6: 单测 + live 验收 + go build — acceptance: `go test ./...` 通过；live `/metrics` 含新指标 (covers: S2.8)
