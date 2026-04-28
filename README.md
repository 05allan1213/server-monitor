# Server Monitor

## 项目简介

这是一个轻量级的服务器监控系统，实现了「探针采集 → 数据存储 → Web 展示」的完整链路，同时集成了 Prometheus + Grafana 可观测性体系。

**适合学习的知识点：**

- Go 语言 Web 开发（Gin + GORM）
- 系统指标采集（gopsutil）
- Prometheus 指标暴露
- Docker 多阶段构建
- Docker Compose 编排
- Kubernetes 部署
- GitHub Actions CI/CD

## 架构

```
┌─────────────┐       ┌───────────┐       ┌─────────────┐
│ server-probe │──────▶│   MySQL   │◀──────│  server-web  │
│  (探针 Agent) │       │ (数据存储)  │       │  (Web 展示)   │
└──────┬───────┘       └───────────┘       └──────────────┘
       │ :9090/metrics
       ▼
┌─────────────┐       ┌───────────┐
│  Prometheus  │──────▶│  Grafana   │
└─────────────┘       └───────────┘
```

## 快速开始

### Docker Compose 一键启动

```bash
make docker-up
```

访问 <http://localhost:8080> 查看监控面板。

### 本地运行

```bash
# 1. 创建数据库
mysql -u root -p
```

```sql
CREATE DATABASE monitor_db;
CREATE USER 'monitor'@'%' IDENTIFIED BY 'monitor123';
GRANT ALL PRIVILEGES ON monitor_db.* TO 'monitor'@'%';
FLUSH PRIVILEGES;
```

```bash
# 2. 启动探针
make run-probe

# 3. 启动 Web（新终端）
make run-web
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
├── server-probe/          # 监控探针
│   ├── main.go            # 采集 CPU/内存，写入 MySQL，暴露 Prometheus 指标
│   ├── Dockerfile
│   └── go.mod
├── server-web/            # Web 展示服务
│   ├── main.go            # Gin 框架，读取 MySQL，HTML 表格展示
│   ├── Dockerfile
│   └── go.mod
├── k8s/                   # Kubernetes 部署清单
├── docker-compose.yml     # Docker Compose 编排
├── Makefile               # 常用命令
└── README.md
```

## 接口

| 服务           | 端口   | 路径         | 说明            |
| ------------ | ---- | ---------- | ------------- |
| server-web   | 8080 | `/`        | 监控面板（每 2 秒刷新） |
| server-probe | 9090 | `/metrics` | Prometheus 指标 |

## 环境变量

| 变量            | 默认值          | 说明       |
| ------------- | ------------ | -------- |
| `DB_HOST`     | `127.0.0.1`  | MySQL 地址 |
| `DB_PORT`     | `3306`       | MySQL 端口 |
| `DB_USER`     | `monody`        | 用户名      |
| `DB_PASSWORD` | `12345678`   | 密码       |
| `DB_NAME`     | `monitor_db` | 数据库名     |

## 技术栈

- Go 1.26
- Gin + GORM
- gopsutil（系统指标）
- Prometheus client\_golang
- MySQL 8.0
- Docker / Kubernetes
- GitHub Actions

