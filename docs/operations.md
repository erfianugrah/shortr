# shortr — operations

Live URL: <https://shortr-erfi.fly.dev>
GitHub: <https://github.com/erfianugrah/shortr>
Releases: see [CHANGELOG.md](../CHANGELOG.md)

> Custom-domain branding (`s.erfi.io` or similar) is blocked by a structural
> Fly+Knot DNS issue — see `AGENTS.md` "Public hostname" section.

## Deployment topology

| Resource | Value |
|---|---|
| Fly app | `shortr-erfi` |
| Region | `fra` (single) |
| Machine size | `shared-cpu-1x` / 256 MB |
| Volume | `shortr_data` (1 GB initial, 14-day snapshot retention) |
| IPv4 (shared) | `66.241.124.26` |
| IPv6 (dedicated) | `2a09:8280:1::11c:f064:0` |
| TLS | Fly-managed Let's Encrypt for `shortr-erfi.fly.dev` (automatic) |
| DNS | n/a — using the `.fly.dev` hostname directly |
| Image registry | `registry.fly.io/shortr-erfi:deployment-*` (auto-built on tag) |
| GitHub Actions secret | `FLY_API_TOKEN` (deploy-scoped, 1y expiry) |
| Fly secrets | `ADMIN_TOKEN`, `IP_HASH_SALT`, `PUBLIC_BASE_URL` |

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
git tag -a v0.X.Y -m "release notes"
git push --tags
# GH Actions deploys via the shared FLY_API_TOKEN repo secret
```

Manual:

```bash
flyctl deploy --remote-only --app shortr-erfi
```

The deploy workflow:
1. Checkout main
2. Set up flyctl
3. `flyctl deploy --remote-only` — Fly's builder runs the multi-stage
   Dockerfile (bun + go) and ships the image to the app.
4. Machine rolls in place; auto-rollback if the healthcheck fails.

### Metrics

`/metrics` exposes Prometheus output. Fly's managed Prometheus scrapes it;
query in the Fly dashboard or via Grafana data source. Custom metrics
defined in `internal/obs/obs.go`:

| Metric | Labels | Notes |
|---|---|---|
| `shortr_http_requests_total` | `route, method, status` | One per request post-handler |
| `shortr_http_request_duration_seconds` | `route, method` | Histogram, default buckets |
| `shortr_redirects_total` | `outcome` ∈ `hit / not_found / expired / exhausted` | Counts redirect outcomes |
| `shortr_redirect_lookup_errors_total` | — | Storage errors on hot path |
| `shortr_clicks_recorded_total` | — | Click events actually written |
| `shortr_clicks_dropped_total` | — | Click events dropped on full buffer (set `CLICK_BUFFER` higher if non-zero) |
| `shortr_links_created_total` | — | |
| `shortr_links_deleted_total` | — | |

Plus the default Go runtime + process collectors (`go_*`, `process_*`).

Quick health probe:

```bash
curl -s https://shortr-erfi.fly.dev/api/health | jq .
# {"app":"shortr-erfi","machine_id":"...","ok":true,"region":"fra"}

curl -s https://shortr-erfi.fly.dev/metrics | rg -oE '^shortr_\w+' | sort -u
```

### DNS changes

None needed currently — the live URL is the `.fly.dev` hostname, which Fly
manages automatically.

When the off-Fly secondary NS is added to erfi.io (see `AGENTS.md` for why),
the path to claim `s.erfi.io` will be:

```bash
~/knot-fly/knotctl add s.erfi.io CNAME shortr-erfi.fly.dev.
flyctl certs add s.erfi.io --app shortr-erfi
# wait ~5 min, then verify:
flyctl certs check s.erfi.io --app shortr-erfi
# update PUBLIC_BASE_URL:
flyctl secrets set --app shortr-erfi PUBLIC_BASE_URL=https://s.erfi.io
```

A records for erfi.io subdomains backed by Fly apps will not work as long
as Fly's resolver can't reach our anycast Knot. The CNAME pattern is
required because it routes Fly's resolver into its own `.fly.dev` zone
(which it can resolve).

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

## Token / config recovery

The local operator copy of the live secrets lives at
`/home/erfi/shortr/.env.shortr-erfi.local` (chmod 600, gitignored).

If lost, rotate:

```bash
NEW=$(openssl rand -hex 32)
flyctl secrets set --app shortr-erfi ADMIN_TOKEN="$NEW"
echo "ADMIN_TOKEN=$NEW" > .env.shortr-erfi.local
echo "PUBLIC_BASE_URL=https://shortr-erfi.fly.dev" >> .env.shortr-erfi.local
chmod 600 .env.shortr-erfi.local
```

Paste the new value into the dashboard `/login` page.
