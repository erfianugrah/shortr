// Package identity is the auth bounded context.
//
// For now: a single bearer token compared in constant time. Designed to be
// swappable for OAuth / magic-links without touching the api/ layer — the
// Verifier interface returns a Principal that the api/ middleware shoves
// into ctx.
package identity

import (
	"context"
	"crypto/subtle"
	"errors"
	"strings"
)

// Principal represents an authenticated caller.
type Principal struct {
	Subject string // "admin" for the bearer-token path
	Method  string // "bearer" | "oauth-github" | ...
}

// ErrUnauthorized is returned when the credential is missing or wrong.
var ErrUnauthorized = errors.New("identity: unauthorized")

// Verifier authenticates a request. api/ middleware calls it with the
// Authorization header value (full string, e.g. "Bearer abc123").
type Verifier interface {
	Verify(ctx context.Context, authHeader string) (Principal, error)
}

// BearerVerifier is the production implementation: one shared admin token.
type BearerVerifier struct {
	token []byte
}

// NewBearerVerifier panics on empty token — startup-time misconfiguration.
func NewBearerVerifier(token string) *BearerVerifier {
	if token == "" {
		panic("identity: empty admin token")
	}
	return &BearerVerifier{token: []byte(token)}
}

// Verify implements Verifier.
func (b *BearerVerifier) Verify(_ context.Context, authHeader string) (Principal, error) {
	const prefix = "Bearer "
	if !strings.HasPrefix(authHeader, prefix) {
		return Principal{}, ErrUnauthorized
	}
	got := []byte(strings.TrimSpace(authHeader[len(prefix):]))
	if subtle.ConstantTimeCompare(got, b.token) != 1 {
		return Principal{}, ErrUnauthorized
	}
	return Principal{Subject: "admin", Method: "bearer"}, nil
}

// ctxKey is unexported so only this package can put/get the Principal.
type ctxKey struct{}

// WithPrincipal returns a context carrying p. Used by api/ middleware.
func WithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, ctxKey{}, p)
}

// FromContext returns the Principal previously stored, or zero + false.
func FromContext(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(ctxKey{}).(Principal)
	return p, ok
}
