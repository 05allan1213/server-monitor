# 第一阶段：编译
FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY . .
ENV GOPROXY=https://goproxy.cn,direct
RUN go mod download
RUN CGO_ENABLED=0 go build -o web .

# 第二阶段：运行
FROM alpine:latest
RUN apk add --no-cache tzdata
WORKDIR /app
COPY --from=builder /app/web .
EXPOSE 8080
CMD ["/app/web"]