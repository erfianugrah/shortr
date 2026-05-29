package identity

import (
	"context"
	"errors"
	"testing"
)

func TestBearerVerifier_Verify(t *testing.T) {
	v := NewBearerVerifier("secret-token-xyz")

	tests := []struct {
		name    string
		header  string
		wantErr error
	}{
		{"valid", "Bearer secret-token-xyz", nil},
		{"missing prefix", "secret-token-xyz", ErrUnauthorized},
		{"wrong scheme", "Basic secret-token-xyz", ErrUnauthorized},
		{"empty", "", ErrUnauthorized},
		{"wrong token", "Bearer nope", ErrUnauthorized},
		{"trailing space tolerated", "Bearer secret-token-xyz   ", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := v.Verify(context.Background(), tt.header)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err: got %v, want %v", err, tt.wantErr)
			}
			if err == nil && p.Subject != "admin" {
				t.Fatalf("subject: got %q, want admin", p.Subject)
			}
		})
	}
}

func TestNewBearerVerifierPanicsOnEmpty(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic")
		}
	}()
	_ = NewBearerVerifier("")
}

func TestPrincipalCtx(t *testing.T) {
	p := Principal{Subject: "admin", Method: "bearer"}
	ctx := WithPrincipal(context.Background(), p)
	got, ok := FromContext(ctx)
	if !ok {
		t.Fatal("FromContext: not found")
	}
	if got != p {
		t.Fatalf("FromContext: got %+v, want %+v", got, p)
	}
}
