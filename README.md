# shortr

Self-hosted URL shortener. Single Go binary on Fly.io. SQLite + Litestream
for storage. Astro 6 dashboard embedded in the binary.

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
| Backup | Litestream sidecar → Tigris (Fly's S3) |
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

See `docs/deploy.md` (TODO; for now follow these steps):

```bash
# one-time setup
flyctl apps create shortr
flyctl storage create shortr-litestream     # creates a Tigris bucket + sets BUCKET_*, AWS_* secrets
flyctl secrets set --app shortr \
  ADMIN_TOKEN="$(openssl rand -hex 32)" \
  PUBLIC_BASE_URL="https://s.<your-zone>"

# DNS — using knotctl from ~/.pi/agent/skills/knotctl/
knotctl add s.<your-zone> A    66.241.124.X   # paste from `flyctl ips list`
knotctl add s.<your-zone> AAAA 2a09:8280:1::Y # ditto

# cert
flyctl certs add s.<your-zone> --app shortr
# wait for cert to issue (~5 min), then:
flyctl certs check s.<your-zone> --app shortr

# deploy
git tag v0.1.0 && git push --tags    # CI deploys on tag
# OR manual:
flyctl deploy --remote-only --app shortr
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
- **Plain SQLite + Litestream**, not LiteFS: Litestream is mature and
  actively developed. LiteFS is in maintenance mode (last release Apr 2025,
  status: beta). If global reads become a measured need, the migration is
  a Dockerfile change.
- **Pure-Go SQLite (modernc.org/sqlite)**, not CGO: simpler Dockerfile,
  faster builds on Fly, cross-compiles cleanly. Perf is fine for this load.
- **Embedded Astro frontend**, not separate Pages app: one binary, one TLS
  cert, one deploy step. The dashboard is small enough that islands +
  embedded `dist/` is the simplest thing that works.

## License

MIT — see [LICENSE](./LICENSE).
