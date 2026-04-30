# Server Monitor

## 项目简介

轻量级云原生服务器监控系统，实现「探针采集 → Prometheus 存储 → 告警推送 → Web 展示」的完整闭环。

**核心链路：**

```
server-probe → Prometheus → AlertManager → server-web webhook → Redis Pub/Sub → WebSocket → 前端大屏
```

**当前能力：**

- 主机指标查询、实时广播与主机区筛选面板
- 活跃告警查询
- 最近告警事件查询
- 告警 WebSocket 实时推送
- Docker Compose 一键启动与联调

**技术要点：**

- Go 后端（Gin + Prometheus client + gopsutil）
- Prometheus 可观测性体系
- Redis 缓存 + Pub/Sub 广播
- AlertManager 告警管理
- WebSocket 实时推送（主机指标 + 告警）
- Vue 3 + TypeScript 暗色监控大屏
- Grafana Provisioning 自动加载 Prometheus 数据源和基础大盘
- Docker / Kubernetes 部署

## 架构

```
┌─────────────┐  :9090/metrics  ┌─────────────┐  alert  ┌──────────────┐
│ server-probe │───────────────▶│  Prometheus  │───────▶│ AlertManager │
│  (采集探针)   │                │  (指标存储)   │        │  (告警管理)    │
└─────────────┘                 └──────┬───────┘        └──────┬───────┘
                                       │ query                  │ webhook
                                       ▼                        ▼
                                ┌──────────────────────────────────┐
                                │           server-web             │
                                │  (API + WebSocket + 静态文件托管)  │
                                │                                  │
                                │  ┌─────────┐  ┌───────────────┐  │
                                │  │  Redis  │  │  Frontend     │  │
                                │  │(缓存+广播)│  │ (监控大屏)     │  │
                                │  └─────────┘  └───────────────┘  │
                                └──────────────────────────────────┘
                                       :8080
```

## 快速开始

### 方式一：Docker Compose 一键启动（生产/完整部署）

```bash
make docker-up
```

访问 <http://localhost:8080> 查看监控大屏。

说明：
- `server-web` 容器会同时托管前端静态文件
- Grafana 地址为 <http://localhost:3000>，默认账号为 `admin`，默认密码为 `server-monitor-local-grafana`
- 首次启动后，Prometheus 抓取和告警规则加载通常需要 `15-30` 秒
- 如果刚启动就访问 `/readyz`，短时间内返回未就绪是正常现象

### 方式二：开发模式（推荐开发阶段使用）

无需构建 Docker 镜像，改代码后秒级生效：

```bash
# 终端 1：启动依赖服务
make dev-deps

# 终端 2：本地运行 server-web（改代码后 Ctrl+C 重启）
make dev-web

# 终端 3：本地运行前端开发服务器（Vite 热更新）
make dev-frontend
```

前端开发服务器访问 <http://localhost:5173>，Vite proxy 自动转发 API 和 WebSocket 到 server-web。

停止开发依赖服务：

```bash
make dev-stop
```

## 第一阶段验收步骤

### Docker Compose 模式

```bash
make docker-up
```

建议按下面顺序检查：

1. 打开 <http://localhost:8080>，确认前端页面可以访问。
2. 打开 <http://localhost:8080/healthz>，确认返回 `healthy: true`。
3. 打开 <http://localhost:8080/readyz>，确认 `prometheus` 和 `redis` 最终变为 `ok`。
4. 打开 <http://localhost:9091/targets>，确认 `server-probe` target 为 `UP`。
5. 打开 <http://localhost:3000>，使用 `admin` / `server-monitor-local-grafana` 登录，确认 Prometheus 数据源和 `Server Monitor Overview` 大盘已自动加载。
6. 打开 <http://localhost:8080/api/v1/hosts>，确认能返回主机指标 JSON。
7. 打开 <http://localhost:8080/api/v1/alerts/active>，确认接口可访问，即使当前没有活跃告警。
8. 打开 <http://localhost:8080/api/v1/alerts/events>，确认最近事件接口可访问。

### 开发模式

```bash
make dev-deps
make dev-web
make dev-frontend
```

开发模式下的访问入口：
- 前端开发页面：<http://localhost:5173>
- 后端健康检查：<http://localhost:8080/healthz>
- Prometheus：<http://localhost:9091>
- AlertManager：<http://localhost:9093>
- Grafana：<http://localhost:3000>

## Kubernetes 部署说明

当前 `k8s/` 目录中的第一阶段清单已经和现有运行模型基本对齐：

- 资源默认部署到 `server-monitor` namespace
- `server-web` 保留为 `Deployment`，`server-probe` 使用 `DaemonSet`
- 非敏感运行配置统一收口到 `monitor-config`
- Redis、Prometheus、AlertManager、Grafana 默认使用 PVC 持久化，集群需要可用的默认 StorageClass
- Ingress 默认启用 TLS，并引用 `monitor-tls` Secret
- `server-web` 通过同一个 Service 同时承载：
  - 前端页面 `/`
  - API `/api/v1/*`
  - WebSocket `/ws/alerts`
- Ingress 当前按原路径透传，不再做 `/ -> /` 重写

### 当前配置入口

`k8s/configmap.yaml` 中的 `monitor-config` 当前承载：

- `server-web` 非敏感配置：
  - `PROMETHEUS_URL`
  - `REDIS_ADDR`
  - `GIN_MODE`
  - `READY_TIMEOUT_SECONDS`
  - `REQUEST_TIMEOUT_SECONDS`
  - `HOSTS_CACHE_TTL_SECONDS`
- `server-probe` 非敏感配置：
  - `PROBE_LISTEN_ADDR`
  - `PROBE_METRICS_PATH`
  - `PROBE_SCRAPE_INTERVAL`

### Ingress 路径模型

当前 [k8s/ingress.yaml](k8s/ingress.yaml) 的设计是把请求原样交给 `server-web`：

- `/` -> 前端页面
- `/api/v1/*` -> 后端 API
- `/ws/alerts` -> WebSocket

这意味着如果要使用 Ingress，需要确保 Ingress Controller 不额外改写这些路径。

## Makefile 命令

```bash
make help
```

### 开发模式

| 命令                 | 说明                                        |
| ------------------ | ----------------------------------------- |
| `make dev-deps`    | 启动依赖服务（Redis/Prometheus/AlertManager/Grafana/Probe） |
| `make dev-web`     | 本地运行 server-web（需先启动 dev-deps）             |
| `make dev-frontend`| 本地运行前端开发服务器（需先启动 dev-web）                 |
| `make dev-stop`    | 停止开发依赖服务                                  |

### Docker 命令

| 命令                 | 说明            |
| ------------------ | ------------- |
| `make docker`      | 构建 Docker 镜像  |
| `make docker-up`   | Docker 启动所有服务 |
| `make docker-down` | Docker 停止所有服务 |
| `make docker-logs` | 查看服务日志        |
| `make docker-clean`| 停止并清理所有数据     |

### 其他命令

| 命令               | 说明            |
| ---------------- | ------------- |
| `make build`     | 构建所有服务        |
| `make run-probe` | 本地运行探针        |
| `make run-web`   | 本地运行 Web      |
| `make test`      | 运行测试          |
| `make fmt`       | 格式化代码         |
| `make tidy`      | 整理依赖          |
| `make clean`     | 清理构建产物        |

## 项目结构

```
server-monitor/
├── server-probe/          # 监控探针：采集 CPU/内存指标，暴露 /metrics
│   ├── collector/         # 采集器（CPU、Memory）
│   ├── config/            # 配置加载
│   ├── Dockerfile
│   └── go.mod
├── server-web/            # Web 后端：Prometheus 查询 + 告警推送 + 静态文件
│   ├── api/               # HTTP 路由和 handler
│   ├── config/            # 配置加载
│   ├── prometheus/        # Prometheus 客户端
│   ├── redis/             # Redis 缓存和 Pub/Sub
│   ├── pubsub/            # 告警广播 Hub
│   ├── websocket/         # WebSocket Hub
│   ├── Dockerfile
│   └── go.mod
├── frontend/              # Vue 3 前端：暗色监控大屏
│   ├── src/
│   │   ├── api/           # API 请求封装
│   │   ├── composables/   # 组合式函数（WebSocket）
│   │   ├── types/         # TypeScript 类型定义
│   │   ├── App.vue        # 主页面
│   │   └── style.css      # 全局主题样式
│   ├── vite.config.ts     # Vite 配置（含 dev proxy）
│   └── package.json
├── docker/                # Docker Compose 专用配置
│   ├── prometheus.yml     # Prometheus 采集配置
│   ├── alertmanager.yml   # AlertManager 配置
│   └── alerts.yml         # 告警规则
├── k8s/                   # Kubernetes 部署清单
├── docker-compose.yml     # Docker Compose 编排
├── Makefile               # 常用命令
└── README.md
```

## 接口

| 服务           | 端口   | 路径                                | 说明                |
| ------------ | ---- | --------------------------------- | ----------------- |
| server-web   | 8080 | `/`                               | 监控大屏（前端）          |
| server-web   | 8080 | `/api/v1/hosts`                   | 主机列表              |
| server-web   | 8080 | `/api/v1/alerts/active`           | 活跃告警              |
| server-web   | 8080 | `/api/v1/alerts/events`           | 最近告警事件          |
| server-web   | 8080 | `/ws/alerts`                      | WebSocket 实时推送    |
| server-web   | 8080 | `/api/v1/webhook/alertmanager`    | AlertManager 回调   |
| server-web   | 8080 | `/healthz` / `/readyz`            | 健康检查              |
| server-probe | 9090 | `/metrics`                        | Prometheus 指标     |
| Prometheus   | 9091 | `/`                               | Prometheus 控制台    |
| AlertManager | 9093 | `/`                               | AlertManager 控制台  |
| Grafana      | 3000 | `/`                               | Grafana 大盘        |

## 环境变量

### server-probe

| 变量                | 默认值      | 说明        |
| ----------------- | -------- | --------- |
| `LISTEN_ADDR`     | `:9090`  | 监听地址      |
| `SCRAPE_INTERVAL` | `5`      | 采集间隔（秒）   |
| `METRICS_PATH`    | `/metrics` | 指标路径      |

### server-web

| 变量                        | 默认值                      | 说明          |
| ------------------------- | ------------------------ | ----------- |
| `LISTEN_ADDR`             | `:8080`                  | 监听地址        |
| `PROMETHEUS_URL`          | `http://prometheus:9090` | Prometheus 地址 |
| `REDIS_ADDR`              | (空)                      | Redis 地址    |
| `REDIS_PASSWORD`          | (空)                      | Redis 密码    |
| `REDIS_DB`                | `0`                      | Redis 数据库   |
| `CORS_ALLOWED_ORIGINS`    | (空)                      | 允许跨域来源，多个用逗号分隔 |
| `RATE_LIMIT_ENABLED`      | `false`                  | 是否启用 Redis 滑动窗口限流 |
| `RATE_LIMIT_REQUESTS`     | `120`                    | 限流窗口内允许的请求数 |
| `RATE_LIMIT_WINDOW_SECONDS` | `60`                   | 限流窗口长度（秒） |
| `RATE_LIMIT_OPERATION_TIMEOUT_MILLISECONDS` | `500` | 限流 Redis 操作超时（毫秒） |
| `REQUEST_TIMEOUT_SECONDS` | `5`                      | 请求超时（秒）    |
| `READY_TIMEOUT_SECONDS`   | `3`                      | 就绪检查超时（秒）  |
| `HTTP_READ_HEADER_TIMEOUT_SECONDS` | `5`             | HTTP 请求头读取超时（秒） |
| `HTTP_READ_TIMEOUT_SECONDS` | `15`                   | HTTP 请求读取超时（秒） |
| `HTTP_WRITE_TIMEOUT_SECONDS` | `30`                  | HTTP 响应写入超时（秒） |
| `HTTP_IDLE_TIMEOUT_SECONDS` | `120`                  | HTTP 空闲连接超时（秒） |
| `SHUTDOWN_TIMEOUT_SECONDS` | `5`                    | 服务优雅关闭超时（秒） |
| `HOSTS_BROADCAST_INTERVAL_SECONDS` | `5`             | WebSocket 主机列表广播周期（秒） |
| `HOSTS_CACHE_TTL_SECONDS` | `30`                     | 主机缓存 TTL（秒） |
| `DASHBOARD_OVERVIEW_TTL_SECONDS` | `10`             | 总览缓存 TTL（秒） |
| `ALERT_EVENT_DEDUPE_TTL_SECONDS` | `86400`          | 告警事件去重 TTL（秒） |
| `ALERTMANAGER_WEBHOOK_MAX_BODY_BYTES` | `1048576`    | Alertmanager Webhook 请求体大小上限（字节） |
| `CACHE_WRITE_TIMEOUT_SECONDS` | `3`                 | 缓存写入超时（秒） |
| `REDIS_STARTUP_TIMEOUT_SECONDS` | `5`               | 启动时 Redis 检查超时（秒） |
| `REDIS_DIAL_TIMEOUT_SECONDS` | `5`                  | Redis 建连超时（秒） |
| `REDIS_READ_TIMEOUT_SECONDS` | `3`                  | Redis 读取超时（秒） |
| `REDIS_WRITE_TIMEOUT_SECONDS` | `3`                 | Redis 写入超时（秒） |
| `REDIS_CONN_MAX_LIFETIME_SECONDS` | `1800`          | Redis 连接最长生命周期（秒） |
| `REDIS_CONN_MAX_IDLE_TIME_SECONDS` | `300`           | Redis 空闲连接最长保留时间（秒） |
| `STATIC_DIR`              | (空)                      | 前端静态文件目录   |
| `GIN_MODE`                | `debug`                  | Gin 模式      |

补充说明：
- Docker Compose 部署时，`server-web` 镜像内默认使用 `STATIC_DIR=/app/static`
- Docker Compose 本地部署默认使用 `REDIS_PASSWORD=server-monitor-local-redis`，生产环境必须通过环境变量覆盖
- K8s / Helm 部署默认使用 `monitor-secret` 注入 `REDIS_PASSWORD`、`GRAFANA_ADMIN_USER`、`GRAFANA_ADMIN_PASSWORD`，生产环境必须替换 Secret 中的默认密码
- Redis 生产环境建议在宿主机开启 `vm.overcommit_memory=1`，否则 Redis 可能在后台保存或内存紧张时输出 warning 并存在失败风险
- 本地开发模式通常不设置 `STATIC_DIR`，由 Vite 开发服务器在 `5173` 端口提供前端页面

## 技术栈

- Go 1.26
- Gin（HTTP 框架）
- gopsutil（系统指标采集）
- Prometheus client_golang（指标暴露）
- go-redis（Redis 客户端）
- gorilla/websocket（WebSocket）
- Vue 3 + TypeScript + Vite（前端）
- Redis 7（缓存 + Pub/Sub）
- Prometheus + AlertManager
- Docker / Kubernetes
