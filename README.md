# shortr

Self-hosted URL shortener. Single Go binary on Fly.io. Plain SQLite on a
Fly volume with daily snapshot retention. Astro 6 dashboard embedded in
the binary.

Live: <https://shortr-erfi.fly.dev>

```
GET https://shortr-erfi.fly.dev/abc   → 302 to https://news.ycombinator.com/...
GET https://shortr-erfi.fly.dev/      → admin dashboard (bearer token required)
```

> Why not `s.erfi.io`? Fly's internal resolver can't reach erfi.io's
> Knot-on-Fly-anycast nameservers (hairpin loop). New certs for any
> erfi.io subdomain are blocked until an off-Fly secondary NS is added.
> See `AGENTS.md` — "Public hostname" section.

## What it does

- **Create** short links with auto-generated 8-char slugs or custom slugs.
- **Expiry**: optional `expires_at` timestamp — link returns 410 Gone after.
- **Click caps**: optional `max_clicks` — link returns 410 Gone when exhausted.
- **Password protection**: bcrypt hash; redirect path takes `?password=...`.
- **Analytics**: per-link click count, recent events (country / Fly region /
  referrer / UA / hashed IP), daily aggregation for sparkline.
- **Admin dashboard**: dense table with copy / view-clicks / edit / delete,
  inline create form with optional fields, right-side edit sheet, click
  drawer with inline SVG sparkline.
- **Single bearer token auth** (`ADMIN_TOKEN`). Easy to swap for OAuth.
- **Prometheus metrics** at `/metrics`. JSON `slog` with request-ID correlation.

## Stack

| Layer | Choice |
|---|---|
| Runtime | Go 1.25+, single static binary, multi-stage Docker |
| HTTP | chi v5 + stdlib net/http |
| Storage | SQLite via modernc.org/sqlite (pure Go, no CGO) |
| Migrations | goose, SQL files embedded via `//go:embed` |
| Queries | **sqlc** — `internal/storage/queries/*.sql` → `sqlitegen/` |
| Backup | Fly volume daily snapshots (14-day retention) |
| Dashboard | Astro 6 + React 19 (islands) + Tailwind 4 + shadcn-ui v4 + zod 4 |
| UI primitives | button, input, label, table, dialog, sheet, sonner, badge, separator, dropdown-menu, select, form |
| Lint/format | biome (web), go vet + race tests (Go) |
| Hosting | Fly.io single region (`fra`), auto-stop machines |
| TLS | Fly-managed Let's Encrypt for `shortr-erfi.fly.dev` (automatic) |
| CI | GitHub Actions: lint → test → build on PRs; `flyctl deploy` on tag |

## Quick start (local)

```bash
git clone https://github.com/erfianugrah/shortr ~/shortr
cd ~/shortr
make setup-web         # bun install in web/

cp .env.example .env
# edit .env: ADMIN_TOKEN=$(openssl rand -hex 32)

make migrate-up        # create ./shortr.db
make dev               # Go server on :8080 + Astro dev on :4321
```

The dev script proxies `/api/*` from Astro :4321 to Go :8080 so the dashboard
hot-reloads while the API stays consistent.

## Deploy to Fly

```bash
# one-time setup
flyctl apps create shortr-erfi
flyctl secrets set --app shortr-erfi \
  ADMIN_TOKEN="$(openssl rand -hex 32)" \
  PUBLIC_BASE_URL="https://shortr-erfi.fly.dev" \
  IP_HASH_SALT="$(openssl rand -hex 16)"

# deploy
git tag -a v0.2.0 -m "release notes"
git push --tags         # CI deploys on tag
# OR manual:
flyctl deploy --remote-only
```

### Disaster recovery

Fly takes a daily snapshot of the `/data` volume; retention is 14 days in
`fly.toml`. Take a manual snapshot before risky changes:

```bash
make backup            # snapshot the prod volume
make snapshots         # list snapshots
# restore: flyctl volumes create --snapshot-id <id> --region fra shortr_data
```

## Development

```bash
make dev               # local dev (Go + Astro)
make test              # go test -race -count=1 ./...
make lint              # biome check + go vet
make sqlc              # regenerate Go from internal/storage/queries/*.sql
make migrate-up        # apply migrations to ./shortr.db
make migrate-down      # roll back one migration
make build             # web-build → go build → bin/shortr
```

## Project structure

See [`AGENTS.md`](./AGENTS.md) for the bounded-context architecture, the
rules around the redirect hot path, the sqlc workflow, and the worked
"add a new field" example.

## Why these choices?

- **Single Fly region**, not multi-region: a URL shortener's traffic is
  one HTTP click per user-action. ~80–280 ms worst-case latency from a
  single well-placed region is fine. Multi-region adds primary election +
  fly-replay middleware + per-region volumes for ~150 ms savings most users
  never notice.
- **Plain SQLite + volume snapshots**, not Litestream: Fly's daily
  snapshots are free, built-in, and recover anything you'd care about for
  a personal shortener. Litestream would lower RPO from 24h to ~1s but
  costs a sidecar process + S3 secrets. Add it later if RPO becomes a real
  requirement.
- **Pure-Go SQLite (modernc.org/sqlite)**, not CGO: simpler Dockerfile,
  faster builds on Fly, cross-compiles cleanly. Perf is fine for this load.
- **sqlc-generated queries**, not an ORM: SQL files are the single source
  of truth; Go calls are typed and compile-checked. No reflection, no
  runtime overhead, no leaky abstraction.
- **Embedded Astro frontend**, not separate Pages app: one binary, one TLS
  cert, one deploy step. The dashboard is small enough that islands +
  embedded `dist/` is the simplest thing that works.

## Versioning

Releases are tagged `vMAJOR.MINOR.PATCH`. See [`CHANGELOG.md`](./CHANGELOG.md).

## Operations

See [docs/operations.md](./docs/operations.md) for the deployed-topology
reference, day-2 commands (logs, ssh, snapshots, secret rotation,
metrics, DNS changes) and the DR drill checklist.

## License

MIT — see [LICENSE](./LICENSE).
