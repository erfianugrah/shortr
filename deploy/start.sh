#!/usr/bin/env bash
# start.sh — entrypoint that runs Litestream alongside shortr.
#
# Litestream restores the SQLite DB from the configured replica on cold start
# (if local data is missing), then replicates ongoing writes back to S3 in the
# background. shortr serve runs in the foreground; if it exits, this script
# exits with the same code so Fly will restart the machine.
set -euo pipefail

DB_PATH="${DB_PATH:-/data/shortr.db}"
LITESTREAM_CONFIG="${LITESTREAM_CONFIG:-/etc/litestream.yml}"

# If a replica URL is configured AND no local DB exists, restore.
if [[ -n "${BUCKET_NAME:-}" ]] && [[ ! -f "$DB_PATH" ]]; then
  echo "[start.sh] no local db; restoring from replica…"
  if litestream restore -if-replica-exists -config "$LITESTREAM_CONFIG" "$DB_PATH"; then
    echo "[start.sh] restore complete: $(stat -c%s "$DB_PATH" 2>/dev/null || echo ?) bytes"
  else
    echo "[start.sh] no prior replica; starting fresh"
  fi
fi

# Start litestream replicate in the background if configured.
if [[ -n "${BUCKET_NAME:-}" ]]; then
  echo "[start.sh] starting litestream replicate"
  litestream replicate -config "$LITESTREAM_CONFIG" &
  LITESTREAM_PID=$!
  trap 'kill -TERM $LITESTREAM_PID 2>/dev/null || true' SIGTERM SIGINT
fi

echo "[start.sh] exec shortr serve"
exec shortr serve
