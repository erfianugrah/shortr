package api

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/erfianugrah/shortr/internal/identity"
	chimw "github.com/go-chi/chi/v5/middleware"
)

// slogMiddleware logs every request once, after the handler returns.
func (s *Server) slogMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := chimw.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)

		// Skip noise for the metrics scraper.
		if r.URL.Path == "/metrics" || r.URL.Path == "/healthz" {
			return
		}

		level := slog.LevelInfo
		switch {
		case ww.Status() >= 500:
			level = slog.LevelError
		case ww.Status() >= 400:
			level = slog.LevelWarn
		}

		s.log.LogAttrs(r.Context(), level, "http",
			slog.String("req_id", chimw.GetReqID(r.Context())),
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Int("status", ww.Status()),
			slog.Int("bytes", ww.BytesWritten()),
			slog.Duration("dur", time.Since(start)),
			slog.String("remote", r.RemoteAddr),
			slog.String("ua", r.UserAgent()),
		)
	})
}

// metricsMiddleware records HTTP counters / latency histogram.
func (s *Server) metricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := chimw.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)

		// Use the chi route pattern, falling back to the literal path so
		// metric cardinality stays bounded.
		route := r.URL.Path
		if rc := chimw.GetReqID(r.Context()); rc != "" {
			if pat := chiRoute(r); pat != "" {
				route = pat
			}
		}

		s.metrics.HTTPRequestsTotal.WithLabelValues(route, r.Method, strconv.Itoa(ww.Status())).Inc()
		s.metrics.HTTPRequestDurationS.WithLabelValues(route, r.Method).Observe(time.Since(start).Seconds())
	})
}

// chiRoute extracts the route pattern from the chi context, if available.
func chiRoute(r *http.Request) string {
	if rctx := chiCtx(r); rctx != nil {
		return rctx.RoutePattern()
	}
	return ""
}

// requireAuth gates routes behind identity.Verifier.
func (s *Server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, err := s.verifier.Verify(r.Context(), r.Header.Get("Authorization"))
		if err != nil {
			if errors.Is(err, identity.ErrUnauthorized) {
				writeProblem(w, r, http.StatusUnauthorized, "unauthorized", "valid bearer token required")
				return
			}
			writeProblem(w, r, http.StatusInternalServerError, "auth_error", err.Error())
			return
		}
		next.ServeHTTP(w, r.WithContext(identity.WithPrincipal(r.Context(), p)))
	})
}
