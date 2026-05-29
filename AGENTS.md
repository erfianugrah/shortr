# shortr — agent guide

A self-hosted URL shortener. Single Go binary on Fly.io with embedded Astro
dashboard, plain SQLite on a Fly volume. Disaster recovery via Fly's
built-in daily volume snapshots (no sidecars, no external object storage).

This file targets pi / Claude / GPT subagents working in this repo. The
top-level skills it pairs with live in `~/.pi/agent/skills/`:

- `software-architecture` — bounded contexts + interface-driven cross-context
  deps + slog/Prometheus observability + the flat single-binary pattern.
- `frontend-stack` — Astro 6 + biome + shadcn/ui + Tailwind 4 + zod 4 +
  tanstack-form defaults applied in `web/`.
- `design-utilitarian` — McMaster-Carr-style dense table UI ethos. No card
  grids, no animation tax, no marketing prose.
- `fly` — flyctl lifecycle, secrets-from-vault discipline, cert + custom DNS.
- `knot-dns` / `knotctl` — managing the `s.<your-zone>` A/AAAA + ACME records
  on the user's Knot DNS authoritative server.
- `infrastructure-stack` — for the OpenTofu module that codifies the Fly app
  if you decide to IaC it later.

## Bounded contexts

Each context is its own package under `internal/`, owns its data model, and
exposes an interface to other contexts. Cross-context calls go through the
interface — no direct struct sharing.

| Context | Owns |
|---|---|
| `internal/shortener` | Slug generation, link CRUD, expiry, password hash. The "links" table is its territory. |
| `internal/analytics` | Click event capture + aggregation. The "clicks" table. Reads `links` only via the shortener interface. |
| `internal/identity` | Bearer-token auth (single `ADMIN_TOKEN`). Easy to swap for OAuth later without touching api/ handlers. |
| `internal/storage` | SQLite connection pool + sqlc-generated query layer. Used by the contexts via their own thin repo interfaces. |
| `internal/api` | HTTP handlers + middleware (request ID, slog ctx, panic recovery, auth, CORS). Composes the bounded contexts via interfaces. |
| `internal/obs` | Prometheus registry + slog setup. |
| `internal/config` | Env-var parsing into a struct, validated at startup. |

## Hot path discipline

`GET /<slug>` is the only latency-sensitive path. Rules for it:

1. One indexed SQLite query against `links`. No joins. No N+1.
2. Click event recording is **fire-and-forget** via a buffered channel
   consumed by a single writer goroutine — never block the redirect on it.
3. No middleware allocation surprises. Stack stays: request-id → slog → panic
   recovery → handler. No auth on this path.

## The redirect path is public; everything else needs auth

Anything under `/api/` requires `Authorization: Bearer <ADMIN_TOKEN>`. The
redirect path `/<slug>` and `/healthz` and `/metrics` are unauthenticated.
Static dashboard assets under `/_app/*` are public; the dashboard's API
calls carry the bearer token (entered once into a localStorage value via
the `/login` page).

## Data flow

```
GET /:slug  ──► api.RedirectHandler ──► shortener.Lookup(slug) ──► storage.Queries.GetLink
                                            │
                                            └──► analytics.Record(event) [fire-and-forget]
                                                       │
                                                       └──► batch writer goroutine
                                                                    │
                                                                    └──► storage.Queries.InsertClick + UpdateClickCount

POST /api/links ──► api.CreateLinkHandler ──► identity.Verify(bearer) ──► shortener.Create(req)
                                                                              │
                                                                              └──► storage.Queries.InsertLink
```

## Adding a new field to `links`

1. New goose migration in `migrations/`. Numbered, descriptive name.
2. Re-run `make sqlc` to regenerate `internal/storage/sqlitegen/`.
3. Update `internal/shortener/types.go` if it's user-visible.
4. Update zod schema in `web/src/lib/schemas.ts` (must mirror Go struct).
5. Wire it through the API handler.
6. Update the dashboard form / table column.

The zod schema and the Go struct are the contract. Tests in
`internal/shortener/*_test.go` verify the Go side; the dashboard relies on
zod at runtime to surface mismatches loudly.

## Local dev loop

```bash
make dev           # runs go run ./cmd/shortr alongside `bun run dev` in web/
make test          # go test ./...
make lint          # biome check (web) + go vet + staticcheck
make sqlc          # regenerate query bindings after migration changes
make migrate-up    # apply pending migrations to ./shortr.db
make web-build     # bun run build → web/dist
make build         # web-build then go build -o bin/shortr ./cmd/shortr
```

## Deploy loop

```bash
make image         # builds + tags ghcr.io/USER/shortr:<git-sha>
make image-push    # push to ghcr.io
flyctl deploy --image ghcr.io/USER/shortr:<git-sha>
```

CI does this on tag push to `v*` — see `.github/workflows/deploy.yml`.

## Disaster recovery

DR is Fly's built-in volume snapshots. Configured in `deploy/fly.toml`:
```
[[mounts]]
  source = "shortr_data"
  destination = "/data"
  snapshot_retention = 14   # daily snapshots, kept 14 days
```

Ops:
- `make backup` — manual snapshot before a risky deploy / migration.
- `make snapshots` — list available snapshots.
- `flyctl volumes create --snapshot-id <id> --region fra shortr_data` — restore.

If you ever need point-in-time recovery to the second, or cross-region
durability, that's when to add Litestream back. Not before.

## Things to NOT do

- **Don't add CGO.** modernc.org/sqlite is the deliberate choice — pure-Go,
  no gcc in build image, faster Fly deploys, cross-compiles cleanly. The
  perf delta vs mattn/go-sqlite3 is negligible at this workload (read-heavy,
  microsecond queries).
- **Don't add LiteFS yet.** It's beta + dev frozen. If the user later
  measures real demand for sub-50ms global redirects, migration is mostly a
  Dockerfile change + `internal/storage/` driver swap. Not a refactor.
- **Don't add Litestream back unless RPO matters.** Fly's daily volume
  snapshots are the disaster-recovery story for this app. Litestream would
  reduce RPO from 24h to ~1s, but adds a sidecar process, S3 secret
  rotation, and ongoing monitoring. Worth it for paying customers, not for
  a personal shortener where re-creating the last 24h of links is trivial.
- **Don't put click recording on the redirect critical path.** Always
  fire-and-forget. The buffered channel has a fixed capacity; on overflow
  drop events with a Prometheus counter incremented — never block.
- **Don't change author/committer identity in commits.** `~/.gitconfig` is
  authoritative. No `-c user.name=`, no `--author=`, no `GIT_AUTHOR_*`.
- **Don't expose secrets in commits.** `.env` is gitignored. `ADMIN_TOKEN`
  lives in `flyctl secrets`.

## Reference layout

```
shortr/
├── cmd/shortr/main.go           # wire-up, server lifecycle, signal handling
├── internal/
│   ├── api/                     # HTTP layer (chi)
│   ├── shortener/               # bounded context: link CRUD
│   ├── analytics/               # bounded context: click capture
│   ├── identity/                # bounded context: bearer auth
│   ├── storage/                 # SQLite + sqlc-generated queries
│   ├── obs/                     # slog + Prometheus
│   └── config/                  # env parsing
├── migrations/                  # goose SQL, embedded
├── web/                         # Astro 6 + React + Tailwind 4 + shadcn
├── static.go                    # //go:embed all:web/dist
├── deploy/
│   ├── fly.toml                 # single Fly region, volume + daily snapshots
│   └── Dockerfile               # multi-stage: bun → go → debian
├── biome.json
├── sqlc.yaml
├── Makefile
└── .github/workflows/deploy.yml
```
