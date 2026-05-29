# shortr — operations

Live URL: <https://s.erfi.io>
Fly internal: <https://shortr-erfi.fly.dev>
GitHub: <https://github.com/erfianugrah/shortr>

## Deployment topology

| Resource | Value |
|---|---|
| Fly app | `shortr-erfi` |
| Region | `fra` (single) |
| Machine size | `shared-cpu-1x` / 256 MB |
| Volume | `shortr_data` (1 GB initial, 14-day snapshot retention) |
| IPv4 (shared) | `66.241.124.26` |
| IPv6 (dedicated) | `2a09:8280:1::11c:f064:0` |
| TLS | Fly-managed Let's Encrypt for `s.erfi.io` |
| DNS | Knot DNS authoritative (`knot-fly-mvp` on Fly) — managed via `knotctl` |

## Day-2 operations

### Tail logs

```bash
flyctl logs --app shortr-erfi
flyctl logs --app shortr-erfi --machine <id>
flyctl logs --app shortr-erfi --no-tail --json | jq 'select(.message | test("ERROR|WARN"))'
```

### Status + restart

```bash
flyctl status --app shortr-erfi
flyctl machine list --app shortr-erfi
flyctl machine restart <machine-id> --app shortr-erfi
```

### SSH in

```bash
flyctl ssh console --app shortr-erfi
# one-shot:
flyctl ssh console --app shortr-erfi -C 'ls -la /data'
flyctl ssh console --app shortr-erfi -C 'sqlite3 /data/shortr.db "SELECT COUNT(*) FROM links;"'
```

### Volume + snapshots

```bash
flyctl volumes list --app shortr-erfi
make snapshots                         # convenience: list snapshots for the only volume
make backup                            # convenience: take a snapshot now

# Restore (creates a new volume from snapshot):
flyctl volumes snapshots list <vol-id> --app shortr-erfi
flyctl volumes create --snapshot-id <snap-id> --region fra shortr_data --app shortr-erfi
# Then update [[mounts]] source = "shortr_data" if name differs, and redeploy.
```

### Secret rotation

```bash
# rotate the bearer token (rolls the machine)
NEW=$(openssl rand -hex 32)
flyctl secrets set --app shortr-erfi ADMIN_TOKEN="$NEW"
# then paste $NEW into the dashboard /login page

# audit currently-set secrets
flyctl secrets list --app shortr-erfi
```

### Deploy a new version

Tag-driven CI (preferred):

```bash
git tag v0.2.0 && git push --tags
# GH Actions deploys via the shared FLY_API_TOKEN repo secret
```

Manual:

```bash
flyctl deploy --remote-only --app shortr-erfi
```

### Metrics

`/metrics` exposes Prometheus output. Fly's managed Prometheus scrapes it; query in the Fly dashboard or via Grafana data source. Custom metrics defined in `internal/obs/obs.go`:

- `shortr_http_requests_total{route,method,status}`
- `shortr_http_request_duration_seconds{route,method}`
- `shortr_redirects_total{outcome}` — `hit | not_found | expired | exhausted`
- `shortr_clicks_recorded_total` / `shortr_clicks_dropped_total`
- `shortr_links_created_total` / `shortr_links_deleted_total`

### DNS changes

Records live on the user's Knot DNS at `knot-fly-mvp`. `knotctl` from `~/knot-fly/`:

```bash
# show current
~/knot-fly/knotctl ls s.erfi.io
~/knot-fly/knotctl ls _acme-challenge.s.erfi.io

# repoint to a different Fly app (CNAME pattern — see AGENTS.md for why)
~/knot-fly/knotctl set s.erfi.io CNAME <new-app>.fly.dev.
```

**Never** use A/AAAA records for erfi.io subdomains backed by Fly apps. The CNAME is load-bearing — Fly's cert verifier can't resolve our anycast Knot, so we route validation through the `.fly.dev` zone. Full explanation in `AGENTS.md`.

If you ever migrate the Fly app (e.g. rename), `knotctl set s.erfi.io CNAME <new-app>.fly.dev.` then tear down the old app once cache TTL (300s here) expires.

## DR drills

Quarterly: verify you can actually restore.

```bash
# 1. take a snapshot
make backup

# 2. note the snapshot ID
make snapshots | head -3

# 3. fork it into a throwaway volume + spin up an isolated machine
flyctl volumes create --snapshot-id <id> --region fra shortr_data_drill --app shortr-erfi
flyctl machine clone <machine-id> --region fra --app shortr-erfi
# attach the cloned machine to the drill volume, verify counts match,
# then destroy:
flyctl machine destroy <drill-machine-id> --force --app shortr-erfi
flyctl volumes destroy <drill-vol-id> --app shortr-erfi
```
