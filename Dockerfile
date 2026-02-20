FROM node:22-alpine AS frontend-builder

WORKDIR /aiproxy/web

COPY ./web/ ./

RUN npm install -g pnpm

RUN pnpm install && pnpm run build

FROM golang:1.26-alpine AS builder

WORKDIR /aiproxy/core

COPY ./ /aiproxy

COPY --from=frontend-builder /aiproxy/web/dist/ /aiproxy/core/public/dist/

RUN sh scripts/swag.sh

RUN go build -trimpath -ldflags "-s -w" -o aiproxy

FROM alpine:latest

RUN mkdir -p /aiproxy

WORKDIR /aiproxy

VOLUME /aiproxy

RUN apk add --no-cache ca-certificates tzdata ffmpeg curl && \
    rm -rf /var/cache/apk/*

COPY --from=builder /aiproxy/core/aiproxy /usr/local/bin/aiproxy

ENV PUID=0 PGID=0 UMASK=022

ENV FFMPEG_ENABLED=true

EXPOSE 3000

ENTRYPOINT ["aiproxy"]
