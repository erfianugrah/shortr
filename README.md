# shortr

Self-hosted URL shortener. Single Go binary on Fly.io. Plain SQLite on a
Fly volume with daily snapshot retention. Astro 6 dashboard embedded in
the binary.

```
GET https://s.example.com/abc       → 302 to https://news.ycombinator.com/...
GET https://s.example.com/          → admin dashboard (bearer token required)
```

## What it does

- Creates short links with auto-generated 8-char slugs or custom slugs.
- Optional expiry (timestamp), max click count, password protection.
- Click analytics: timestamp, country (via Fly headers), referrer, UA, hashed IP.
- Single bearer token auth (`ADMIN_TOKEN`).
- Prometheus metrics at `/metrics`. JSON slog with request-ID correlation.
- Continuous backup of the SQLite file to S3-compatible storage via Litestream.

## Stack

| Layer | Choice |
|---|---|
| Runtime | Go 1.26+, single static binary, multi-stage Docker |
| HTTP | chi v5 + stdlib net/http |
| Storage | SQLite via modernc.org/sqlite (pure Go, no CGO) |
| Migrations | goose, SQL files embedded via `//go:embed` |
| Queries | sqlc-generated, type-safe |
| Backup | Fly volume daily snapshots (14-day retention) |
| Dashboard | Astro 6 + React (islands) + Tailwind 4 + shadcn/ui + zod 4 + tanstack-form |
| Lint/format | biome (web), go vet + staticcheck (Go) |
| Hosting | Fly.io single region, auto-stop machines |
| DNS | Knot DNS via knotctl (`s.<your-zone>` A/AAAA records) |
| TLS | Fly-managed Let's Encrypt |
| CI | GitHub Actions: lint → test → build → push ghcr.io → fly deploy on tag |

## Quick start (local)

```bash
# 1. clone + install deps
git clone https://github.com/erfianugrah/shortr ~/shortr
cd ~/shortr
make setup-web         # bun install in web/

# 2. dev loop
cp .env.example .env   # edit ADMIN_TOKEN, etc.
make migrate-up        # creates ./shortr.db
make dev               # runs Go server on :8080 + Astro dev on :4321
```

## Deploy to Fly

```bash
# one-time setup
flyctl apps create shortr-erfi
flyctl secrets set --app shortr-erfi \
  ADMIN_TOKEN="$(openssl rand -hex 32)" \
  PUBLIC_BASE_URL="https://s.erfi.io"

# DNS — using knotctl against the user's Knot DNS server
knotctl add s.erfi.io A    <ipv4>   # paste from `flyctl ips list -a shortr-erfi`
knotctl add s.erfi.io AAAA <ipv6>   # ditto

# cert
flyctl certs add s.erfi.io --app shortr-erfi
# wait ~5 min for issuance, then:
flyctl certs check s.erfi.io --app shortr-erfi

# deploy
git tag v0.1.0 && git push --tags    # CI deploys on tag
# OR manual:
flyctl deploy --config deploy/fly.toml --remote-only
```

### Disaster recovery

Fly takes a daily snapshot of the `/data` volume; retention is set to 14
days in `deploy/fly.toml`. Take a manual snapshot before risky changes:

```bash
make backup        # snapshot the prod volume
make snapshots     # list snapshots
# restore: flyctl volumes create --snapshot-id <id> --region fra shortr_data
```

## Development

```bash
make dev               # local dev (Go + Astro)
make test              # go test ./...
make lint              # biome check + go vet
make sqlc              # regenerate Go from migrations/*.sql
make migrate-up        # apply migrations to ./shortr.db
make migrate-down      # roll back one migration
make build             # web-build → go build → bin/shortr
```

## Project structure

See `AGENTS.md` for the bounded-context architecture and the rules around
the redirect hot path.

## Why these choices?

- **Single Fly region**, not multi-region: a URL shortener's traffic is
  one HTTP click per user-action. ~80-280ms worst-case latency from a
  single well-placed region is fine. Multi-region adds primary election +
  fly-replay middleware + per-region volumes for ~150ms savings most users
  never notice.
- **Plain SQLite + volume snapshots**, not Litestream: Fly's daily
  snapshots are free, built-in, and recover anything you'd care about for
  a personal shortener. Litestream would lower RPO from 24h to ~1s but
  costs a sidecar process + S3 secrets. Add it later if RPO becomes a real
  requirement.
- **Pure-Go SQLite (modernc.org/sqlite)**, not CGO: simpler Dockerfile,
  faster builds on Fly, cross-compiles cleanly. Perf is fine for this load.
- **Embedded Astro frontend**, not separate Pages app: one binary, one TLS
  cert, one deploy step. The dashboard is small enough that islands +
  embedded `dist/` is the simplest thing that works.

## License

MIT — see [LICENSE](./LICENSE).
