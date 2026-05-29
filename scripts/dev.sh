#!/usr/bin/env bash
# Run `go run ./cmd/shortr` and `bun run dev` in web/ concurrently.
# Ctrl-C stops both.
set -euo pipefail

cd "$(dirname "$0")/.."

if [[ ! -f .env ]]; then
  echo "warn: no .env file; copying .env.example"
  cp .env.example .env
fi

# shellcheck disable=SC1091
set -a; source .env; set +a

trap 'kill 0' SIGINT SIGTERM EXIT

# Astro dev (writes to web/dist/, not used yet by the Go server in dev mode —
# the Go server reads templates from the embedded fs at compile time. For
# instant feedback in dev, we hit the Astro dev server directly on :4321.)
( cd web && bun run dev ) &

# Go server with verbose logs
LOG_LEVEL=debug LOG_FORMAT=text go run . serve &

wait
