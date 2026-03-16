# 构建阶段
FROM golang:1.24-alpine AS builder

WORKDIR /app

# 安装依赖
RUN apk add --no-cache git

# 复制依赖文件
COPY go.mod go.sum ./
RUN go mod download

# 复制源代码
COPY *.go ./

# 编译
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o jcp-mcp-server main.go sse_server.go

# 运行阶段
FROM alpine:latest

# 安装 ca-certificates 用于 HTTPS
RUN apk --no-cache add ca-certificates tzdata

WORKDIR /root/

# 从构建阶段复制可执行文件
COPY --from=builder /app/jcp-mcp-server .

# 暴露端口（SSE 模式）
EXPOSE 8080

# 默认以 SSE 模式运行
ENV MCP_MODE=sse
ENV PORT=8080

# 健康检查
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# 启动命令
ENTRYPOINT ["./jcp-mcp-server"]
CMD ["-mode=sse"]
