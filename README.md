# iKuai Exporter
![GitHub Release](https://img.shields.io/github/v/release/golimit/ikuai-exporter?include_prereleases)

一个用于采集爱快路由统计数据，并导出为 Prometheus 格式的 Exporter。

本仓库基于 [jakeslee/ikuai-exporter](https://github.com/jakeslee/ikuai-exporter) 二次开发，镜像由 GitHub Actions 自动构建并推送到 GHCR。

### 版本

|     版本     | 爱快版本 |              描述              |
|:----------:|:----:|:----------------------------:|
| \>= v0.4.0 | 3.x / 4.0+ | 自动识别版本；DNAT 端口映射监控（两端）；会话总数/明细与 DNAT 连接数关联（Session 仅 4.x） |
| \>= v0.3.0 | 4.0+ | 支持 iKuai 4.0 版本，不保证兼容 3.0 版本 |
|   v0.2.x   | 3.x  |       支持 iKuai 3.0 版本        |

#### 新增功能

- `dnat` 模块：端口映射规则清单、启用/禁用计数、每条规则当前连接数（`ikuai_dnat_connections`）；兼容 iKuai 3.x / 4.x
- `session` 模块：当前会话总数（`ikuai_session_total`），可选会话明细（`ikuai_session_info`）；**仅 4.x**，3.x 会标记 `collector_status=1`
- 启动时自动识别 iKuai 3.x / 4.x API
- 默认可采集模块扩展为 `sysStat,lanDevice,interfaceInfo,dnat,session`
- 支持 `.env` + `docker compose` 本地构建部署
- `master` 分支推送自动构建并更新 `latest` 镜像

### 部署

拉取预构建镜像：

```shell
docker pull ghcr.io/golimit/ikuai-exporter:latest
```

使用 docker-compose 部署（预构建镜像）：

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

使用本地源码构建（项目根目录已提供 `docker-compose.yaml`）：

```shell
cp .env.example .env
# 编辑 .env，填入爱快地址与登录凭据
docker compose up -d --build
```

`docker-compose.yaml` 通过 `env_file: .env` 加载配置，`.env` 已加入 `.gitignore`，请勿提交真实凭据。可参考 `.env.example`：

```dotenv
IKUAI_URL=http://10.0.1.253
IKUAI_USERNAME=test
IKUAI_PASSWORD=test123
IKUAI_SESSION_DETAIL=false
IKUAI_SESSION_DETAIL_LIMIT=200
```

修改本地代码后需加 `--build` 重新构建镜像，否则会复用旧镜像。

部署完成后，访问 `http://IP:9090/metrics`（或自定义映射端口，如 `9401`）验证运行情况。可检查 `ikuai_exporter_metrics_collector_status` 是否为 `0`（成功）。

接下来将 exporter 的采集地址 IP 配置到 Prometheus 的 `scrape_configs` 中就可开始使用。

详细配置和 Grafana 配置示例可以参考使用[样例](https://blog.imoe.tech/2022/12/25/48-use-ikuai-exporter-to-gather-metrics/)，最新的演示 Dashboard 在[这里](https://github.com/jakeslee/ikuai-exporter/raw/refs/heads/master/examples/grafana-dashboard.json)。

### 参数说明

登录的帐号密码建议创建一个只读用户使用。

```bash
Run metrics endpoint

Usage:
i kuai-exporter server [flags]

Flags:
    -h, --help                        help for server
        --insecure-skip               Skip iKuai certificate verification (default true)
    -l, --level string                Log level (default "info")
        --modules strings             The modules to be collected. (default [sysStat,lanDevice,interfaceInfo,dnat,session])
    -p, --password string             The password for the user on iKuai (default "test123")
        --session-detail              Export per-session detail metrics (high cardinality) (default false)
        --session-detail-limit int    Max number of session detail series (default 200)
        --timeout int                 The timeout (seconds) for a request to iKuai API.  (default 2)
        --url string                  iKuai URL (default "http://10.0.1.253")
    -u, --username string             iKuai username (default "test")

```

| 变量名 | 环境变量 | 说明 | 默认值 |
|:------|:--------|:-----|:-----|
| url | `IKUAI_URL` | 爱快地址 | `http://10.0.1.253` |
| username | `IKUAI_USERNAME` | 登录用户名 | `test` |
| password | `IKUAI_PASSWORD` | 登录密码 | `test123` |
| modules | `IKUAI_MODULES` | 采集模块（逗号分隔） | `sysStat,lanDevice,interfaceInfo,dnat,session` |
| insecure-skip | `IKUAI_INSECURE_SKIP` | 跳过 HTTPS 证书验证 | `true` |
| timeout | `IKUAI_TIMEOUT` | 请求超时时间（秒） | `2` |
| session-detail | `IKUAI_SESSION_DETAIL` | 是否导出会话明细指标 | `false` |
| session-detail-limit | `IKUAI_SESSION_DETAIL_LIMIT` | 会话明细最大条数 | `200` |

### 采集模块

| 模块 | 说明 | 是否默认开启 |
|:---|:---|:---|
| sysStat | 系统状态（CPU/内存/版本等） | 是 |
| lanDevice | 内网终端 | 是 |
| interfaceInfo | 接口流量 | 是 |
| dnat | 端口映射规则与每条规则当前连接数（3.x / 4.x） | 是 |
| session | 当前连接会话总数（仅 4.x；3.x 输出 collector_status=1） | 是（仅总数；明细需另开） |

`dnat` 与 `session` 已包含在默认 `modules` 中，**无需额外开启**。Exporter 只读取爱快上已有的端口映射规则，不会在路由器上创建或启用 DNAT。

如需关闭某个模块，可通过 `IKUAI_MODULES` 指定子集，例如：

```yaml
environment:
    IKUAI_MODULES: "sysStat,lanDevice,interfaceInfo"
```

### 会话明细（可选）

默认导出 `ikuai_session_total`（会话总数）和 `ikuai_dnat_connections`（每条 DNAT 规则的连接数）。如需每条连接的明细指标 `ikuai_session_info`，需显式开启（高基数，谨慎使用）：

```yaml
environment:
    IKUAI_SESSION_DETAIL: "true"
    IKUAI_SESSION_DETAIL_LIMIT: "200"
```

### 端口映射 / 会话指标（iKuai 3.x / 4.x）

| 指标 | 说明 |
|:---|:---|
| `ikuai_dnat_info` | 端口映射规则清单 |
| `ikuai_dnat_total` / `ikuai_dnat_enabled_total` / `ikuai_dnat_disabled_total` | 规则计数 |
| `ikuai_dnat_connections` | 每条启用规则当前匹配连接数 |
| `ikuai_session_total` | 当前会话总数 |
| `ikuai_session_info` | 可选会话明细（需 `IKUAI_SESSION_DETAIL=true`，高基数） |
| `ikuai_exporter_metrics_collector_status{type="dnat"}` | DNAT 采集状态（`0` 成功，`1` 失败） |

从 v0.2.1 开始，可以使用环境变量来设置上面的参数，格式为 `IKUAI_XXX`（flag 名中的 `-` 对应 `_`），如 `IKUAI_URL=http://10.0.1.253`、`IKUAI_USERNAME=test`、`IKUAI_SESSION_DETAIL=true`。

下面的方式依然支持，但**将在以后版本中弃用**。

| 变量名     | 说明     |
|:------- |:------ |
| IK_URL  | 爱快地址   |
| IK_USER | 爱快登录用户 |
| IK_PWD  | 爱快登录密码 |
