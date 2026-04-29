.PHONY: all build build-probe build-web run run-probe run-web docker docker-up docker-down docker-logs clean test help

all: build

# ============================================
# 构建相关
# ============================================

build: build-probe build-web

build-probe:
	@echo "构建 server-probe..."
	cd server-probe && go build -o probe .

build-web:
	@echo "构建 server-web..."
	cd server-web && go build -o web .

# ============================================
# 本地运行
# ============================================

run-probe:
	@echo "启动 server-probe..."
	cd server-probe && go run main.go

run-web:
	@echo "启动 server-web..."
	cd server-web && go run main.go

# ============================================
# Docker 相关
# ============================================

docker:
	@echo "构建 Docker 镜像..."
	docker compose build

docker-up:
	@echo "启动 Docker Compose..."
	docker compose up -d
	@echo "服务已启动，访问 http://localhost:8080"

docker-down:
	@echo "停止 Docker Compose..."
	docker compose down

docker-logs:
	docker compose logs -f

docker-clean:
	@echo "清理 Docker 资源..."
	docker compose down -v
	@echo "清理完成"

# ============================================
# 测试与检查
# ============================================

test:
	@echo "运行测试..."
	cd server-probe && go test -v ./...
	cd server-web && go test -v ./...

fmt:
	@echo "格式化代码..."
	cd server-probe && go fmt ./...
	cd server-web && go fmt ./...

lint:
	@echo "静态检查..."
	@which golangci-lint > /dev/null || (echo "请先安装 golangci-lint" && exit 1)
	cd server-probe && golangci-lint run
	cd server-web && golangci-lint run

# ============================================
# 清理
# ============================================

clean:
	@echo "清理构建产物..."
	rm -f server-probe/probe
	rm -f server-web/web
	rm -f main
	@echo "清理完成"

# ============================================
# 依赖管理
# ============================================

tidy:
	@echo "整理依赖..."
	cd server-probe && go mod tidy
	cd server-web && go mod tidy

# ============================================
# 帮助
# ============================================

help:
	@echo "Server Monitor - Makefile 命令说明"
	@echo ""
	@echo "构建命令:"
	@echo "  make build          构建所有服务"
	@echo "  make build-probe    构建 server-probe"
	@echo "  make build-web      构建 server-web"
	@echo ""
	@echo "运行命令:"
	@echo "  make run-probe      运行 server-probe"
	@echo "  make run-web        运行 server-web"
	@echo ""
	@echo "Docker 命令:"
	@echo "  make docker         构建 Docker 镜像"
	@echo "  make docker-up      启动所有服务"
	@echo "  make docker-down    停止所有服务"
	@echo "  make docker-logs    查看服务日志"
	@echo "  make docker-clean   停止并清理所有数据"
	@echo ""
	@echo "测试与检查:"
	@echo "  make test           运行测试"
	@echo "  make fmt            格式化代码"
	@echo "  make lint           静态检查（需安装 golangci-lint）"
	@echo ""
	@echo "其他:"
	@echo "  make tidy           整理依赖"
	@echo "  make clean          清理构建产物"
	@echo "  make help           显示此帮助信息"
