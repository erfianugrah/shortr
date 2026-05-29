package shortener

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"github.com/erfianugrah/shortr/internal/config"
	"github.com/erfianugrah/shortr/internal/obs"
	"golang.org/x/crypto/bcrypt"
)

// Service is the bounded-context interface consumed by api/.
//
// Implementations should be safe for concurrent use.
type Service interface {
	// Lookup is the redirect hot path. Must be cheap.
	Lookup(ctx context.Context, slug string, now time.Time) (Link, error)

	// VerifyPassword returns nil if the supplied password matches the link's hash.
	// Returns ErrPasswordRequired if the link has no password.
	VerifyPassword(ctx context.Context, slug, password string) error

	// Create inserts a new link, generating a slug if input.Slug is empty.
	Create(ctx context.Context, input CreateLinkInput, now time.Time) (Link, error)

	// Get returns the full link record, including expired/exhausted ones (admin).
	Get(ctx context.Context, slug string) (Link, error)

	// List paginates links by created_at descending. Pass cursor=time.Time{} for first page.
	List(ctx context.Context, cursor time.Time, limit int) ([]Link, error)

	// Update modifies a link's fields.
	Update(ctx context.Context, slug string, input UpdateLinkInput) (Link, error)

	// Delete removes a link and (via FK cascade) its clicks.
	Delete(ctx context.Context, slug string) error

	// IncrementClickCount bumps the cached counter on the link row.
	// Called by the analytics writer goroutine, not on the hot path directly.
	IncrementClickCount(ctx context.Context, slug string) error
}

// Repo is the storage dependency. internal/storage/sqlitegen.Queries
// satisfies a superset of this; we wrap it in a thin adapter (in
// shortener/storage.go) to keep this package independent of the generated
// types.
type Repo interface {
	InsertLink(ctx context.Context, l Link) error
	GetLink(ctx context.Context, slug string) (Link, error)
	GetActiveLink(ctx context.Context, slug string, now time.Time) (Link, error)
	ListLinks(ctx context.Context, cursor time.Time, limit int) ([]Link, error)
	UpdateLink(ctx context.Context, slug string, l Link) error
	DeleteLink(ctx context.Context, slug string) error
	IncrementClickCount(ctx context.Context, slug string) error
}

// DefaultService is the production Service implementation.
type DefaultService struct {
	repo    Repo
	cfg     config.ShortenerConfig
	log     *slog.Logger
	metrics *obs.Metrics
}

// NewDefaultService constructs a Service. All deps are required (panic at startup if nil).
func NewDefaultService(repo Repo, cfg config.ShortenerConfig, log *slog.Logger, metrics *obs.Metrics) *DefaultService {
	if repo == nil || log == nil || metrics == nil {
		panic("shortener: nil dependency")
	}
	return &DefaultService{repo: repo, cfg: cfg, log: log, metrics: metrics}
}

// Lookup implements the redirect hot path.
func (s *DefaultService) Lookup(ctx context.Context, slug string, now time.Time) (Link, error) {
	l, err := s.repo.GetActiveLink(ctx, slug, now)
	if err != nil {
		return Link{}, err
	}
	return l, nil
}

func (s *DefaultService) VerifyPassword(ctx context.Context, slug, password string) error {
	l, err := s.repo.GetLink(ctx, slug)
	if err != nil {
		return err
	}
	if l.PasswordHash == nil || *l.PasswordHash == "" {
		return nil
	}
	if err := bcrypt.CompareHashAndPassword([]byte(*l.PasswordHash), []byte(password)); err != nil {
		return ErrPasswordRequired
	}
	return nil
}

func (s *DefaultService) Create(ctx context.Context, input CreateLinkInput, now time.Time) (Link, error) {
	if err := validateTargetURL(input.TargetURL); err != nil {
		return Link{}, err
	}
	if input.Slug != "" {
		if err := s.validateSlug(input.Slug); err != nil {
			return Link{}, err
		}
	}

	link := Link{
		TargetURL: input.TargetURL,
		CreatedAt: now,
		ExpiresAt: input.ExpiresAt,
		MaxClicks: input.MaxClicks,
		Note:      input.Note,
		CreatedBy: input.CreatedBy,
	}
	if input.Password != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
		if err != nil {
			return Link{}, fmt.Errorf("hash password: %w", err)
		}
		s := string(hash)
		link.PasswordHash = &s
	}

	// Slug-or-generate, with retry on collision when auto-generating.
	if input.Slug != "" {
		link.Slug = input.Slug
		if err := s.repo.InsertLink(ctx, link); err != nil {
			return Link{}, err
		}
	} else {
		const maxAttempts = 5
		for i := 0; i < maxAttempts; i++ {
			link.Slug = generateSlug(s.cfg.SlugLength, s.cfg.SlugAlphabet)
			err := s.repo.InsertLink(ctx, link)
			if err == nil {
				break
			}
			if !errors.Is(err, ErrSlugTaken) {
				return Link{}, err
			}
			if i == maxAttempts-1 {
				return Link{}, fmt.Errorf("slug generation: %w", err)
			}
		}
	}

	s.metrics.LinksCreatedTotal.Inc()
	s.log.InfoContext(ctx, "link created", "slug", link.Slug, "by", link.CreatedBy)
	return link, nil
}

func (s *DefaultService) Get(ctx context.Context, slug string) (Link, error) {
	return s.repo.GetLink(ctx, slug)
}

func (s *DefaultService) List(ctx context.Context, cursor time.Time, limit int) ([]Link, error) {
	if cursor.IsZero() {
		cursor = time.Now().Add(24 * time.Hour) // future-stamp = first page
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	return s.repo.ListLinks(ctx, cursor, limit)
}

func (s *DefaultService) Update(ctx context.Context, slug string, input UpdateLinkInput) (Link, error) {
	current, err := s.repo.GetLink(ctx, slug)
	if err != nil {
		return Link{}, err
	}
	if input.TargetURL != nil {
		if err := validateTargetURL(*input.TargetURL); err != nil {
			return Link{}, err
		}
		current.TargetURL = *input.TargetURL
	}
	if input.ClearExpiry {
		current.ExpiresAt = nil
	} else if input.ExpiresAt != nil {
		current.ExpiresAt = input.ExpiresAt
	}
	if input.ClearMaxClicks {
		current.MaxClicks = nil
	} else if input.MaxClicks != nil {
		current.MaxClicks = input.MaxClicks
	}
	if input.Password != nil {
		if *input.Password == "" {
			current.PasswordHash = nil
		} else {
			hash, err := bcrypt.GenerateFromPassword([]byte(*input.Password), bcrypt.DefaultCost)
			if err != nil {
				return Link{}, fmt.Errorf("hash password: %w", err)
			}
			h := string(hash)
			current.PasswordHash = &h
		}
	}
	if input.Note != nil {
		current.Note = *input.Note
	}
	if err := s.repo.UpdateLink(ctx, slug, current); err != nil {
		return Link{}, err
	}
	return current, nil
}

func (s *DefaultService) Delete(ctx context.Context, slug string) error {
	if err := s.repo.DeleteLink(ctx, slug); err != nil {
		return err
	}
	s.metrics.LinksDeletedTotal.Inc()
	return nil
}

func (s *DefaultService) IncrementClickCount(ctx context.Context, slug string) error {
	return s.repo.IncrementClickCount(ctx, slug)
}

// --- helpers ---

func (s *DefaultService) validateSlug(slug string) error {
	if slug == "" {
		return ErrInvalidSlug
	}
	if len(slug) > s.cfg.MaxCustomLength {
		return ErrSlugTooLong
	}
	for _, r := range slug {
		if !isAllowedSlugRune(r) {
			return ErrInvalidSlug
		}
	}
	// reserve some prefixes for internal routes
	for _, reserved := range []string{"api", "healthz", "metrics", "_app", "favicon.ico", "robots.txt", "login"} {
		if slug == reserved {
			return ErrInvalidSlug
		}
	}
	return nil
}

func isAllowedSlugRune(r rune) bool {
	switch {
	case r >= '0' && r <= '9':
		return true
	case r >= 'a' && r <= 'z':
		return true
	case r >= 'A' && r <= 'Z':
		return true
	case r == '-' || r == '_':
		return true
	}
	return false
}

func validateTargetURL(s string) error {
	if s == "" {
		return ErrInvalidTargetURL
	}
	if !strings.HasPrefix(s, "http://") && !strings.HasPrefix(s, "https://") {
		return ErrInvalidTargetURL
	}
	u, err := url.Parse(s)
	if err != nil || u.Host == "" {
		return ErrInvalidTargetURL
	}
	return nil
}

func generateSlug(length int, alphabet string) string {
	if length <= 0 {
		length = 8
	}
	if alphabet == "" {
		alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	}
	buf := make([]byte, length)
	letters := []byte(alphabet)
	rnd := make([]byte, length)
	if _, err := rand.Read(rnd); err != nil {
		// crypto/rand failure is fatal; surface as a panic — startup-time
		// bug, not a runtime path the hot loop needs to handle.
		panic(fmt.Errorf("shortener: rand.Read: %w", err))
	}
	for i := range buf {
		buf[i] = letters[int(rnd[i])%len(letters)]
	}
	return string(buf)
}
