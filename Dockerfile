# syntax=docker/dockerfile:1.7
# ---------- builder ----------
FROM golang:1.26-bookworm AS builder

# CGO is required for govips.
ENV CGO_ENABLED=1 GOFLAGS=-trimpath

# libvips-dev provides headers + .so for govips to link against.
# pkg-config + build-essential are needed by cgo.
RUN apt-get update && apt-get install -y --no-install-recommends \
        build-essential \
        pkg-config \
        libvips-dev \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -ldflags="-s -w" -o /out/imget ./cmd/imget

# ---------- runtime ----------
FROM debian:12-slim

# libvips42 is the runtime shared library — no -dev needed.
# Includes built-in support for WebP, AVIF (via libheif/libaom), JPEG, PNG,
# GIF, TIFF, and more.
RUN apt-get update && apt-get install -y --no-install-recommends \
        libvips42 \
        ca-certificates \
        tzdata \
        wget \
    && rm -rf /var/lib/apt/lists/* \
    && groupadd -r imget && useradd -r -g imget imget

WORKDIR /app
COPY --from=builder /out/imget /app/imget

RUN mkdir -p /app/database /app/images && chown -R imget:imget /app

USER imget
EXPOSE 8080

ENV HTTP_ADDR=:8080 \
    DB_PATH=/app/database/imget.sqlite \
    IMAGES_DIR=/app/images

HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
    CMD wget -qO- http://127.0.0.1:8080/healthz || exit 1

ENTRYPOINT ["/app/imget"]
CMD ["serve"]
