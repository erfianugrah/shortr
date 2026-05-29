# TODO

Optional backlog. Nothing here blocks daily use of shortr.

## Pickable any time

### UX

- **Sortable / filterable columns** in the links table. Currently chronological
  only. Add column-header click → toggle sort; add a top-of-table filter
  input that narrows by slug / target / note substring. `LinksTable.tsx`.
- **Bulk import / CSV export.** Useful for migrating from another service or
  taking a snapshot of just the link table (separate from the volume backup).
  New `/api/links/export?format=csv` + `/api/links/import` endpoints, plus a
  pair of buttons in the dashboard header.
- **Dark mode.** `sonner` already accepts a `theme` prop. Add a toggle in
  `Main.astro`, persist to localStorage, swap the body class. The shadcn
  globals.css already has a `.dark` variant scaffolded but commented out.
- **Real homepage at `/`** — currently `/` is the dashboard, `/login` is the
  token form. A public landing page (or a simple link form for the
  authenticated user) might be nicer.
- **Per-link QR code** in the edit sheet. `qrcode` npm package, render to
  SVG inline. ~20 LOC.
- **Slug suggestions / collision avoidance** in `CreateLinkForm`: try the
  custom slug, on 409 conflict pre-fill an auto-generated one as a
  suggestion. Cheaper than two round-trips.

### Backend

- **Multi-user OAuth** instead of single bearer token. The `identity`
  bounded context is designed for the swap — replace `BearerVerifier` with
  a `GitHubVerifier` (or magic-link / generic OIDC). Add a `principals`
  table, scope link ownership.
- **Per-API-key rate limiting** if the bearer token ever leaks publicly.
  Token-bucket on the `requireAuth` middleware; back the bucket with the
  link-create / link-update endpoints. Drop into `internal/api/middleware.go`.
- **Click event Prometheus exemplars / histograms** for redirect latency.
  Currently the histogram covers HTTP-overall; useful to break the redirect
  hot path out separately so you can SLO it.
- **`/api/links/export?format=jsonl`** — full table dump for cold storage
  or rehydration. Streaming, not buffered.
- **Idempotency keys on `POST /api/links`** — `Idempotency-Key` header,
  short-lived bbolt cache, return the same response if a duplicate POST
  comes in. Useful if a flaky network double-submits a create.

### Storage

- **Composite indexes** on `(created_by, created_at DESC)` if multi-user
  lands. Currently only `created_at` is indexed.
- **Click event retention policy.** `clicks` table grows unbounded. Add a
  `make prune-clicks` target that drops events older than N days, plus an
  optional background goroutine that does the same on a timer (off by
  default, opt in via config).

### Ops / infra

- **Off-Fly secondary NS for `erfi.io`.** Sign up for he.net free DNS
  secondary, point AXFR-in at `knot-fly-mvp`, add `ns3.he.net` to the
  Namecheap delegation. Unblocks Fly certs for `s.erfi.io` (and other
  erfi.io subdomains on Fly that are currently stuck — see
  `gloryhole.erfi.io`). Fully documented in the `knot-dns` skill's
  backlog. ~30 min once started.
- **OpenTofu module** that codifies the Fly app, volume, secrets binding,
  and a Tigris bucket (if Litestream comes back later). Lives under
  `infra/` alongside any other Fly-managed things.
- **Branch protection on `main`** — require CI to pass before merge.
  Currently solo development pushes directly to main; this is fine but
  would matter if anyone else ever contributes.
- **Vault / SOPS for the local `.env.shortr-erfi.local`** — currently
  chmod 600, gitignored. Encrypting via SOPS-age would make it safe to
  check into a private repo if you want a portable operator copy.

### Telemetry

- **OpenTelemetry traces** through the redirect hot path. Span: incoming
  HTTP → `shortener.Lookup` → `analytics.Record`. Export via OTLP to Fly's
  managed observability or a self-hosted collector. The `obs` package has
  the right shape to drop this in.
- **Real Grafana dashboard JSON** committed under `docs/grafana/` so the
  prom metrics are usable out of the box.

## Hard NO (already debated and discarded)

- **Multi-region with LiteFS.** LiteFS is in maintenance mode (last release
  Apr 2025, status: beta). Single region + Fly's `.fly.dev` anycast is fine
  for this traffic shape. If global redirect latency ever becomes a real
  measured problem, the migration is a Dockerfile change, not a refactor.
- **Litestream / Tigris.** Tried briefly during the scaffold; tore it out.
  Fly's daily volume snapshots are the right DR story for personal scale.
  Litestream costs a sidecar process + S3 secrets for an RPO improvement
  the user doesn't need.
- **CGO SQLite.** `modernc.org/sqlite` is the deliberate choice — pure-Go,
  no gcc in the build image, cross-compiles cleanly. The perf delta vs
  `mattn/go-sqlite3` is negligible at read-heavy / microsecond queries.
- **ORM.** sqlc + thin repo wrappers is the chosen path. No GORM, no Ent,
  no Bun. SQL is the language; reflection-based ORMs leak.

## House rules

When you pick something up:

- Open a branch only if the change is big; otherwise commit straight to
  `main` (solo repo).
- TDD where it pays — i.e. for bug fixes (red test reproduces the bug)
  and non-trivial business logic. Scaffolding and UI tweaks are fine
  without tests.
- `make lint && make test && make build` before pushing.
- Tag a release (`v0.X.Y`) when the change is user-visible; CI deploys.
- Update `CHANGELOG.md` in the same commit as the feature, not after.
- If you remove an item from this list, delete it; don't strike it.
