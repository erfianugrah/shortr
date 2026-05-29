package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/erfianugrah/shortr/internal/shortener"
	chimw "github.com/go-chi/chi/v5/middleware"
)

// problem is the canonical JSON error body. RFC 7807-ish, simplified.
type problem struct {
	Title    string `json:"title"`
	Detail   string `json:"detail,omitempty"`
	Status   int    `json:"status"`
	RequestID string `json:"request_id,omitempty"`
}

func writeProblem(w http.ResponseWriter, r *http.Request, status int, title, detail string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(problem{
		Title:     title,
		Detail:    detail,
		Status:    status,
		RequestID: chimw.GetReqID(r.Context()),
	})
}

// mapShortenerError converts shortener sentinel errors → HTTP status.
func mapShortenerError(err error) (int, string, string) {
	switch {
	case errors.Is(err, shortener.ErrNotFound):
		return http.StatusNotFound, "not_found", "no link with that slug"
	case errors.Is(err, shortener.ErrSlugTaken):
		return http.StatusConflict, "slug_taken", "that slug is already in use"
	case errors.Is(err, shortener.ErrExpired):
		return http.StatusGone, "expired", "this link has expired"
	case errors.Is(err, shortener.ErrExhausted):
		return http.StatusGone, "exhausted", "this link has reached its click cap"
	case errors.Is(err, shortener.ErrPasswordRequired):
		return http.StatusUnauthorized, "password_required", "this link is password-protected"
	case errors.Is(err, shortener.ErrInvalidTargetURL):
		return http.StatusBadRequest, "invalid_target_url", "target URL must start with http:// or https://"
	case errors.Is(err, shortener.ErrInvalidSlug):
		return http.StatusBadRequest, "invalid_slug", "slug contains invalid characters or is reserved"
	case errors.Is(err, shortener.ErrSlugTooLong):
		return http.StatusBadRequest, "slug_too_long", "slug exceeds maximum length"
	}
	return http.StatusInternalServerError, "internal", "internal error"
}
