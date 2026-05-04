# ============================================
# 第一阶段：编译
# ============================================
FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY . .

ENV GOPROXY=https://goproxy.cn,direct

RUN go mod download
RUN CGO_ENABLED=0 go build -o alert-service .

# ============================================
# 第二阶段：运行
# ============================================
FROM alpine:3.19

RUN apk add --no-cache tzdata \
	&& addgroup -S app \
	&& adduser -S -G app app

WORKDIR /app

COPY --from=builder /app/alert-service .

USER app

EXPOSE 8081

CMD ["/app/alert-service"]
