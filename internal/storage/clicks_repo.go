package storage

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/erfianugrah/shortr/internal/analytics"
)

// ClicksRepo is the analytics.Repo implementation backed by SQLite.
type ClicksRepo struct {
	db *sql.DB
}

// NewClicksRepo constructs a ClicksRepo.
func NewClicksRepo(db *sql.DB) *ClicksRepo {
	return &ClicksRepo{db: db}
}

// InsertClick appends a click event row.
func (r *ClicksRepo) InsertClick(ctx context.Context, c analytics.Click) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO clicks (slug, ts, country, user_agent, referrer, ip_hash, fly_region)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`,
		c.Slug,
		c.TS.Unix(),
		c.Country,
		c.UserAgent,
		c.Referrer,
		c.IPHash,
		c.FlyRegion,
	)
	if err != nil {
		return fmt.Errorf("insert click: %w", err)
	}
	return nil
}

// ListClicksForSlug returns up to `limit` clicks for slug, ordered ts DESC,
// strictly before cursor.
func (r *ClicksRepo) ListClicksForSlug(ctx context.Context, slug string, cursor time.Time, limit int) ([]analytics.Click, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT slug, ts, country, user_agent, referrer, ip_hash, fly_region
		FROM   clicks
		WHERE  slug = ? AND ts < ?
		ORDER  BY ts DESC
		LIMIT  ?
	`, slug, cursor.Unix(), limit)
	if err != nil {
		return nil, fmt.Errorf("list clicks: %w", err)
	}
	defer rows.Close()

	var out []analytics.Click
	for rows.Next() {
		var c analytics.Click
		var tsUnix int64
		if err := rows.Scan(&c.Slug, &tsUnix, &c.Country, &c.UserAgent, &c.Referrer, &c.IPHash, &c.FlyRegion); err != nil {
			return nil, fmt.Errorf("scan click: %w", err)
		}
		c.TS = time.Unix(tsUnix, 0).UTC()
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// CountClicksForSlug returns total click count for slug.
func (r *ClicksRepo) CountClicksForSlug(ctx context.Context, slug string) (int64, error) {
	var n int64
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM clicks WHERE slug = ?`, slug).Scan(&n); err != nil {
		return 0, fmt.Errorf("count clicks: %w", err)
	}
	return n, nil
}
