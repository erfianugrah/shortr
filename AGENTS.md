# shortr — agent guide

A self-hosted URL shortener. Single Go binary on Fly.io with embedded Astro
dashboard, plain SQLite on a Fly volume. Disaster recovery via Fly's
built-in daily volume snapshots (no sidecars, no external object storage).

Live: <https://shortr-erfi.fly.dev>

This file targets pi / Claude / GPT subagents working in this repo. The
top-level skills it pairs with live in `~/.pi/agent/skills/`:

- `software-architecture` — bounded contexts + interface-driven cross-context
  deps + slog/Prometheus observability + the flat single-binary pattern.
- `frontend-stack` — Astro 6 + biome + shadcn/ui + Tailwind 4 + zod 4 +
  tanstack-form defaults applied in `web/`.
- `design-utilitarian` — McMaster-Carr-style dense table UI ethos. No card
  grids, no animation tax, no marketing prose.
- `fly` — flyctl lifecycle, secrets-from-vault discipline, cert + custom DNS.
- `knot-dns` / `knotctl` — managing DNS on the user's Knot authoritative
  (`knot-fly-mvp`) — see the **Public hostname** section for the structural
  reason this app does NOT use a custom domain.
- `infrastructure-stack` — for the OpenTofu module that codifies the Fly app
  if you decide to IaC it later.

## Bounded contexts

Each context is its own package under `internal/`, owns its data model, and
exposes an interface to other contexts. Cross-context calls go through the
interface — no direct struct sharing.

| Context | Owns | Public methods |
|---|---|---|
| `internal/shortener` | Slug generation, link CRUD, expiry, password hash. The "links" table is its territory. | `Lookup`, `VerifyPassword`, `Create`, `Get`, `List`, `Update`, `Delete`, `IncrementClickCount` |
| `internal/analytics` | Click event capture + aggregation. The "clicks" table. Reads `links` only via the shortener interface. | `Record` (fire-and-forget), `List`, `Count`, `ByDay` (per-day buckets for the dashboard sparkline) |
| `internal/identity` | Bearer-token auth (single `ADMIN_TOKEN`). Easy to swap for OAuth later without touching api/ handlers. | `Verify` |
| `internal/storage` | SQLite connection pool + `sqlc`-generated query layer in `sqlitegen/`. Used by the contexts via their own thin repo interfaces. | `Open`, `MigrateUp/Down/Status`, `NewLinksRepo`, `NewClicksRepo` |
| `internal/api` | HTTP handlers + middleware (request ID, slog ctx, panic recovery, auth). Composes the bounded contexts via interfaces. | `New(Deps)`, `Routes() http.Handler` |
| `internal/obs` | Prometheus registry + slog setup. | `NewLogger`, `NewMetrics` |
| `internal/config` | Env-var parsing into a struct, validated at startup. | `Load() (Config, error)` |

## SQL: sqlc-generated, single source of truth

Queries live in `internal/storage/queries/*.sql`. `make sqlc` regenerates
`internal/storage/sqlitegen/` (the typed `Queries` struct + per-query
parameter types). The hand-written repos in `internal/storage/{links,clicks}_repo.go`
are thin translation layers between `sqlitegen.Link` / `sqlitegen.Click`
(column-shaped) and the bounded-context value types (`shortener.Link`,
`analytics.Click`, `analytics.DayBucket`).

When you change SQL:

1. Edit `internal/storage/queries/<context>.sql`.
2. Add a goose migration in `internal/storage/migrations/` if the schema is changing.
3. `make sqlc` to regenerate.
4. Update the repo's translation if the column shape changed.
5. `go test ./...` — race detector on.

## Hot path discipline

`GET /<slug>` is the only latency-sensitive path. Rules for it:

1. One indexed SQLite query against `links` (`storage.Queries.GetLink`).
   No joins, no N+1.
2. Click event recording is **fire-and-forget** via a buffered channel
   consumed by a single writer goroutine — never block the redirect on it.
3. No middleware allocation surprises. Stack stays: request-id → real-ip →
   recoverer → slog → metrics → handler. No auth on this path.
4. Password gate, if set, runs after the lookup. `?password=...` query
   string, bcrypt compare.

## The redirect path is public; everything else needs auth

Anything under `/api/` requires `Authorization: Bearer <ADMIN_TOKEN>`. The
redirect path `/<slug>` and `/healthz`, `/metrics`, and `/api/health` are
unauthenticated. Static dashboard assets under `/_astro/*` are public; the
dashboard's API calls carry the bearer token (entered once into a
localStorage value via the `/login` page).

## Data flow

```
GET /:slug   ──► api.RedirectHandler ──► shortener.Lookup(slug, now)
                                            │
                                            └► storage.Queries.GetLink (1 indexed read)
                                            │
                                            └► analytics.Record(event) [fire-and-forget]
                                                  │
                                                  └► buffered chan → writer goroutine
                                                                       │
                                                                       └► storage.Queries.InsertClick
                                                                       └► storage.Queries.IncrementClickCount

POST /api/links   ──► requireAuth ──► api.CreateLinkHandler ──► shortener.Create(req)
                                                                       │
                                                                       └► storage.Queries.InsertLink

GET /api/links/:slug/clicks  ──► requireAuth ──► api.ListClicksHandler ──► analytics.List + .Count + .ByDay
```

## Adding a new field to `links` (worked example)

1. New goose migration in `internal/storage/migrations/`. Numbered, descriptive name.
2. Update `internal/storage/queries/links.sql` to reference the new column
   in INSERT / UPDATE / SELECT.
3. `make sqlc` — regenerates `sqlitegen.Link` + the typed params.
4. Update `internal/shortener/types.go` if the field is user-visible.
5. Update `internal/storage/links_repo.go` translation (`rowToLink`,
   `InsertLink`, `UpdateLink`).
6. Wire it through the API handler in `internal/api/links.go` + the DTO.
7. Update the zod schema in `web/src/lib/schemas.ts` (must mirror the Go DTO).
8. Update the create form (`CreateLinkForm.tsx`) and edit sheet
   (`EditLinkSheet.tsx`) + the table column in `LinksTable.tsx`.

The zod schema and the Go DTO are the contract. Tests in
`internal/shortener/*_test.go` verify the Go side; the dashboard relies on
zod at runtime to surface mismatches loudly.

## Local dev loop

```bash
make dev               # runs go run . alongside `bun run dev` in web/ (proxy on :4321)
make test              # go test -race -count=1 ./...
make lint              # biome check (web) + go vet
make sqlc              # regenerate internal/storage/sqlitegen/ after queries/*.sql changes
make migrate-up        # apply pending migrations to ./shortr.db
make web-build         # bun run build → web/dist
make build             # web-build then go build -o bin/shortr .
```

## Deploy loop

```bash
make image             # builds + tags ghcr.io/erfianugrah/shortr:<git-sha>
make image-push        # push to ghcr.io
make deploy-remote     # flyctl deploy --remote-only (Fly builds + ships)
```

CI deploys automatically on tag push to `v*` — see `.github/workflows/deploy.yml`.
Cut a release with:

```bash
git tag -a v0.X.Y -m "release notes"
git push --tags
```

CI uses the `FLY_API_TOKEN` GH secret (deploy-scoped, 1y expiry).

## Public hostname — shortr-erfi.fly.dev, not s.erfi.io

The live URL is `https://shortr-erfi.fly.dev` (Fly-managed cert, no custom
domain). We tried `s.erfi.io` and ran into a structural Fly limitation:

- `erfi.io` is served by Knot on Fly anycast (`knot-fly-mvp`, `169.155.56.21`).
- Fly's internal resolver hairpins when it tries to chase `ns1.erfi.io` /
  `ns2.erfi.io` (also `169.155.56.21`) — anycast loops the query back to itself.
- Result: Fly's cert verifier sees `dns_records.{a,aaaa,cname,ownership_txt}=[]`
  with `soa="a0.nic.io"` (the .io TLD, not erfi.io's actual SOA). Verifier
  is structurally unable to resolve ANYTHING under erfi.io.
- Neither A+AAAA records, nor a CNAME into `.fly.dev`, nor `_fly-ownership`
  TXT, nor `_acme-challenge` CNAME unblocks this. Fly's resolver can't read
  any of them.

`ntfy.erfi.io` (also on Fly) is in the `Issued` state — but that cert was
issued before the Knot-on-Fly DNS migration. New certs for erfi.io subdomains
are blocked until the structural issue is fixed.

### The two real fixes (deferred)

1. **Off-Fly secondary NS for erfi.io** (the proper fix). Sign up for
   Hurricane Electric's free DNS secondary (https://dns.he.net), point its
   AXFR-in at `knot-fly-mvp`'s public IP, add `ns3.he.net` to the erfi.io
   delegation at the Namecheap registrar. Fly's resolver then falls back
   to he.net's anycast NSes (not on Fly fabric) and can resolve normally.
   Unblocks `s.erfi.io`, `gloryhole.erfi.io`, and any future erfi.io subdomain.
2. **Use a zone Fly's resolver can reach.** `erfi.dev` is delegated to CF's
   nameservers, which work fine. `s.erfi.dev` would issue today.

Neither is needed for shortr to function. Park decision: stay on
`shortr-erfi.fly.dev` until the off-Fly secondary lands as part of the
`knot-dns` skill backlog.

### When you change the URL

Update three places:
- `PUBLIC_BASE_URL` secret: `flyctl secrets set --app shortr-erfi PUBLIC_BASE_URL=https://<new>`
- This `AGENTS.md` block + `README.md` + `docs/operations.md`
- The local `.env.shortr-erfi.local` (operator copy of the token + URL)

## Disaster recovery

DR is Fly's built-in volume snapshots. Configured in `fly.toml`:
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
- **Don't bypass sqlc.** Raw `db.Exec` / `db.Query` in the repos defeats
  the type-safety win. If you need a query sqlc can't express, add it as a
  separate method on the repo and document why.
- **Don't change author/committer identity in commits.** `~/.gitconfig` is
  authoritative. No `-c user.name=`, no `--author=`, no `GIT_AUTHOR_*`.
- **Don't expose secrets in commits.** `.env` and `.env.*.local` are
  gitignored. `ADMIN_TOKEN` lives in `flyctl secrets`.

## Reference layout

```
shortr/
├── main.go                      # CLI dispatch (serve | migrate | version)
├── static.go                    # //go:embed all:web/dist
├── fly.toml                     # Fly config (at root, per Fly convention)
├── go.mod / go.sum
├── sqlc.yaml                    # query → Go codegen config
├── biome.json                   # web lint/format config
├── Makefile                     # dev / build / migrate / deploy / backup
├── LICENSE                      # MIT
├── README.md                    # user-facing
├── AGENTS.md                    # this file
├── CHANGELOG.md                 # versioned release notes
├── TODO.md                      # optional-future-work backlog
├── docs/
│   └── operations.md            # day-2 ops + DR + DNS + metrics
├── deploy/
│   └── Dockerfile               # multi-stage: bun → go → debian
├── scripts/
│   └── dev.sh                   # Astro dev :4321 + Go server :8080 concurrently
├── internal/
│   ├── api/                     # HTTP layer (chi)
│   │   ├── api.go               # router composition
│   │   ├── middleware.go        # request-id, slog, metrics, auth
│   │   ├── links.go             # /api/links CRUD + /clicks
│   │   ├── redirect.go          # /<slug> hot path
│   │   ├── health.go            # /healthz, /api/health, /api/me
│   │   ├── errors.go            # sentinel → HTTP status mapping
│   │   └── static.go            # dashboard SPA shell
│   ├── shortener/               # link CRUD bounded context
│   ├── analytics/               # click capture + DayBucket aggregation
│   ├── identity/                # bearer-token auth
│   ├── storage/
│   │   ├── storage.go           # connection pool + goose migrations
│   │   ├── links_repo.go        # shortener.Repo (wraps sqlitegen)
│   │   ├── clicks_repo.go       # analytics.Repo (wraps sqlitegen)
│   │   ├── migrations/          # goose SQL files (embedded)
│   │   ├── queries/             # sqlc input SQL
│   │   └── sqlitegen/           # generated; DO NOT EDIT BY HAND
│   ├── obs/                     # slog + Prometheus
│   └── config/                  # env parsing
├── web/                         # Astro 6 + React 19 islands
│   ├── src/
│   │   ├── pages/               # /, /login
│   │   ├── layouts/             # Main.astro (wraps + mounts Toaster)
│   │   ├── components/          # LinksTable, CreateLinkForm, EditLinkSheet,
│   │   │                        # ClicksDrawer, LoginForm, Toaster
│   │   ├── components/ui/       # shadcn-ui v4 primitives (generated)
│   │   ├── lib/                 # api.ts, schemas.ts, utils.ts
│   │   └── styles/global.css    # Tailwind v4 + design tokens
│   ├── public/favicon.svg
│   ├── astro.config.mjs
│   ├── components.json          # shadcn CLI config
│   ├── tsconfig.json
│   └── package.json
└── .github/workflows/
    ├── ci.yml                   # PR + main: go test + bun build + biome
    └── deploy.yml               # tag v*: flyctl deploy --remote-only
```
