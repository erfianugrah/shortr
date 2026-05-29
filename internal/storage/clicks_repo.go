package storage

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/erfianugrah/shortr/internal/analytics"
	"github.com/erfianugrah/shortr/internal/storage/sqlitegen"
)

// ClicksRepo is the analytics.Repo implementation backed by SQLite.
//
// SQL lives in internal/storage/queries/clicks.sql; this file translates
// between sqlitegen.Click and analytics.Click value types.
type ClicksRepo struct {
	q *sqlitegen.Queries
}

// NewClicksRepo constructs a ClicksRepo.
func NewClicksRepo(db *sql.DB) *ClicksRepo {
	return &ClicksRepo{q: sqlitegen.New(db)}
}

// InsertClick appends a click event row.
func (r *ClicksRepo) InsertClick(ctx context.Context, c analytics.Click) error {
	if err := r.q.InsertClick(ctx, sqlitegen.InsertClickParams{
		Slug:      c.Slug,
		Ts:        c.TS.Unix(),
		Country:   c.Country,
		UserAgent: c.UserAgent,
		Referrer:  c.Referrer,
		IpHash:    c.IPHash,
		FlyRegion: c.FlyRegion,
	}); err != nil {
		return fmt.Errorf("insert click: %w", err)
	}
	return nil
}

// ListClicksForSlug returns up to `limit` clicks for slug, ordered ts DESC,
// strictly before cursor.
func (r *ClicksRepo) ListClicksForSlug(ctx context.Context, slug string, cursor time.Time, limit int) ([]analytics.Click, error) {
	rows, err := r.q.ListClicksForSlug(ctx, sqlitegen.ListClicksForSlugParams{
		Slug:  slug,
		Ts:    cursor.Unix(),
		Limit: int64(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("list clicks: %w", err)
	}
	out := make([]analytics.Click, 0, len(rows))
	for _, row := range rows {
		out = append(out, analytics.Click{
			Slug:      row.Slug,
			TS:        time.Unix(row.Ts, 0).UTC(),
			Country:   row.Country,
			UserAgent: row.UserAgent,
			Referrer:  row.Referrer,
			IPHash:    row.IpHash,
			FlyRegion: row.FlyRegion,
		})
	}
	return out, nil
}

// CountClicksForSlug returns total click count for slug.
func (r *ClicksRepo) CountClicksForSlug(ctx context.Context, slug string) (int64, error) {
	n, err := r.q.CountClicksForSlug(ctx, slug)
	if err != nil {
		return 0, fmt.Errorf("count clicks: %w", err)
	}
	return n, nil
}

// ClicksByDay aggregates clicks into daily buckets for the dashboard sparkline.
// Returns a slice of {day_start_unix, count} pairs, ascending by day.
func (r *ClicksRepo) ClicksByDay(ctx context.Context, slug string, since time.Time) ([]analytics.DayBucket, error) {
	rows, err := r.q.ClicksByDay(ctx, sqlitegen.ClicksByDayParams{
		Slug: slug,
		Ts:   since.Unix(),
	})
	if err != nil {
		return nil, fmt.Errorf("clicks by day: %w", err)
	}
	out := make([]analytics.DayBucket, 0, len(rows))
	for _, row := range rows {
		out = append(out, analytics.DayBucket{
			DayStart: time.Unix(row.DayStart, 0).UTC(),
			Hits:     row.Hits,
		})
	}
	return out, nil
}
