ARG USE_CN_MIRROR=true

FROM node:22-alpine AS frontend-builder

ARG USE_CN_MIRROR

WORKDIR /aiproxy/web

# Conditional npm mirror: CN mirror for domestic, official for overseas
RUN if [ "$USE_CN_MIRROR" = "true" ]; then \
      npm config set registry https://registry.npmmirror.com && \
      npm install -g pnpm && \
      pnpm config set registry https://registry.npmmirror.com; \
    else \
      npm install -g pnpm; \
    fi

# Cache layer: only re-install when package.json/lock changes
COPY ./web/package.json ./web/pnpm-lock.yaml ./
RUN CI=true pnpm install

COPY ./web/ ./
RUN pnpm run build

FROM golang:1.26-alpine AS builder

ARG USE_CN_MIRROR

# Conditional Go proxy: goproxy.cn for domestic, default for overseas
RUN if [ "$USE_CN_MIRROR" = "true" ]; then \
      go env -w GOPROXY=https://goproxy.cn,direct; \
    fi

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

ARG USE_CN_MIRROR

# Conditional APK mirror: CN mirrors with fallback chain (Tencent → Tsinghua → Aliyun)
RUN if [ "$USE_CN_MIRROR" = "true" ]; then \
      cp /etc/apk/repositories /etc/apk/repositories.bak && \
      sed -i 's|dl-cdn.alpinelinux.org|mirrors.tencent.com|g' /etc/apk/repositories && \
      apk add --no-cache ca-certificates tzdata ffmpeg curl || \
      ( sed -i 's|mirrors.tencent.com|mirrors.tuna.tsinghua.edu.cn|g' /etc/apk/repositories && \
        apk add --no-cache ca-certificates tzdata ffmpeg curl ) || \
      ( sed -i 's|mirrors.tuna.tsinghua.edu.cn|mirrors.aliyun.com|g' /etc/apk/repositories && \
        apk add --no-cache ca-certificates tzdata ffmpeg curl ); \
    else \
      apk add --no-cache ca-certificates tzdata ffmpeg curl; \
    fi && \
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
