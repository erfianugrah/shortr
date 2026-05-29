// Package api wires the HTTP layer.
//
// Composition: Server holds references to the bounded-context services
// (shortener, analytics, identity) and a few config/log/metrics deps.
// Routes() returns the http.Handler ready for net/http.Server to serve.
package api

import (
	"log/slog"
	"net/http"

	"github.com/erfianugrah/shortr/internal/analytics"
	"github.com/erfianugrah/shortr/internal/config"
	"github.com/erfianugrah/shortr/internal/identity"
	"github.com/erfianugrah/shortr/internal/obs"
	"github.com/erfianugrah/shortr/internal/shortener"
	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
)

// Server is the HTTP composition root.
type Server struct {
	cfg       config.HTTPConfig
	short     shortener.Service
	clicks    analytics.Service
	verifier  identity.Verifier
	staticFS  http.FileSystem
	log       *slog.Logger
	metrics   *obs.Metrics
}

// Deps bundles construction-time dependencies.
type Deps struct {
	HTTP      config.HTTPConfig
	Shortener shortener.Service
	Analytics analytics.Service
	Verifier  identity.Verifier
	Static    http.FileSystem // dashboard static assets (web/dist)
	Log       *slog.Logger
	Metrics   *obs.Metrics
}

// New constructs a Server.
func New(d Deps) *Server {
	return &Server{
		cfg:      d.HTTP,
		short:    d.Shortener,
		clicks:   d.Analytics,
		verifier: d.Verifier,
		staticFS: d.Static,
		log:      d.Log,
		metrics:  d.Metrics,
	}
}

// Routes returns the configured router.
func (s *Server) Routes() http.Handler {
	r := chi.NewRouter()

	// Global middleware. Order matters:
	//   1. RequestID — cheap UUID, surfaces in logs + responses.
	//   2. RealIP    — extract x-forwarded-for / fly-client-ip.
	//   3. Recoverer — turn panics into 500.
	//   4. Logger    — slog with route + status + duration + req-id.
	//   5. Metrics   — Prometheus per-route.
	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP)
	r.Use(chimw.Recoverer)
	r.Use(s.slogMiddleware)
	r.Use(s.metricsMiddleware)

	// Public probes.
	r.Get("/healthz", s.handleHealthz)
	r.Get("/metrics", s.metrics.Handler().ServeHTTP)

	// Public API health (no auth — for status pages).
	r.Get("/api/health", s.handleAPIHealth)

	// Authenticated API.
	r.Route("/api", func(r chi.Router) {
		r.Use(s.requireAuth)
		r.Get("/me", s.handleMe)

		r.Route("/links", func(r chi.Router) {
			r.Get("/", s.handleListLinks)
			r.Post("/", s.handleCreateLink)
			r.Get("/{slug}", s.handleGetLink)
			r.Patch("/{slug}", s.handleUpdateLink)
			r.Delete("/{slug}", s.handleDeleteLink)
			r.Get("/{slug}/clicks", s.handleListClicks)
		})
	})

	// Dashboard static assets — embedded Astro build.
	if s.staticFS != nil {
		fileServer := http.FileServer(s.staticFS)
		r.Handle("/_astro/*", fileServer)
		r.Handle("/favicon.ico", fileServer)
		r.Handle("/favicon.svg", fileServer)
		r.Handle("/robots.txt", fileServer)
		r.Handle("/manifest.webmanifest", fileServer)
		r.Handle("/apple-touch-icon.png", fileServer)
		r.Get("/", s.serveIndex)
		r.Get("/login", s.serveIndex)
	}

	// Redirect catch-all — MUST be last.
	r.Get("/{slug}", s.handleRedirect)

	return r
}
