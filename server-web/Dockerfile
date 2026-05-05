# ============================================
# 第一阶段：前端构建
# ============================================
FROM node:20-alpine AS frontend-builder

WORKDIR /app

COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci

COPY frontend/ ./
RUN npm run build

# ============================================
# 第二阶段：Go 编译
# ============================================
FROM golang:1.26-alpine AS go-builder

WORKDIR /app

COPY server-web/ ./

ENV GOPROXY=https://goproxy.cn,direct

RUN go mod download
RUN CGO_ENABLED=0 go build -o web .

# ============================================
# 第三阶段：运行
# ============================================
FROM alpine:3.19

RUN apk add --no-cache tzdata

WORKDIR /app

COPY --from=go-builder /app/web .
COPY --from=prom/prometheus:v2.51.0 /bin/promtool /usr/local/bin/promtool
COPY --from=frontend-builder /app/dist ./static

ENV STATIC_DIR=/app/static

EXPOSE 8080

CMD ["/app/web"]
