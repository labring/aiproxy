FROM golang:1.24-alpine AS builder

RUN apk add --no-cache curl

WORKDIR /aiproxy/core

COPY ./ /aiproxy

RUN sh scripts/tiktoken.sh

RUN go install github.com/swaggo/swag/cmd/swag@latest

RUN sh scripts/swag.sh

RUN go build -trimpath -tags "jsoniter" -ldflags "-s -w" -o aiproxy

# Frontend build stage
FROM node:23-alpine AS frontend-builder

WORKDIR /aiproxy/web

COPY ./web/ ./

# Install pnpm globally
RUN npm install -g pnpm

# Install dependencies and build with pnpm
RUN pnpm install && pnpm run build

FROM alpine:latest

RUN mkdir -p /aiproxy

WORKDIR /aiproxy

VOLUME /aiproxy

RUN apk add --no-cache ca-certificates tzdata ffmpeg curl && \
    rm -rf /var/cache/apk/*

COPY --from=builder /aiproxy/core/aiproxy /usr/local/bin/aiproxy
# Copy frontend dist files
COPY --from=frontend-builder /aiproxy/web/dist/ ./web/dist/

ENV PUID=0 PGID=0 UMASK=022

ENV FFMPEG_ENABLED=true

EXPOSE 3000

ENTRYPOINT ["aiproxy"]
