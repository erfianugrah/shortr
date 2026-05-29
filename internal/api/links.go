package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/erfianugrah/shortr/internal/identity"
	"github.com/erfianugrah/shortr/internal/shortener"
	"github.com/go-chi/chi/v5"
)

// linkDTO is the over-the-wire representation. Times are RFC3339 strings;
// pointer fields stay null when absent.
type linkDTO struct {
	Slug       string  `json:"slug"`
	TargetURL  string  `json:"target_url"`
	CreatedAt  string  `json:"created_at"`
	ExpiresAt  *string `json:"expires_at"`
	HasPassword bool   `json:"has_password"`
	MaxClicks  *int64  `json:"max_clicks"`
	ClickCount int64   `json:"click_count"`
	Note       string  `json:"note"`
	CreatedBy  string  `json:"created_by"`
}

func toLinkDTO(l shortener.Link) linkDTO {
	dto := linkDTO{
		Slug:       l.Slug,
		TargetURL:  l.TargetURL,
		CreatedAt:  l.CreatedAt.UTC().Format(time.RFC3339),
		HasPassword: l.PasswordHash != nil && *l.PasswordHash != "",
		MaxClicks:  l.MaxClicks,
		ClickCount: l.ClickCount,
		Note:       l.Note,
		CreatedBy:  l.CreatedBy,
	}
	if l.ExpiresAt != nil {
		s := l.ExpiresAt.UTC().Format(time.RFC3339)
		dto.ExpiresAt = &s
	}
	return dto
}

// createLinkRequest mirrors the zod schema in web/src/lib/schemas.ts.
type createLinkRequest struct {
	Slug      string  `json:"slug"`
	TargetURL string  `json:"target_url"`
	ExpiresAt *string `json:"expires_at"`
	Password  string  `json:"password"`
	MaxClicks *int64  `json:"max_clicks"`
	Note      string  `json:"note"`
}

func (s *Server) handleCreateLink(w http.ResponseWriter, r *http.Request) {
	var req createLinkRequest
	if err := decodeJSON(r, &req); err != nil {
		writeProblem(w, r, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	in := shortener.CreateLinkInput{
		Slug:      req.Slug,
		TargetURL: req.TargetURL,
		Password:  req.Password,
		MaxClicks: req.MaxClicks,
		Note:      req.Note,
	}
	if req.ExpiresAt != nil && *req.ExpiresAt != "" {
		t, err := time.Parse(time.RFC3339, *req.ExpiresAt)
		if err != nil {
			writeProblem(w, r, http.StatusBadRequest, "bad_request", "expires_at must be RFC3339")
			return
		}
		in.ExpiresAt = &t
	}
	if p, ok := identity.FromContext(r.Context()); ok {
		in.CreatedBy = p.Subject
	}

	link, err := s.short.Create(r.Context(), in, time.Now())
	if err != nil {
		status, title, detail := mapShortenerError(err)
		writeProblem(w, r, status, title, detail)
		return
	}
	writeJSON(w, http.StatusCreated, toLinkDTO(link))
}

func (s *Server) handleGetLink(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	link, err := s.short.Get(r.Context(), slug)
	if err != nil {
		status, title, detail := mapShortenerError(err)
		writeProblem(w, r, status, title, detail)
		return
	}
	writeJSON(w, http.StatusOK, toLinkDTO(link))
}

func (s *Server) handleListLinks(w http.ResponseWriter, r *http.Request) {
	cursor := time.Time{}
	if c := r.URL.Query().Get("cursor"); c != "" {
		t, err := time.Parse(time.RFC3339, c)
		if err == nil {
			cursor = t
		}
	}
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil {
			limit = n
		}
	}

	links, err := s.short.List(r.Context(), cursor, limit)
	if err != nil {
		writeProblem(w, r, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	out := make([]linkDTO, 0, len(links))
	for _, l := range links {
		out = append(out, toLinkDTO(l))
	}
	writeJSON(w, http.StatusOK, map[string]any{"links": out})
}

type updateLinkRequest struct {
	TargetURL      *string `json:"target_url"`
	ExpiresAt      *string `json:"expires_at"`
	ClearExpiry    bool    `json:"clear_expiry"`
	Password       *string `json:"password"`
	MaxClicks      *int64  `json:"max_clicks"`
	ClearMaxClicks bool    `json:"clear_max_clicks"`
	Note           *string `json:"note"`
}

func (s *Server) handleUpdateLink(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	var req updateLinkRequest
	if err := decodeJSON(r, &req); err != nil {
		writeProblem(w, r, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	in := shortener.UpdateLinkInput{
		TargetURL:      req.TargetURL,
		ClearExpiry:    req.ClearExpiry,
		Password:       req.Password,
		MaxClicks:      req.MaxClicks,
		ClearMaxClicks: req.ClearMaxClicks,
		Note:           req.Note,
	}
	if req.ExpiresAt != nil && *req.ExpiresAt != "" && !req.ClearExpiry {
		t, err := time.Parse(time.RFC3339, *req.ExpiresAt)
		if err != nil {
			writeProblem(w, r, http.StatusBadRequest, "bad_request", "expires_at must be RFC3339")
			return
		}
		in.ExpiresAt = &t
	}

	link, err := s.short.Update(r.Context(), slug, in)
	if err != nil {
		status, title, detail := mapShortenerError(err)
		writeProblem(w, r, status, title, detail)
		return
	}
	writeJSON(w, http.StatusOK, toLinkDTO(link))
}

func (s *Server) handleDeleteLink(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	if err := s.short.Delete(r.Context(), slug); err != nil {
		status, title, detail := mapShortenerError(err)
		writeProblem(w, r, status, title, detail)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleListClicks(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	cursor := time.Time{}
	if c := r.URL.Query().Get("cursor"); c != "" {
		if t, err := time.Parse(time.RFC3339, c); err == nil {
			cursor = t
		}
	}
	limit := 100
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil {
			limit = n
		}
	}
	days := 30
	if d := r.URL.Query().Get("days"); d != "" {
		if n, err := strconv.Atoi(d); err == nil && n > 0 && n <= 365 {
			days = n
		}
	}

	events, err := s.clicks.List(r.Context(), slug, cursor, limit)
	if err != nil {
		writeProblem(w, r, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	count, _ := s.clicks.Count(r.Context(), slug)

	since := time.Now().AddDate(0, 0, -days)
	buckets, _ := s.clicks.ByDay(r.Context(), slug, since)

	dtoBuckets := make([]map[string]any, 0, len(buckets))
	for _, b := range buckets {
		dtoBuckets = append(dtoBuckets, map[string]any{
			"day":  b.DayStart.UTC().Format("2006-01-02"),
			"hits": b.Hits,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"slug":     slug,
		"count":    count,
		"events":   events,
		"by_day":   dtoBuckets,
		"since":    since.UTC().Format(time.RFC3339),
	})
}

// --- shared json helpers ---

func decodeJSON(r *http.Request, v any) error {
	defer r.Body.Close()
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20)) // 1 MiB
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return errors.New("invalid json body: " + err.Error())
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
