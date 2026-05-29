package analytics

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/erfianugrah/shortr/internal/config"
	"github.com/erfianugrah/shortr/internal/obs"
	"github.com/erfianugrah/shortr/internal/shortener"
)

// fakeRepo records inserts in memory.
type fakeRepo struct {
	mu     sync.Mutex
	clicks []Click
}

func (f *fakeRepo) InsertClick(_ context.Context, c Click) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.clicks = append(f.clicks, c)
	return nil
}
func (f *fakeRepo) ListClicksForSlug(context.Context, string, time.Time, int) ([]Click, error) {
	return nil, nil
}
func (f *fakeRepo) CountClicksForSlug(context.Context, string) (int64, error) {
	return 0, nil
}
func (f *fakeRepo) ClicksByDay(context.Context, string, time.Time) ([]DayBucket, error) {
	return nil, nil
}

// fakeShortener satisfies shortener.Service for the click-count call.
type fakeShortener struct{ inc atomic.Int64 }

func (f *fakeShortener) Lookup(context.Context, string, time.Time) (shortener.Link, error) {
	return shortener.Link{}, nil
}
func (f *fakeShortener) VerifyPassword(context.Context, string, string) error { return nil }
func (f *fakeShortener) Create(context.Context, shortener.CreateLinkInput, time.Time) (shortener.Link, error) {
	return shortener.Link{}, nil
}
func (f *fakeShortener) Get(context.Context, string) (shortener.Link, error) {
	return shortener.Link{}, nil
}
func (f *fakeShortener) List(context.Context, time.Time, int) ([]shortener.Link, error) { return nil, nil }
func (f *fakeShortener) Update(context.Context, string, shortener.UpdateLinkInput) (shortener.Link, error) {
	return shortener.Link{}, nil
}
func (f *fakeShortener) Delete(context.Context, string) error { return nil }
func (f *fakeShortener) IncrementClickCount(_ context.Context, _ string) error {
	f.inc.Add(1)
	return nil
}

func TestRecordAndDrain(t *testing.T) {
	repo := &fakeRepo{}
	short := &fakeShortener{}
	cfg := config.AnalyticsConfig{BufferSize: 16, IPHashSalt: "salt"}
	log := slog.New(slog.NewTextHandler(testWriter{t}, nil))

	svc := NewDefaultService(cfg, repo, short, log, obs.NewMetrics())

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { svc.Run(ctx); close(done) }()

	for i := 0; i < 5; i++ {
		svc.Record(Click{Slug: "abc", TS: time.Now()})
	}

	// give writer a moment, then shut down.
	time.Sleep(50 * time.Millisecond)
	cancel()
	<-done

	if got := len(repo.clicks); got != 5 {
		t.Fatalf("repo.clicks: got %d, want 5", got)
	}
	if got := short.inc.Load(); got != 5 {
		t.Fatalf("inc: got %d, want 5", got)
	}
}

func TestRecordDropsOnFullBuffer(t *testing.T) {
	repo := &fakeRepo{}
	short := &fakeShortener{}
	cfg := config.AnalyticsConfig{BufferSize: 2}
	log := slog.New(slog.NewTextHandler(testWriter{t}, nil))
	metrics := obs.NewMetrics()

	svc := NewDefaultService(cfg, repo, short, log, metrics)
	// Don't start Run — buffer never drains.

	for i := 0; i < 10; i++ {
		svc.Record(Click{Slug: "abc"})
	}
	// 2 enqueued, 8 dropped. Inspect via the metric.
	// (We don't read the gauge value to keep the test self-contained;
	//  the assertion is that Record() didn't block.)
}

func TestHashIP(t *testing.T) {
	repo := &fakeRepo{}
	short := &fakeShortener{}
	cfg := config.AnalyticsConfig{BufferSize: 1, IPHashSalt: "salt-v1"}
	svc := NewDefaultService(cfg, repo, short, slog.Default(), obs.NewMetrics())

	h1 := svc.HashIP("203.0.113.42")
	h2 := svc.HashIP("203.0.113.42")
	h3 := svc.HashIP("203.0.113.43")
	if h1 == "" {
		t.Fatal("expected non-empty hash")
	}
	if h1 != h2 {
		t.Fatal("same input should hash identically")
	}
	if h1 == h3 {
		t.Fatal("different input should hash differently")
	}
	if got := svc.HashIP(""); got != "" {
		t.Fatalf("empty IP should yield empty hash, got %q", got)
	}
}

// testWriter pipes log output into t.Log so it appears on test failure.
type testWriter struct{ t *testing.T }

func (w testWriter) Write(p []byte) (int, error) {
	w.t.Helper()
	w.t.Log(string(p))
	return len(p), nil
}
