// Package shortener owns the "links" bounded context.
//
// Public API:
//   - Link, CreateLinkInput, UpdateLinkInput value types
//   - ErrNotFound, ErrSlugTaken, ErrExpired, ErrExhausted, ErrPasswordRequired sentinel errors
//   - Service interface (consumed by api/) + DefaultService implementation
//   - Repo interface (storage-side dependency, satisfied by internal/storage/sqlitegen)
//
// The hot path is Service.Lookup — one indexed query, no allocations beyond
// what the query layer requires.
package shortener

import (
	"errors"
	"time"
)

// Link is the canonical value type for a shortened URL.
type Link struct {
	Slug         string
	TargetURL    string
	CreatedAt    time.Time
	ExpiresAt    *time.Time
	PasswordHash *string
	MaxClicks    *int64
	ClickCount   int64
	Note         string
	CreatedBy    string
}

// CreateLinkInput is the user-supplied payload for POST /api/links.
type CreateLinkInput struct {
	Slug      string     // optional — empty = auto-generate
	TargetURL string     // required, must parse as URL
	ExpiresAt *time.Time // optional
	Password  string     // optional plaintext, hashed before storage
	MaxClicks *int64     // optional
	Note      string     // optional free-form
	CreatedBy string     // set by api/ from auth principal
}

// UpdateLinkInput is the user-supplied payload for PATCH /api/links/:slug.
// Pointer fields signal "leave alone if nil".
type UpdateLinkInput struct {
	TargetURL *string
	ExpiresAt *time.Time
	ClearExpiry bool
	Password    *string // empty string clears the password
	MaxClicks   *int64
	ClearMaxClicks bool
	Note        *string
}

// Sentinel errors. api/ maps these to HTTP status codes; nothing else.
var (
	ErrNotFound          = errors.New("shortener: not found")
	ErrSlugTaken         = errors.New("shortener: slug already taken")
	ErrExpired           = errors.New("shortener: link expired")
	ErrExhausted         = errors.New("shortener: max clicks reached")
	ErrPasswordRequired  = errors.New("shortener: password required")
	ErrInvalidTargetURL  = errors.New("shortener: invalid target URL")
	ErrInvalidSlug       = errors.New("shortener: invalid slug")
	ErrSlugTooLong       = errors.New("shortener: slug too long")
)

// IsLinkActive returns nil if the link is currently usable (not expired, not
// exhausted) at the given moment. Otherwise returns the relevant error.
func IsLinkActive(l Link, now time.Time) error {
	if l.ExpiresAt != nil && !l.ExpiresAt.After(now) {
		return ErrExpired
	}
	if l.MaxClicks != nil && l.ClickCount >= *l.MaxClicks {
		return ErrExhausted
	}
	return nil
}
