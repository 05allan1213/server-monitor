# Server Monitor

## 项目简介

轻量级云原生服务器监控系统，实现「探针采集 → Prometheus 存储 → 告警推送 → Web 展示」的完整闭环。

**核心链路：**

```
server-probe → Prometheus → AlertManager → server-web webhook → Redis → WebSocket → 前端
```

**技术要点：**

- Go 后端（Gin + Prometheus client + gopsutil）
- Prometheus + Grafana 可观测性体系
- Redis 缓存 + Pub/Sub 广播
- AlertManager 告警管理
- WebSocket 实时推送
- Vue3 + TypeScript 前端
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
                                │  │ (缓存)   │  │  (告警面板)    │  │
                                │  └─────────┘  └───────────────┘  │
                                └──────────────────────────────────┘
                                       :8080
```

## 快速开始

### Docker Compose 一键启动

```bash
make docker-up
```

访问 <http://localhost:8080> 查看监控面板。

### 本地运行

```bash
# 1. 启动依赖（Redis + Prometheus + AlertManager）
docker-compose up -d redis prometheus alertmanager

# 2. 启动探针
make run-probe

# 3. 启动 Web（新终端）
PROMETHEUS_URL=http://localhost:9091 REDIS_ADDR=localhost:6379 make run-web
```

## Makefile 命令

```bash
make help
```

| 命令                 | 说明            |
| ------------------ | ------------- |
| `make build`       | 构建所有服务        |
| `make docker-up`   | Docker 启动所有服务 |
| `make docker-down` | Docker 停止所有服务 |
| `make run-probe`   | 本地运行探针        |
| `make run-web`     | 本地运行 Web      |
| `make clean`       | 清理构建产物        |

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
│   ├── webhook/           # AlertManager webhook 处理
│   ├── websocket/         # WebSocket Hub
│   ├── Dockerfile
│   └── go.mod
├── frontend/              # Vue3 前端：告警面板
│   ├── src/
│   ├── Dockerfile (通过 server-web Dockerfile 构建)
│   └── package.json
├── docker/                # Docker Compose 专用配置
│   ├── prometheus.yml
│   └── alertmanager.yml
├── k8s/                   # Kubernetes 部署清单
├── docker-compose.yml     # Docker Compose 编排
├── Makefile               # 常用命令
└── README.md
```

## 接口

| 服务           | 端口   | 路径                                | 说明                |
| ------------ | ---- | --------------------------------- | ----------------- |
| server-web   | 8080 | `/`                               | 监控面板（前端）          |
| server-web   | 8080 | `/api/v1/hosts`                   | 主机列表              |
| server-web   | 8080 | `/api/v1/alerts/active`           | 活跃告警              |
| server-web   | 8080 | `/ws/alerts`                      | 告警 WebSocket 推送   |
| server-web   | 8080 | `/api/v1/webhook/alertmanager`    | AlertManager 回调   |
| server-web   | 8080 | `/healthz` / `/readyz`            | 健康检查              |
| server-probe | 9090 | `/metrics`                        | Prometheus 指标     |

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
| `REQUEST_TIMEOUT_SECONDS` | `5`                      | 请求超时（秒）    |
| `READY_TIMEOUT_SECONDS`   | `3`                      | 就绪检查超时（秒）  |
| `HOSTS_CACHE_TTL_SECONDS` | `30`                     | 主机缓存 TTL（秒） |
| `STATIC_DIR`              | (空)                      | 前端静态文件目录   |
| `GIN_MODE`                | `debug`                  | Gin 模式      |

## 技术栈

- Go 1.26
- Gin（HTTP 框架）
- gopsutil（系统指标采集）
- Prometheus client_golang（指标暴露）
- go-redis（Redis 客户端）
- gorilla/websocket（WebSocket）
- Vue 3 + TypeScript + Vite（前端）
- Redis 7（缓存 + Pub/Sub）
- Prometheus + AlertManager + Grafana
- Docker / Kubernetes
