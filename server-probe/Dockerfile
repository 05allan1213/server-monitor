# 第一阶段：编译
FROM golang:1.25-alpine AS builder
WORKDIR /app                          
COPY . .         
ENV GOPROXY=https://goproxy.cn,direct                     
RUN go mod download                   # 下载依赖
RUN CGO_ENABLED=0 go build -o probe .  # 静态编译成二进制文件

# 第二阶段：运行
FROM alpine:latest
RUN apk add --no-cache tzdata
WORKDIR /app
COPY --from=builder /app/probe .     
CMD ["/app/probe"]