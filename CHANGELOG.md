# Changelog

All notable changes to this project are documented here. Format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/); the project uses
[semantic versioning](https://semver.org/).

## [Unreleased]

Nothing yet. See [`TODO.md`](./TODO.md) for the optional backlog.

## [0.1.0] — 2026-05-29

First tagged release. Working self-hosted URL shortener on Fly.io with a
full admin dashboard.

### Added

#### Redirect + API

- `GET /<slug>` redirect with one indexed SQLite read on the hot path.
- Fire-and-forget click capture via buffered channel + background writer
  goroutine. Drops on overflow with a Prometheus counter; never blocks
  the redirect.
- Optional `expires_at`, `max_clicks`, and bcrypt-hashed `password`
  protection per link. Redirect returns `410 Gone` on expiry / exhaustion;
  `401` on missing or wrong password.
- `POST /api/links` to create (auto-slug or custom), `GET /api/links` to
  list, `GET /api/links/:slug` to fetch, `PATCH /api/links/:slug` to
  update (with `clear_expiry` / `clear_max_clicks` / empty-`password`
  flags to nullify fields), `DELETE /api/links/:slug` to remove.
- `GET /api/links/:slug/clicks?days=N&limit=M` returns recent events +
  per-day aggregation buckets + total count.
- Bearer-token auth (`ADMIN_TOKEN`) guards all `/api/*` writes; constant-
  time compare; `Verifier` interface designed for future OAuth swap.
- Public probes: `/healthz`, `/api/health`, `/metrics`.

#### Observability

- Structured `slog` JSON output with `X-Request-ID` correlation through
  every handler.
- Prometheus metrics: `shortr_http_*`, `shortr_redirects_total{outcome}`,
  `shortr_clicks_{recorded,dropped}_total`, `shortr_links_{created,
  deleted}_total`, plus Go runtime + process collectors.

#### Storage

- SQLite via pure-Go `modernc.org/sqlite` — no CGO, statically linked
  into the binary.
- goose migrations embedded via `//go:embed`; auto-applied at startup
  and exposable via `shortr migrate up|down|status`.
- `sqlc`-generated query layer in `internal/storage/sqlitegen/`. SQL files
  in `internal/storage/queries/*.sql` are the single source of truth; the
  repos are thin translation layers between sqlc's column types and the
  bounded-context value types.

#### Dashboard

- Astro 6 + React 19 islands, embedded into the Go binary via
  `//go:embed all:web/dist`.
- shadcn-ui v4 primitives: button, input, label, table, dialog, sheet,
  sonner toaster, badge, separator, dropdown-menu, select, form.
- Tailwind v4 with oklch design tokens; utilitarian dense palette.
- Login page (`/login`) for pasting the admin token into localStorage,
  with a `whoami` verification round-trip.
- Dashboard (`/`) with:
  - Inline `CreateLinkForm` above the table, with `+ options` toggle
    revealing note, expiry (date input), max_clicks, password.
  - Dense `LinksTable` with per-row icon actions: copy short URL, view
    clicks, edit, delete. Flag badges for password-protected and
    click-capped links.
  - `EditLinkSheet` (right-side panel) for all editable fields plus a
    "clear existing password" checkbox and clear-expiry / clear-max
    semantics.
  - `ClicksDrawer` (right-side panel) with an inline SVG sparkline over
    the last 30 days and a dense table of recent events (timestamp,
    country, Fly region, referrer, UA-head).
- Toast feedback via `sonner` on every mutation.

#### Deploy

- Multi-stage `deploy/Dockerfile`: `oven/bun:1.3-debian` builds the
  Astro dashboard → `golang:1.25-bookworm` compiles the Go binary with
  `web/dist/` embedded → `debian:bookworm-slim` runtime, ~33 MB image.
- Fly app `shortr-erfi` in `fra`, shared-cpu-1x / 256 MB, 1 GB volume
  `shortr_data` mounted at `/data`, 14-day snapshot retention.
- `make backup` / `make snapshots` for manual volume snapshots.
- GitHub Actions: `ci.yml` runs `go vet` + `go test -race` + `bun
  build` + `biome check` on every push and PR; `deploy.yml` runs
  `flyctl deploy --remote-only` on every `v*` tag.

### Trade-offs deliberately made

- **Single Fly region.** ~80–280 ms worst-case redirect latency. Multi-
  region adds primary election + fly-replay + per-region volumes for
  ~150 ms savings most users never notice on a URL shortener.
- **Volume snapshots, not Litestream.** RPO is 24h, retention 14 days.
  No sidecar, no S3 secrets to rotate. Add Litestream later if RPO
  becomes a hard requirement.
- **No CGO.** Slightly slower SQLite vs `mattn/go-sqlite3`, but negligible
  at this workload, and the Dockerfile gets simpler.
- **No custom domain.** `shortr-erfi.fly.dev` works; `s.erfi.io` is
  blocked by a Fly resolver hairpin on the user's Knot-on-Fly anycast
  setup. See `AGENTS.md` "Public hostname" section. Off-Fly secondary
  NS is the deferred proper fix.

[Unreleased]: https://github.com/erfianugrah/shortr/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/erfianugrah/shortr/releases/tag/v0.1.0
