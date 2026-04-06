FROM node:22-alpine AS frontend-builder

WORKDIR /aiproxy/web

# Chinese npm mirror for faster downloads in mainland China
RUN npm config set registry https://registry.npmmirror.com && \
    npm install -g pnpm && \
    pnpm config set registry https://registry.npmmirror.com

# Cache layer: only re-install when package.json/lock changes
COPY ./web/package.json ./web/pnpm-lock.yaml ./
RUN CI=true pnpm install

COPY ./web/ ./
RUN pnpm run build

FROM golang:1.26-alpine AS builder

# goproxy.cn: production server is in mainland China, cannot reach proxy.golang.org
ENV GOPROXY=https://goproxy.cn,direct

WORKDIR /aiproxy

# Cache layer: only re-download Go deps when go.mod/go.sum/go.work change
# Must copy all workspace module go.mod files for replace directives to resolve
COPY go.work go.work.sum ./
COPY core/go.mod core/go.sum ./core/
COPY mcp-servers/go.mod mcp-servers/go.sum ./mcp-servers/
COPY openapi-mcp/go.mod openapi-mcp/go.sum ./openapi-mcp/
RUN cd core && go mod download

# Now copy full source (changes here don't invalidate the download layer)
COPY ./ /aiproxy

COPY --from=frontend-builder /aiproxy/web/dist/ /aiproxy/core/public/dist/

WORKDIR /aiproxy/core

RUN sh scripts/swag.sh

RUN go build -tags enterprise -trimpath -ldflags "-s -w" -o aiproxy

# Pin alpine version to avoid base image changes invalidating cache
FROM alpine:3.21

# Chinese APK mirror for faster downloads in mainland China
RUN sed -i 's|dl-cdn.alpinelinux.org|mirrors.aliyun.com|g' /etc/apk/repositories && \
    apk add --no-cache ca-certificates tzdata ffmpeg curl && \
    rm -rf /var/cache/apk/*

RUN mkdir -p /aiproxy

WORKDIR /aiproxy

VOLUME /aiproxy

COPY --from=builder /aiproxy/core/aiproxy /usr/local/bin/aiproxy

ENV PUID=0 PGID=0 UMASK=022

ENV FFMPEG_ENABLED=true

EXPOSE 3000

HEALTHCHECK --interval=5s --timeout=3s --retries=10 \
  CMD curl -f http://localhost:3000/api/status || exit 1

ENTRYPOINT ["aiproxy"]
