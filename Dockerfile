# syntax=docker/dockerfile:1

# 1) 构建网页管理端
FROM node:24-alpine AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# 2) 构建 Service，把网页产物内嵌进二进制
FROM golang:1.27-alpine AS build
WORKDIR /src/service
COPY service/go.mod service/go.sum ./
RUN go mod download
COPY service/ ./
RUN rm -rf internal/webui/dist && mkdir -p internal/webui/dist
COPY --from=web /src/web/dist/ internal/webui/dist/
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=docker" -o /out/LianT ./cmd/liant

# 3) 运行镜像
FROM alpine:3.22
RUN apk add --no-cache ca-certificates tzdata && adduser -D -u 10001 liant
WORKDIR /app
COPY --from=build /out/LianT /usr/local/bin/LianT
COPY scripts/docker-entrypoint.sh /usr/local/bin/entrypoint.sh
RUN chmod +x /usr/local/bin/entrypoint.sh && mkdir -p /app/data && chown -R liant:liant /app
USER liant
ENV LIANT_HOME=/app
VOLUME ["/app"]
EXPOSE 8443
ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
CMD ["serve"]
