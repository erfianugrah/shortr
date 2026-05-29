package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/erfianugrah/shortr/internal/shortener"
	"github.com/erfianugrah/shortr/internal/storage/sqlitegen"
	"modernc.org/sqlite"
)

// LinksRepo is the shortener.Repo implementation backed by SQLite.
//
// The actual SQL lives in internal/storage/queries/links.sql and is compiled
// to type-safe Go via sqlc. This file is the translation layer between the
// generated sqlitegen.Link type (column-shaped) and the shortener.Link value
// type (domain-shaped), plus sentinel-error mapping.
type LinksRepo struct {
	q *sqlitegen.Queries
}

// NewLinksRepo constructs a LinksRepo.
func NewLinksRepo(db *sql.DB) *LinksRepo {
	return &LinksRepo{q: sqlitegen.New(db)}
}

// InsertLink inserts a new link. Returns shortener.ErrSlugTaken on PK clash.
func (r *LinksRepo) InsertLink(ctx context.Context, l shortener.Link) error {
	err := r.q.InsertLink(ctx, sqlitegen.InsertLinkParams{
		Slug:         l.Slug,
		TargetUrl:    l.TargetURL,
		CreatedAt:    l.CreatedAt.Unix(),
		ExpiresAt:    timeToUnixPtr(l.ExpiresAt),
		PasswordHash: l.PasswordHash,
		MaxClicks:    l.MaxClicks,
		Note:         l.Note,
		CreatedBy:    l.CreatedBy,
	})
	if isUniqueViolation(err) {
		return shortener.ErrSlugTaken
	}
	if err != nil {
		return fmt.Errorf("insert link: %w", err)
	}
	return nil
}

// GetLink returns the full row regardless of expiry/exhaustion.
func (r *LinksRepo) GetLink(ctx context.Context, slug string) (shortener.Link, error) {
	row, err := r.q.GetLink(ctx, slug)
	if errors.Is(err, sql.ErrNoRows) {
		return shortener.Link{}, shortener.ErrNotFound
	}
	if err != nil {
		return shortener.Link{}, fmt.Errorf("get link: %w", err)
	}
	return rowToLink(row), nil
}

// GetActiveLink — hot path. Returns ErrNotFound if missing, ErrExpired /
// ErrExhausted if the row exists but isn't usable.
//
// We don't use the sqlc-generated `GetActiveLink` (which combines the active
// check into the WHERE clause) because we want specific sentinel errors for
// the expired/exhausted cases, not a generic "no row found". Cost is one
// row scanned that might be ignored; negligible at this scale.
func (r *LinksRepo) GetActiveLink(ctx context.Context, slug string, now time.Time) (shortener.Link, error) {
	link, err := r.GetLink(ctx, slug)
	if err != nil {
		return shortener.Link{}, err
	}
	if err := shortener.IsLinkActive(link, now); err != nil {
		return shortener.Link{}, err
	}
	return link, nil
}

// ListLinks paginates by created_at descending; cursor is exclusive.
func (r *LinksRepo) ListLinks(ctx context.Context, cursor time.Time, limit int) ([]shortener.Link, error) {
	rows, err := r.q.ListLinks(ctx, sqlitegen.ListLinksParams{
		CreatedAt: cursor.Unix(),
		Limit:     int64(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("list links: %w", err)
	}
	out := make([]shortener.Link, 0, len(rows))
	for _, row := range rows {
		out = append(out, rowToLink(row))
	}
	return out, nil
}

// UpdateLink rewrites mutable fields (slug + created_at stay).
func (r *LinksRepo) UpdateLink(ctx context.Context, slug string, l shortener.Link) error {
	// Existence check first so we can return ErrNotFound on missing rows
	// (sqlc's UpdateLink with :exec swallows RowsAffected).
	if _, err := r.q.GetLink(ctx, slug); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return shortener.ErrNotFound
		}
		return fmt.Errorf("update link (pre-check): %w", err)
	}
	if err := r.q.UpdateLink(ctx, sqlitegen.UpdateLinkParams{
		TargetUrl:    l.TargetURL,
		ExpiresAt:    timeToUnixPtr(l.ExpiresAt),
		PasswordHash: l.PasswordHash,
		MaxClicks:    l.MaxClicks,
		Note:         l.Note,
		Slug:         slug,
	}); err != nil {
		return fmt.Errorf("update link: %w", err)
	}
	return nil
}

// DeleteLink removes a row. ErrNotFound if missing.
func (r *LinksRepo) DeleteLink(ctx context.Context, slug string) error {
	if _, err := r.q.GetLink(ctx, slug); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return shortener.ErrNotFound
		}
		return fmt.Errorf("delete link (pre-check): %w", err)
	}
	if err := r.q.DeleteLink(ctx, slug); err != nil {
		return fmt.Errorf("delete link: %w", err)
	}
	return nil
}

// IncrementClickCount bumps the cached counter atomically.
func (r *LinksRepo) IncrementClickCount(ctx context.Context, slug string) error {
	if err := r.q.IncrementClickCount(ctx, slug); err != nil {
		return fmt.Errorf("incr click_count: %w", err)
	}
	return nil
}

// --- type translation ---

func rowToLink(row sqlitegen.Link) shortener.Link {
	out := shortener.Link{
		Slug:         row.Slug,
		TargetURL:    row.TargetUrl,
		CreatedAt:    time.Unix(row.CreatedAt, 0).UTC(),
		PasswordHash: row.PasswordHash,
		MaxClicks:    row.MaxClicks,
		ClickCount:   row.ClickCount,
		Note:         row.Note,
		CreatedBy:    row.CreatedBy,
	}
	if row.ExpiresAt != nil {
		t := time.Unix(*row.ExpiresAt, 0).UTC()
		out.ExpiresAt = &t
	}
	return out
}

func timeToUnixPtr(t *time.Time) *int64 {
	if t == nil {
		return nil
	}
	v := t.Unix()
	return &v
}

// isUniqueViolation detects modernc.org/sqlite UNIQUE constraint failures.
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	var serr *sqlite.Error
	if errors.As(err, &serr) {
		// sqlite extended result codes: 2067 = SQLITE_CONSTRAINT_UNIQUE,
		// 1555 = SQLITE_CONSTRAINT_PRIMARYKEY.
		switch serr.Code() {
		case 2067, 1555:
			return true
		}
	}
	return false
}
