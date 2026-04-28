# Server Monitor - 云原生服务器监控平台

基于 Go 语言开发的轻量级服务器实时监控系统，采用探针采集 + Web 展示的经典架构，集成 Prometheus + Grafana 可观测性体系，支持 Docker Compose 一键部署和 Kubernetes 编排。

## 项目架构

```
┌─────────────┐       ┌───────────┐       ┌─────────────┐
│ server-probe │──────▶│   MySQL   │◀──────│  server-web  │
│  (探针 Agent) │       │ (数据存储)  │       │  (Web 展示)   │
└──────┬───────┘       └───────────┘       └──────┬──────┘
       │                                          │
       │ :9090/metrics                            │ :8080
       ▼                                          ▼
┌─────────────┐                            ┌───────────┐
│  Prometheus  │───────────────────────────▶│  Grafana   │
│  (指标抓取)   │                            │  (可视化)   │
└─────────────┘                            └───────────┘
```

### 核心组件

| 组件 | 目录 | 说明 |
|------|------|------|
| **server-probe** | `server-probe/` | 监控探针，部署在被监控服务器上，每 5 秒采集 CPU/内存数据写入 MySQL，同时暴露 Prometheus 指标 |
| **server-web** | `server-web/` | Web 展示服务，基于 Gin 框架，从 MySQL 读取最近监控数据，以 HTML 表格实时展示 |
| **MySQL** | - | 数据存储，保存 `server_metrics` 表 |
| **Prometheus** | `k8s/prometheus.yaml` | 指标抓取与存储 |
| **Grafana** | `k8s/grafana.yaml` | 监控数据可视化仪表盘 |

### 数据流

```
server-probe ──(每5秒写入)──> MySQL ──(查询最近10条)──> server-web ──(HTML表格)──> 浏览器
     │
     └──(暴露 /metrics)──> Prometheus ──> Grafana
```

## 技术栈

- **语言**: Go 1.23+
- **Web 框架**: Gin
- **ORM**: GORM
- **系统指标采集**: gopsutil
- **监控指标**: Prometheus client_golang
- **数据库**: MySQL 8.0
- **容器化**: Docker (多阶段构建)
- **编排**: Docker Compose / Kubernetes
- **CI/CD**: GitHub Actions → DockerHub

## 快速启动

### 方式一：Docker Compose（推荐）

```bash
docker-compose up --build
```

启动后访问：
- Web 监控面板：http://localhost:8080
- Prometheus 指标：http://localhost:9090/metrics（需在 docker-compose.yml 中为 server-probe 暴露 9090 端口）

### 方式二：本地运行

前置条件：本地已安装 Go 1.23+ 和 MySQL 8.0。

1. 创建 MySQL 数据库和用户：

```sql
CREATE DATABASE monitor_db;
CREATE USER 'xiu'@'%' IDENTIFIED BY '12345678';
GRANT ALL PRIVILEGES ON monitor_db.* TO 'xiu'@'%';
FLUSH PRIVILEGES;
```

2. 启动探针：

```bash
cd server-probe
go run main.go
```

3. 启动 Web 服务：

```bash
cd server-web
go run main.go
```

4. 浏览器访问 http://localhost:8080

### 方式三：Kubernetes 部署

```bash
kubectl apply -f k8s/
```

K8s 资源清单包含：MySQL、Probe、Web、Prometheus、Grafana、Ingress、HPA、ConfigMap、Secret。

## 环境变量

server-probe 和 server-web 共享以下环境变量：

| 变量名 | 默认值 | 说明 |
|--------|--------|------|
| `DB_HOST` | `127.0.0.1` (probe) / `192.168.106.132` (web) | MySQL 主机地址 |
| `DB_PORT` | `3306` | MySQL 端口 |
| `DB_USER` | `xiu` | MySQL 用户名 |
| `DB_PASSWORD` | `12345678` | MySQL 密码 |
| `DB_NAME` | `monitor_db` | 数据库名 |

Docker Compose 环境下 `DB_HOST` 会自动设为 `mysql`（容器服务名）。

## 接口说明

### server-probe

| 方法 | 路径 | 端口 | 说明 |
|------|------|------|------|
| GET | `/metrics` | 9090 | Prometheus 指标端点，暴露 `probe_cpu_usage_percent` 和 `probe_mem_usage_percent` 两个 Gauge 指标 |

### server-web

| 方法 | 路径 | 端口 | 说明 |
|------|------|------|------|
| GET | `/` | 8080 | 监控大盘页面，返回 HTML 表格，展示最近 10 条监控数据，页面每 2 秒自动刷新 |

### Prometheus 指标

| 指标名 | 类型 | 说明 |
|--------|------|------|
| `probe_cpu_usage_percent` | Gauge | 当前 CPU 使用率百分比 |
| `probe_mem_usage_percent` | Gauge | 当前内存使用率百分比 |

### 数据库表结构

表名：`server_metrics`（由 server-web 启动时通过 GORM AutoMigrate 自动创建）

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | UINT (PK) | 自增主键 |
| `ip_address` | VARCHAR | 主机名（通过 `os.Hostname()` 获取） |
| `cpu_percent` | FLOAT | CPU 使用率 (%) |
| `mem_percent` | FLOAT | 内存使用率 (%) |
| `report_time` | DATETIME | 上报时间 |

## 项目结构

```
server-monitor/
├── .github/workflows/ci.yaml   # CI/CD：构建并推送 Docker 镜像
├── k8s/                        # Kubernetes 部署清单
│   ├── configmap.yaml
│   ├── grafana.yaml
│   ├── hpa.yaml
│   ├── ingress.yaml
│   ├── mysql.yaml
│   ├── probe.yaml
│   ├── prometheus.yaml
│   ├── secret.yaml
│   └── web.yaml
├── server-probe/               # 监控探针
│   ├── Dockerfile
│   ├── go.mod
│   ├── go.sum
│   └── main.go
├── server-web/                 # Web 展示服务
│   ├── Dockerfile
│   ├── go.mod
│   ├── go.sum
│   └── main.go
├── docker-compose.yml          # Docker Compose 编排
└── .gitignore
```

## CI/CD

推送到 `main` 分支时，GitHub Actions 自动构建并推送 Docker 镜像至 DockerHub：

- `xiujacksun/server-probe:latest`
- `xiujacksun/server-web:latest`
