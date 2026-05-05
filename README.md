# Server Monitor

## 项目简介

轻量级云原生服务器监控系统，实现「探针采集 → Prometheus 存储 → 告警推送 → Web 展示」的完整闭环。

**核心链路：**

```
server-probe → Prometheus → AlertManager → server-web webhook → Redis Pub/Sub → WebSocket → 前端大屏
server-probe/server-web stdout → Fluent Bit → Elasticsearch → Kibana
```

**当前能力：**

- 主机指标查询、实时广播与主机区筛选面板
- 活跃告警查询
- 最近告警事件查询
- 告警 WebSocket 实时推送
- Docker Compose 一键启动与联调
- Docker Compose 本地日志链路，支持 Kibana 查询结构化 JSON 日志

**技术要点：**

- Go 后端（Gin + Prometheus client + gopsutil）
- Prometheus 可观测性体系
- Redis 缓存 + Pub/Sub 广播
- AlertManager 告警管理
- WebSocket 实时推送（主机指标 + 告警）
- Vue 3 + TypeScript 暗色监控大屏
- Grafana Provisioning 自动加载 Prometheus 数据源和基础大盘
- Fluent Bit + Elasticsearch + Kibana 本地日志查询链路
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

### 方式一：Docker Compose 一键启动（本机完整部署）

```bash
make docker-up
```

访问 <http://localhost:8080> 查看监控大屏。

说明：
- `server-web` 容器会同时托管前端静态文件
- Docker Compose 默认只绑定宿主机 `127.0.0.1`，并开启 `server-web` 鉴权
- 监控大屏默认管理员账号为 `admin`，默认密码为 `server-monitor-local-admin`
- Grafana 地址为 <http://localhost:3000>，默认账号为 `admin`，默认密码为 `server-monitor-local-grafana`
- Kibana 地址为 <http://localhost:5601>，用于查询 `sm-logs-*` 日志索引
- 首次启动后，Prometheus 抓取和告警规则加载通常需要 `15-30` 秒
- Elasticsearch / Kibana 首次启动通常更慢，日志链路可查询前需要等待服务健康和 Fluent Bit 完成采集
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

1. 打开 <http://localhost:8080>，确认前端页面可以访问，并使用 `admin` / `server-monitor-local-admin` 登录。
2. 打开 <http://localhost:8080/healthz>，确认返回 `healthy: true`。
3. 打开 <http://localhost:8080/readyz>，确认 `prometheus` 和 `redis` 最终变为 `ok`。
4. 打开 <http://localhost:9091/targets>，确认 `server-probe` target 为 `UP`。
5. 打开 <http://localhost:3000>，使用 `admin` / `server-monitor-local-grafana` 登录，确认 Prometheus 数据源和 `Server Monitor Overview` 大盘已自动加载。
6. 登录后打开主机列表页面，确认能返回主机指标。
7. 登录后打开活跃告警页面，确认接口可访问，即使当前没有活跃告警。
8. 登录后打开最近告警事件页面，确认最近事件接口可访问。
9. 打开 <http://localhost:5601>，创建 Data View：`sm-logs-*`，时间字段选择 `@timestamp`。
10. 访问 <http://localhost:8080/healthz> 后，在 Kibana Discover 中按 `service: server-web` 或 `path: /healthz` 查询请求日志。
11. 打开 <http://localhost:3000> 的 `Server Monitor Overview`，确认 `Log Volume` 和 `Warning and Error Logs` 面板已加载。

### 本地日志链路说明

Docker Compose 模式会启动 Elasticsearch、Kibana 和 Fluent Bit：

- Elasticsearch：<http://localhost:9200>，单节点开发模式，关闭安全认证。
- Kibana：<http://localhost:5601>，用于查询 `sm-logs-*`。
- Fluent Bit：默认挂载 `/var/lib/docker/containers`，解析 Docker JSON 外层和应用 JSON 内层。

常用验证命令：

```bash
docker compose config
docker compose ps elasticsearch kibana fluent-bit
curl -sf http://localhost:9200/_cluster/health
curl -sf http://localhost:8080/healthz
```

如果 Fluent Bit 没有采集到日志，优先检查：

- 当前 Docker 环境是否能把宿主机 Docker 容器日志目录挂载进容器。
- `docker compose logs fluent-bit` 是否有 parser、tail 或 Elasticsearch output 错误。
- Elasticsearch 是否健康：`curl -sf http://localhost:9200/_cluster/health`。

Docker Desktop、WSL 或 rootless Docker 环境下，容器日志目录可能不是 `/var/lib/docker/containers`。此时可以通过 `DOCKER_CONTAINER_LOG_PATH` 覆盖宿主机路径，例如 `DOCKER_CONTAINER_LOG_PATH=/data/docker/containers make docker-up`。

Grafana 会自动 provision Elasticsearch 数据源：

- 数据源名称：`Elasticsearch`
- 索引：`sm-logs-*`
- 时间字段：`@timestamp`
- 日志消息字段：`msg`
- 日志级别字段：`level`

如果修改了 Grafana datasource 或 dashboard 配置，需要重启 Grafana 容器或重新执行 `make docker-up` 才会重新加载 provisioning 文件。

Elasticsearch 初始化由 `elasticsearch-init` 一次性服务完成：

- ILM policy：`sm-logs-policy`，默认 30 天后删除日志索引。
- Index template：`sm-logs-template`，匹配 `sm-logs-*`。
- 预留字段：`trace_id`、`span_id`，供后续链路追踪阶段使用。

验证初始化：

```bash
docker compose run --rm elasticsearch-init
curl -sf http://localhost:9200/_ilm/policy/sm-logs-policy
curl -sf http://localhost:9200/_index_template/sm-logs-template
```

注意：index template 只影响后续新建索引，不会自动修改已经存在的旧索引。

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

### Helm 日志链路

Helm Chart 在第二阶段会部署 Kubernetes 版日志链路：

- Elasticsearch：单节点 StatefulSet，Service 名称 `elasticsearch`，默认关闭安全认证，仅用于学习/开发环境。
- Kibana：Deployment，Service 名称 `kibana`，默认 NodePort `30561`。
- Fluent Bit：DaemonSet，每个 Node 一个 Pod，采集 `/var/log/containers/*.log`，按 CRI 格式解析并通过 Kubernetes filter 添加 Pod 元数据。
- Fluent Bit 启动前会等待 `sm-logs-policy` 和 `sm-logs-template` 初始化完成，避免先写入日志导致索引 mapping 不稳定。

默认开关位于 `charts/server-monitor/values.yaml`：

```yaml
elasticsearch:
  enabled: true
kibana:
  enabled: true
fluentBit:
  enabled: true
config:
  logLevel: info
```

验证渲染：

```bash
helm lint charts/server-monitor
helm template server-monitor charts/server-monitor > /tmp/server-monitor-rendered.yaml
kubectl apply --dry-run=client -f /tmp/server-monitor-rendered.yaml
```

部署后验证：

```bash
kubectl get pods
kubectl get daemonset fluent-bit
kubectl logs daemonset/fluent-bit
kubectl port-forward svc/kibana 5601:5601
```

在 Kibana 中创建 Data View：`sm-logs-*`，时间字段选择 `@timestamp`。访问 `server-web /healthz` 后，应能按 `service: server-web`、`path: /healthz`、`kubernetes.namespace_name` 等字段查询日志。

注意：Fluent Bit 创建了 ClusterRole / ClusterRoleBinding 读取 Pod 和 Namespace 元数据，安装 Chart 的账号需要集群级 RBAC 权限。不同 Kubernetes 发行版的容器日志路径可能不同，如无法采集日志，应先检查节点上的 `/var/log/containers` 和 Fluent Bit Pod 挂载。

Helm Chart 也会在 Grafana provisioning 中新增 Elasticsearch 数据源，并在 `Service Monitor` Dashboard 中增加 `Log Volume` 与 `Warning and Error Logs` 面板。Grafana Pod 需要重启或重新部署后才会加载新的 provisioning 配置。

Helm Chart 会通过 `elasticsearch-init` Job 初始化 `sm-logs-policy` 和 `sm-logs-template`。默认保留期在 `charts/server-monitor/values.yaml` 中配置：

```yaml
elasticsearch:
  init:
    enabled: true
    retentionDays: 30
```

验证初始化：

```bash
kubectl get job elasticsearch-init
kubectl logs job/elasticsearch-init
kubectl exec statefulset/elasticsearch -- curl -sf http://127.0.0.1:9200/_ilm/policy/sm-logs-policy
kubectl exec statefulset/elasticsearch -- curl -sf http://127.0.0.1:9200/_index_template/sm-logs-template
```

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
│   ├── alerts.yml         # 告警规则
│   └── fluent-bit/        # Docker Compose 日志采集配置
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
| Elasticsearch| 9200 | `/_cluster/health`                | 日志索引存储          |
| Kibana       | 5601 | `/`                               | 日志查询              |

## 环境变量

### server-probe

| 变量                | 默认值      | 说明        |
| ----------------- | -------- | --------- |
| `LISTEN_ADDR`     | `:9090`  | 监听地址      |
| `SCRAPE_INTERVAL` | `5`      | 采集间隔（秒）   |
| `METRICS_PATH`    | `/metrics` | 指标路径      |
| `LOG_LEVEL`       | `info`     | JSON 日志级别：debug / info / warn / error |

### server-web

| 变量                        | 默认值                      | 说明          |
| ------------------------- | ------------------------ | ----------- |
| `LISTEN_ADDR`             | `:8080`                  | 监听地址        |
| `PROMETHEUS_URL`          | `http://prometheus:9090` | Prometheus 地址 |
| `REDIS_ADDR`              | (空)                      | Redis 地址    |
| `REDIS_PASSWORD`          | (空)                      | Redis 密码    |
| `REDIS_DB`                | `0`                      | Redis 数据库   |
| `JWT_SECRET`              | (空)                      | JWT 签名密钥，开启鉴权时至少 32 字节 |
| `JWT_EXPIRE_HOURS`        | `24`                     | JWT 有效期（小时） |
| `AUTH_ENABLED`            | `true`                   | 是否启用登录鉴权 |
| `ADMIN_PASSWORD`          | (空)                      | 初始管理员密码 |
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
| `LOG_LEVEL`               | `info`                   | JSON 日志级别：debug / info / warn / error |

补充说明：
- Docker Compose 部署时，`server-web` 镜像内默认使用 `STATIC_DIR=/app/static`
- Docker Compose 本地部署默认只绑定 `127.0.0.1`，并使用 `AUTH_ENABLED=true`、`ADMIN_PASSWORD=server-monitor-local-admin`、`REDIS_PASSWORD=server-monitor-local-redis`，生产环境必须通过环境变量覆盖默认密码和密钥
- K8s / Helm 部署默认使用 `monitor-secret` 注入 `REDIS_PASSWORD`、`MYSQL_PASSWORD`、`MYSQL_ROOT_PASSWORD`、`JWT_SECRET`、`ADMIN_PASSWORD`、`GRAFANA_ADMIN_USER`、`GRAFANA_ADMIN_PASSWORD`，生产环境必须替换 Secret 中的默认密码和密钥
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
- Fluent Bit + Elasticsearch + Kibana
- Docker / Kubernetes
