# ============================================
# 第一阶段：编译（Builder Stage）
# ============================================
# 使用 Go 官方镜像作为编译环境
FROM golang:1.26-alpine AS builder

# 设置工作目录
WORKDIR /app

# 复制源代码到容器
COPY . .

# 设置 Go 代理，加速依赖下载
ENV GOPROXY=https://goproxy.cn,direct

# 下载依赖
RUN go mod download

# 静态编译（CGO_ENABLED=0 禁用 CGO，生成纯静态二进制）
RUN CGO_ENABLED=0 go build -o probe .

# ============================================
# 第二阶段：运行（Runtime Stage）
# ============================================
# 使用 Alpine 最小化镜像，减小镜像体积
FROM alpine:latest

# 安装时区数据（支持时区设置）
RUN apk add --no-cache tzdata

# 设置工作目录
WORKDIR /app

# 从编译阶段复制二进制文件
COPY --from=builder /app/probe .

# 容器启动命令
CMD ["/app/probe"]
