# 多阶段构建Dockerfile

# 第一阶段：构建阶段
FROM golang:1.21-alpine AS builder

# 设置工作目录
WORKDIR /app

# 复制go mod文件
COPY go.mod go.sum ./

# 下载依赖
RUN go mod download

# 复制源代码
COPY . .

# 构建应用 (禁用CGO以获得静态二进制文件)
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o datamiddleware ./cmd/server

# 第二阶段：运行阶段
FROM alpine:latest

# 安装ca-certificates用于HTTPS请求
RUN apk --no-cache add ca-certificates tzdata

# 创建非root用户
RUN addgroup -g 1001 -S appgroup && \
    adduser -u 1001 -S appuser -G appgroup

# 设置工作目录
WORKDIR /home/appuser/

# 从构建阶段复制二进制文件
COPY --from=builder /app/datamiddleware .

# 复制配置文件
COPY --from=builder /app/configs ./configs/

# 创建日志目录
RUN mkdir -p logs && \
    chown -R appuser:appgroup /home/appuser/

# 切换到非root用户
USER appuser

# 暴露端口
EXPOSE 8080 9090

# 健康检查
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# 启动命令
CMD ["./datamiddleware"]
