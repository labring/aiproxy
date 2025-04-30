FROM golang:1.24-alpine AS builder

RUN apk add --no-cache curl

WORKDIR /aiproxy/core

COPY ./ /aiproxy

RUN sh scripts/tiktoken.sh

RUN go install github.com/swaggo/swag/cmd/swag@latest

RUN sh scripts/swag.sh

RUN go build -trimpath -tags "jsoniter" -ldflags "-s -w" -o aiproxy

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
