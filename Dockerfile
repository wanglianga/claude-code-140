# ---- 1) 构建前端 ----
FROM node:22-alpine AS frontend
WORKDIR /app/frontend
COPY frontend/package*.json ./
RUN npm install --no-audit --no-fund
COPY frontend/ ./
RUN npm run build

# ---- 2) 构建后端（前端产物通过 go:embed 打入单一二进制）----
FROM golang:1.22-alpine AS backend
WORKDIR /app
ENV CGO_ENABLED=0 GOOS=linux
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
COPY --from=frontend /app/frontend/dist ./internal/web/dist
RUN go build -ldflags="-s -w" -o /pvserver ./cmd/server

# ---- 3) 运行镜像 ----
FROM alpine:3.20
RUN adduser -D -u 10001 appuser && apk add --no-cache ca-certificates wget
COPY --from=backend /pvserver /pvserver
USER appuser
EXPOSE 8080
HEALTHCHECK --interval=15s --timeout=5s --retries=5 \
  CMD wget -qO- http://127.0.0.1:8080/ >/dev/null 2>&1 || exit 1
ENTRYPOINT ["/pvserver"]
