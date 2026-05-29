# shortr — multi-stage container.
#
# Stages:
#   1. web   — Bun builds the Astro dashboard → web/dist/
#   2. go    — Go compiles the binary with web/dist/ embedded
#   3. final — debian-slim runtime, exec the binary directly
#
# Data lives in /data (Fly volume). Disaster recovery via Fly's daily
# volume snapshots (configured in deploy/fly.toml under [[mounts]]).
# No sidecars, no external backup service.

# ---------- 1. dashboard build ----------
FROM oven/bun:1.3-debian AS web
WORKDIR /src/web

COPY web/package.json web/bun.lock* ./
RUN bun install --frozen-lockfile || bun install

COPY web/ ./
RUN bun run build

# ---------- 2. go build ----------
FROM golang:1.25-bookworm AS go
ENV CGO_ENABLED=0 \
    GOFLAGS="-trimpath -mod=readonly"
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
COPY --from=web /src/web/dist ./web/dist

ARG VERSION=dev
RUN go build -ldflags="-s -w -X main.version=${VERSION}" -o /out/shortr .

# ---------- 3. runtime ----------
FROM debian:bookworm-slim

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates tini sqlite3 \
    && rm -rf /var/lib/apt/lists/*

COPY --from=go /out/shortr /usr/local/bin/shortr

ENV PORT=8080 \
    DB_PATH=/data/shortr.db
EXPOSE 8080
VOLUME ["/data"]

# tini = PID 1 + signal handling. shortr serve is the entire process.
ENTRYPOINT ["/usr/bin/tini", "--", "/usr/local/bin/shortr", "serve"]
