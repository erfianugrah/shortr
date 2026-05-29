package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/erfianugrah/shortr/internal/shortener"
	"modernc.org/sqlite"
)

// LinksRepo is the shortener.Repo implementation backed by SQLite.
//
// Hand-written until sqlc generation is wired up. Once `make sqlc` runs,
// this can be replaced with thin adapters around internal/storage/sqlitegen.
type LinksRepo struct {
	db *sql.DB
}

// NewLinksRepo constructs a LinksRepo.
func NewLinksRepo(db *sql.DB) *LinksRepo {
	return &LinksRepo{db: db}
}

// InsertLink inserts a new link. Returns shortener.ErrSlugTaken on PK clash.
func (r *LinksRepo) InsertLink(ctx context.Context, l shortener.Link) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO links (slug, target_url, created_at, expires_at, password_hash, max_clicks, click_count, note, created_by)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		l.Slug,
		l.TargetURL,
		l.CreatedAt.Unix(),
		nullableUnix(l.ExpiresAt),
		nullableString(l.PasswordHash),
		nullableInt64(l.MaxClicks),
		l.ClickCount,
		l.Note,
		l.CreatedBy,
	)
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
	row := r.db.QueryRowContext(ctx, `
		SELECT slug, target_url, created_at, expires_at, password_hash, max_clicks, click_count, note, created_by
		FROM   links WHERE slug = ? LIMIT 1
	`, slug)
	return scanLink(row)
}

// GetActiveLink — hot path. Returns ErrNotFound if missing, ErrExpired /
// ErrExhausted if the row exists but isn't usable.
func (r *LinksRepo) GetActiveLink(ctx context.Context, slug string, now time.Time) (shortener.Link, error) {
	// Fetch the row, then evaluate state in Go — gives us specific sentinel errors
	// instead of a generic "not found" when the row is just expired.
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
	rows, err := r.db.QueryContext(ctx, `
		SELECT slug, target_url, created_at, expires_at, password_hash, max_clicks, click_count, note, created_by
		FROM   links
		WHERE  created_at < ?
		ORDER  BY created_at DESC
		LIMIT  ?
	`, cursor.Unix(), limit)
	if err != nil {
		return nil, fmt.Errorf("list links: %w", err)
	}
	defer rows.Close()

	var out []shortener.Link
	for rows.Next() {
		l, err := scanLinkRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// UpdateLink rewrites mutable fields (slug + created_at stay).
func (r *LinksRepo) UpdateLink(ctx context.Context, slug string, l shortener.Link) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE links
		SET    target_url = ?, expires_at = ?, password_hash = ?, max_clicks = ?, note = ?
		WHERE  slug = ?
	`,
		l.TargetURL,
		nullableUnix(l.ExpiresAt),
		nullableString(l.PasswordHash),
		nullableInt64(l.MaxClicks),
		l.Note,
		slug,
	)
	if err != nil {
		return fmt.Errorf("update link: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return shortener.ErrNotFound
	}
	return nil
}

// DeleteLink removes a row. ErrNotFound if missing.
func (r *LinksRepo) DeleteLink(ctx context.Context, slug string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM links WHERE slug = ?`, slug)
	if err != nil {
		return fmt.Errorf("delete link: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return shortener.ErrNotFound
	}
	return nil
}

// IncrementClickCount bumps the cached counter atomically.
func (r *LinksRepo) IncrementClickCount(ctx context.Context, slug string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE links SET click_count = click_count + 1 WHERE slug = ?`, slug)
	if err != nil {
		return fmt.Errorf("incr click_count: %w", err)
	}
	return nil
}

// --- scan helpers ---

type rowScanner interface {
	Scan(dest ...any) error
}

func scanLink(row rowScanner) (shortener.Link, error) {
	return scanLinkRow(row)
}

func scanLinkRow(row rowScanner) (shortener.Link, error) {
	var (
		l            shortener.Link
		createdUnix  int64
		expiresUnix  sql.NullInt64
		passwordHash sql.NullString
		maxClicks    sql.NullInt64
	)
	err := row.Scan(
		&l.Slug,
		&l.TargetURL,
		&createdUnix,
		&expiresUnix,
		&passwordHash,
		&maxClicks,
		&l.ClickCount,
		&l.Note,
		&l.CreatedBy,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return shortener.Link{}, shortener.ErrNotFound
		}
		return shortener.Link{}, fmt.Errorf("scan link: %w", err)
	}
	l.CreatedAt = time.Unix(createdUnix, 0).UTC()
	if expiresUnix.Valid {
		t := time.Unix(expiresUnix.Int64, 0).UTC()
		l.ExpiresAt = &t
	}
	if passwordHash.Valid {
		s := passwordHash.String
		l.PasswordHash = &s
	}
	if maxClicks.Valid {
		v := maxClicks.Int64
		l.MaxClicks = &v
	}
	return l, nil
}

// --- nullable helpers ---

func nullableUnix(t *time.Time) sql.NullInt64 {
	if t == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: t.Unix(), Valid: true}
}

func nullableString(s *string) sql.NullString {
	if s == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: *s, Valid: true}
}

func nullableInt64(v *int64) sql.NullInt64 {
	if v == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: *v, Valid: true}
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
