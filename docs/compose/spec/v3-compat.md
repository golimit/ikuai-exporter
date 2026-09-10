---
feature: v3-compat
status: delivered
updated: 2026-09-11
branch: feature/v3-compat
commits: dd853ad..378f0eb
---

# iKuai 3.x 兼容（版本识别 + DNAT/Session）

## Report

**What was built** — 启动时自动识别 iKuai 3.x/4.x，并通过 `Source` 抽象统一采集。v3 使用 `Data/Result/ErrMsg` 信封与 `TYPE=data,total` 拉 DNAT；v4 保持原有 `collect_conn` 会话。匹配算法支持 `tcp+udp` 与端口区间。v3 无会话 API 时 `session` 模块标记 `collector_status=1` 并告警，DNAT 仍正常导出。

**Verification** — `go build` / `go test ./pkg` 通过。live v3 设备：version=3.7.15，`ikuai_dnat_info` 多条，session status=1，其余模块 status=0。live v4 设备：version=4.0.310，DNAT+session 回归通过。

**Journey log** —
- v3 成功信封为 `ErrMsg=Success`（`Result` 常为 30000，不是布尔成功码）
- v3 DNAT 不支持 `enabled_total` TYPE，计数改为本地统计
- v3 无 `collect_conn`；端口映射 `interface` 为接口名，可能为 `all`，端口可为区间
- 同一 `Source` 接口让 collector 不再绑定 v4 SDK 类型

## [S1] Problem

当前 exporter 写死 `ikuai.NewV4`，只能连 iKuai 4.x。用户有 iKuai **3.7.15** 设备且已配置多条 DNAT，需要：

1. 自动识别 3.x / 4.x
2. 同一镜像/同一代码采集两代设备
3. DNAT 在两边都可用；Session 在 4.x 可用、3.x 优雅降级

## [S2] Design

### 2.1 版本识别

启动时调用 sysstat/homepage 取 `verinfo.version`：

```text
3.7.15 → major=3
4.x.x  → major=4
```

无法识别时默认按 v4 处理（与现网 upstream 行为一致），并打 warn。

### 2.2 API 信封差异（live 已验证）

| | v3 (3.7.15) | v4 |
|--|-------------|-----|
| 成功 | `ErrMsg=="Success"`，`Result` 常为 30000 | `code==0`，`message=="Success"` |
| 数据 | `Data.data[]`，`Data.total` | `results.data[]`，`results.total` |
| DNAT TYPE | 仅 `data,total`（`enabled_total` 会报 unknown TYPE） | `data,total,enabled_total,disabled_total` |
| Session | **无** `collect_conn`（Not found funcname） | `collect_conn` TYPE=all |
| DNAT 连接数 | `monitor_lanip` TYPE=conn,conn_num（按 `lan_addr` 查，`src_port` 匹配 `lan_port`） | `collect_conn` 会话匹配 |
| DNAT.interface | 接口名 `wan1` / `all` | 多为 WAN IP 或 `wan1,wan2,...` |
| DNAT.protocol | 可为 `tcp+udp` | 多为 `tcp`/`udp`/`any` |
| DNAT.port | 可为区间 `32080-32443` | 多为单端口字符串 |

### 2.3 统一数据模型

沿用现有 `DNATRule` / `Session`。额外：

- `protocol`：规范化为小写；`tcp+udp` 视为匹配 tcp 或 udp 会话
- 端口区间：匹配时若 rule 端口含 `-`，则 session 端口落在区间内也算命中
- v3 的 `enabled_total/disabled_total` 由本地统计得出

### 2.4 Client 抽象（最小侵入）

```go
type APIClient interface {
    MajorVersion() int
    ShowDNAT() ([]DNATRule, error)
    ShowSessions() ([]Session, error) // v3 返回 ErrSessionUnsupported
}
```

- `clientV3`：`*ikuai.IKuai` + v3 信封解析
- `clientV4`：现有 `ShowDNAT`/`ShowCollectConn` 逻辑迁入
- Exporter 持有 `APIClient`，Collect 逻辑不变

### 2.5 Session 在 v3 的行为

- `ikuai_exporter_metrics_collector_status{type="session"}` = 1
- 不输出 `ikuai_session_total`（或输出 0 并由 status 标明不可用）
- 日志：`session API not available on iKuai 3.x`
- DNAT 仍输出；`ikuai_dnat_connections` 经 `monitor_lanip` 按内网主机端口统计（不依赖 `collect_conn`）

### 2.6 启动与配置

- 无新 flag；沿用 `--url/--username/--password/--modules`
- 版本自动识别；`ikuai_version` 已有 label 可观察
- 默认 modules 不变（含 dnat,session）

### 2.7 测试

- 单测：v3/v4 信封解析、tcp+udp 协议匹配、端口区间匹配、版本解析
- live：v3 设备出 `ikuai_dnat_info`（≥1 条）；session status=1
- live：v4 设备回归 DNAT+session 仍可用

## [S3] Out of Scope

- Grafana Dashboard
- 推送/发版流程之外的改动
- v3 会话表 API（设备侧不存在）

## Tasks

- [x] T1: 实现版本识别与 APIClient 接口（v3/v4）— acceptance: 单测覆盖信封与版本解析 (covers: S2.1, S2.2, S2.4)
- [x] T2: v3 DNAT 解析 + 协议/端口区间匹配 — acceptance: live v3 出 ikuai_dnat_info；单测通过 (covers: S2.3, S2.5)
- [x] T3: v3 session 优雅降级 — acceptance: status{session}=1，日志明确 (covers: S2.5)
- [x] T4: 接入 exporter 启动路径 — acceptance: 按版本选 client，v4 回归通过 (covers: S2.4, S2.6)
- [x] T5: live 双机验收 + 文档 — acceptance: v3+v4 /metrics 符合预期；README 补充兼容说明 (covers: S2.7)
