# shortr — multi-stage container.
#
# Stages:
#   1. web   — Bun builds the Astro dashboard → web/dist/
#   2. go    — Go compiles the binary with web/dist/ embedded
#   3. fetch-litestream — pulls the Litestream binary from its scratch image
#   4. final — debian-slim runtime; entrypoint is a wrapper that runs
#              litestream replicate as a sidecar alongside shortr serve
#
# Pinned tags. Bump deliberately; CI verifies green build + tests.

# ---------- 1. dashboard build ----------
FROM oven/bun:1.3-debian AS web
WORKDIR /src/web

# Cache deps separately
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

# Bring in the rest, including web/dist/ from the previous stage.
COPY . .
COPY --from=web /src/web/dist ./web/dist

ARG VERSION=dev
RUN go build -ldflags="-s -w -X main.version=${VERSION}" -o /out/shortr .

# ---------- 3. litestream binary ----------
FROM litestream/litestream:0.5.11 AS litestream

# ---------- 4. runtime ----------
FROM debian:bookworm-slim

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates tini sqlite3 \
    && rm -rf /var/lib/apt/lists/*

COPY --from=litestream /usr/local/bin/litestream /usr/local/bin/litestream
COPY --from=go         /out/shortr               /usr/local/bin/shortr
COPY deploy/litestream.yml                       /etc/litestream.yml
COPY deploy/start.sh                             /usr/local/bin/start.sh
RUN chmod +x /usr/local/bin/start.sh

ENV PORT=8080 \
    DB_PATH=/data/shortr.db
EXPOSE 8080
VOLUME ["/data"]

# tini = PID 1 + signal handling. start.sh launches litestream replicate in
# the background, then execs shortr serve in the foreground.
ENTRYPOINT ["/usr/bin/tini", "--", "/usr/local/bin/start.sh"]
