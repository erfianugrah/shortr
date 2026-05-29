// Package config parses environment variables into a validated Config struct.
//
// All env-var consumption in shortr happens here. Other packages receive the
// already-parsed Config (or sub-structs) — no other package calls os.Getenv.
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
)

// Config is the application's full runtime configuration.
type Config struct {
	HTTP      HTTPConfig
	Storage   StorageConfig
	Shortener ShortenerConfig
	Identity  IdentityConfig
	Analytics AnalyticsConfig
	Log       LogConfig
}

// HTTPConfig — listener configuration.
type HTTPConfig struct {
	Port            int
	PublicBaseURL   string
	ShutdownTimeout int // seconds
}

// StorageConfig — SQLite + file paths.
type StorageConfig struct {
	DBPath string
}

// ShortenerConfig — slug generation policy.
type ShortenerConfig struct {
	SlugLength      int
	SlugAlphabet    string
	MaxCustomLength int
}

// IdentityConfig — auth bearer-token settings.
type IdentityConfig struct {
	AdminToken string
}

// AnalyticsConfig — click-event capture buffer.
type AnalyticsConfig struct {
	BufferSize int
	IPHashSalt string
}

// LogConfig — slog level + format.
type LogConfig struct {
	Level  slog.Level
	Format string // "json" | "text"
}

// Load reads env vars into a Config and validates required fields.
func Load() (Config, error) {
	cfg := Config{
		HTTP: HTTPConfig{
			Port:            envInt("PORT", 8080),
			PublicBaseURL:   strings.TrimRight(envStr("PUBLIC_BASE_URL", "http://localhost:8080"), "/"),
			ShutdownTimeout: envInt("SHUTDOWN_TIMEOUT_SECONDS", 15),
		},
		Storage: StorageConfig{
			DBPath: envStr("DB_PATH", "./shortr.db"),
		},
		Shortener: ShortenerConfig{
			SlugLength:      envInt("SLUG_LENGTH", 8),
			SlugAlphabet:    envStr("SLUG_ALPHABET", "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"),
			MaxCustomLength: envInt("MAX_CUSTOM_SLUG_LENGTH", 64),
		},
		Identity: IdentityConfig{
			AdminToken: envStr("ADMIN_TOKEN", ""),
		},
		Analytics: AnalyticsConfig{
			BufferSize: envInt("CLICK_BUFFER", 1024),
			IPHashSalt: envStr("IP_HASH_SALT", ""),
		},
		Log: LogConfig{
			Level:  parseLevel(envStr("LOG_LEVEL", "info")),
			Format: strings.ToLower(envStr("LOG_FORMAT", "json")),
		},
	}

	if err := cfg.validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) validate() error {
	var errs []error
	if c.Identity.AdminToken == "" {
		errs = append(errs, errors.New("ADMIN_TOKEN is required"))
	}
	if c.HTTP.PublicBaseURL == "" {
		errs = append(errs, errors.New("PUBLIC_BASE_URL is required"))
	}
	if c.Shortener.SlugLength < 4 || c.Shortener.SlugLength > 32 {
		errs = append(errs, fmt.Errorf("SLUG_LENGTH out of range [4,32]: %d", c.Shortener.SlugLength))
	}
	if c.Analytics.BufferSize < 1 {
		errs = append(errs, fmt.Errorf("CLICK_BUFFER must be >= 1: %d", c.Analytics.BufferSize))
	}
	if c.Log.Format != "json" && c.Log.Format != "text" {
		errs = append(errs, fmt.Errorf("LOG_FORMAT must be json|text: %q", c.Log.Format))
	}
	return errors.Join(errs...)
}

func envStr(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func parseLevel(s string) slog.Level {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
