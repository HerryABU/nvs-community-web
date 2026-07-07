# ==================== 构建阶段 ====================
FROM golang:1.21-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /build

# 先复制依赖文件，利用 Docker 缓存
COPY go.mod go.sum ./
RUN go mod download

# 复制源码并构建
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o nvs-server .

# ==================== 运行阶段 ====================
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata
ENV TZ=Asia/Shanghai

WORKDIR /app

COPY --from=builder /build/nvs-server .

# 创建数据目录
RUN mkdir -p /data/novels /data/uploads

EXPOSE 8080

CMD ["./nvs-server"]
