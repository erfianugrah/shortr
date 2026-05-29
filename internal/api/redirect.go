package api

import (
	"errors"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/erfianugrah/shortr/internal/analytics"
	"github.com/erfianugrah/shortr/internal/shortener"
	"github.com/go-chi/chi/v5"
)

// handleRedirect is the hot path. One indexed query, then 302; click event
// is enqueued fire-and-forget.
func (s *Server) handleRedirect(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	if slug == "" {
		http.NotFound(w, r)
		return
	}

	now := time.Now()
	link, err := s.short.Lookup(r.Context(), slug, now)
	if err != nil {
		switch {
		case errors.Is(err, shortener.ErrNotFound):
			s.metrics.RedirectsTotal.WithLabelValues("not_found").Inc()
			http.NotFound(w, r)
		case errors.Is(err, shortener.ErrExpired):
			s.metrics.RedirectsTotal.WithLabelValues("expired").Inc()
			w.WriteHeader(http.StatusGone)
			_, _ = w.Write([]byte("link expired"))
		case errors.Is(err, shortener.ErrExhausted):
			s.metrics.RedirectsTotal.WithLabelValues("exhausted").Inc()
			w.WriteHeader(http.StatusGone)
			_, _ = w.Write([]byte("link exhausted"))
		default:
			s.metrics.RedirectLookupErrors.Inc()
			s.log.ErrorContext(r.Context(), "redirect lookup failed", "slug", slug, "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
		}
		return
	}

	// Password gate (if set).
	if link.PasswordHash != nil && *link.PasswordHash != "" {
		if pw := r.URL.Query().Get("password"); pw == "" {
			http.Error(w, "password required", http.StatusUnauthorized)
			return
		} else if err := s.short.VerifyPassword(r.Context(), slug, pw); err != nil {
			http.Error(w, "wrong password", http.StatusUnauthorized)
			return
		}
	}

	// Fire-and-forget click event.
	s.clicks.Record(analytics.Click{
		Slug:      slug,
		TS:        now,
		Country:   firstHeader(r, "Fly-Region-Country", "CF-IPCountry"),
		UserAgent: truncate(r.UserAgent(), 500),
		Referrer:  truncate(r.Referer(), 500),
		IPHash:    s.clicks.HashIP(r.RemoteAddr),
		FlyRegion: os.Getenv("FLY_REGION"),
	})

	s.metrics.RedirectsTotal.WithLabelValues("hit").Inc()
	http.Redirect(w, r, link.TargetURL, http.StatusFound)
}

func firstHeader(r *http.Request, keys ...string) string {
	for _, k := range keys {
		if v := r.Header.Get(k); v != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}
