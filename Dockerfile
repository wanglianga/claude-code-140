# ---- 前端构建 ----
FROM node:20-alpine AS frontend
WORKDIR /fe
COPY frontend/package.json frontend/package-lock.json* ./
RUN npm install --no-audit --no-fund
COPY frontend/ ./
RUN npm run build

# ---- 后端构建 ----
FROM golang:1.19-alpine AS backend
WORKDIR /src
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 go build -o /out/pvserver .

# ---- 运行镜像(全栈单容器:Go 同时提供 API 与前端静态文件) ----
FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata && \
    adduser -D -u 10001 appuser
WORKDIR /app
COPY --from=backend /out/pvserver /app/pvserver
COPY --from=frontend /fe/dist /app/dist
ENV TZ=Asia/Shanghai \
    HTTP_ADDR=":8080" \
    FRONTEND_DIST="/app/dist" \
    AUTO_MIGRATE="true"
USER appuser
EXPOSE 8080
ENTRYPOINT ["/app/pvserver"]
