// Package analytics is the click-events bounded context.
//
// The hot path enqueues a Click into a buffered channel; a single writer
// goroutine drains the channel and writes to storage in small batches. On
// buffer overflow we drop the event and bump shortr_clicks_dropped_total —
// never block the redirect.
package analytics

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"time"

	"github.com/erfianugrah/shortr/internal/config"
	"github.com/erfianugrah/shortr/internal/obs"
	"github.com/erfianugrah/shortr/internal/shortener"
)

// Click is the value type captured per redirect.
type Click struct {
	Slug      string
	TS        time.Time
	Country   string
	UserAgent string
	Referrer  string
	IPHash    string
	FlyRegion string
}

// DayBucket is one row of the per-day click aggregation for sparklines.
type DayBucket struct {
	DayStart time.Time
	Hits     int64
}

// Repo is the storage dependency.
type Repo interface {
	InsertClick(ctx context.Context, c Click) error
	ListClicksForSlug(ctx context.Context, slug string, cursor time.Time, limit int) ([]Click, error)
	CountClicksForSlug(ctx context.Context, slug string) (int64, error)
	ClicksByDay(ctx context.Context, slug string, since time.Time) ([]DayBucket, error)
}

// Recorder is the public surface used by api/redirect.
type Recorder interface {
	Record(c Click)
	HashIP(addr string) string
}

// Service is the consumer-facing bounded-context interface.
type Service interface {
	Recorder
	List(ctx context.Context, slug string, cursor time.Time, limit int) ([]Click, error)
	Count(ctx context.Context, slug string) (int64, error)
	ByDay(ctx context.Context, slug string, since time.Time) ([]DayBucket, error)
}

// DefaultService is the production Service. It owns a buffered channel and
// a background writer goroutine. Call Run(ctx) to start it; Run blocks until
// ctx is done, then drains.
type DefaultService struct {
	cfg     config.AnalyticsConfig
	repo    Repo
	links   shortener.Service // for click_count increment
	log     *slog.Logger
	metrics *obs.Metrics

	ch     chan Click
	closed chan struct{}
}

// NewDefaultService constructs the service. Call Run(ctx) on the returned
// pointer to start the background writer.
func NewDefaultService(cfg config.AnalyticsConfig, repo Repo, links shortener.Service, log *slog.Logger, metrics *obs.Metrics) *DefaultService {
	if repo == nil || links == nil || log == nil || metrics == nil {
		panic("analytics: nil dependency")
	}
	return &DefaultService{
		cfg:     cfg,
		repo:    repo,
		links:   links,
		log:     log,
		metrics: metrics,
		ch:      make(chan Click, cfg.BufferSize),
		closed:  make(chan struct{}),
	}
}

// Record enqueues a click event. Drops on full buffer.
func (s *DefaultService) Record(c Click) {
	select {
	case s.ch <- c:
	default:
		s.metrics.ClicksDroppedTotal.Inc()
	}
}

// HashIP returns a salted SHA-256 hex digest of the IP, suitable for
// dedup/aggregation without storing PII.
func (s *DefaultService) HashIP(addr string) string {
	if addr == "" || s.cfg.IPHashSalt == "" {
		return ""
	}
	mac := hmac.New(sha256.New, []byte(s.cfg.IPHashSalt))
	mac.Write([]byte(addr))
	return hex.EncodeToString(mac.Sum(nil))
}

// Run drives the background writer until ctx is done, then drains the
// channel before returning.
func (s *DefaultService) Run(ctx context.Context) {
	defer close(s.closed)
	for {
		select {
		case <-ctx.Done():
			s.drain()
			return
		case c := <-s.ch:
			s.write(ctx, c)
		}
	}
}

// drain flushes any remaining buffered events on shutdown.
func (s *DefaultService) drain() {
	for {
		select {
		case c := <-s.ch:
			// shutdown context — use Background for the drain writes.
			s.write(context.Background(), c)
		default:
			return
		}
	}
}

func (s *DefaultService) write(ctx context.Context, c Click) {
	if err := s.repo.InsertClick(ctx, c); err != nil {
		s.log.WarnContext(ctx, "insert click failed", "slug", c.Slug, "err", err)
		return
	}
	if err := s.links.IncrementClickCount(ctx, c.Slug); err != nil {
		s.log.WarnContext(ctx, "increment click_count failed", "slug", c.Slug, "err", err)
		return
	}
	s.metrics.ClicksRecordedTotal.Inc()
}

// List returns recent click events for a slug.
func (s *DefaultService) List(ctx context.Context, slug string, cursor time.Time, limit int) ([]Click, error) {
	if cursor.IsZero() {
		cursor = time.Now().Add(24 * time.Hour)
	}
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	return s.repo.ListClicksForSlug(ctx, slug, cursor, limit)
}

// Count returns the total click count for a slug.
func (s *DefaultService) Count(ctx context.Context, slug string) (int64, error) {
	return s.repo.CountClicksForSlug(ctx, slug)
}

// ByDay returns per-day click aggregation since `since`.
func (s *DefaultService) ByDay(ctx context.Context, slug string, since time.Time) ([]DayBucket, error) {
	return s.repo.ClicksByDay(ctx, slug, since)
}
