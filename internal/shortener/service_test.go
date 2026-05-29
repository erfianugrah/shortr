package shortener

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestIsLinkActive(t *testing.T) {
	now := time.Date(2026, 5, 29, 12, 0, 0, 0, time.UTC)
	past := now.Add(-1 * time.Hour)
	future := now.Add(1 * time.Hour)

	tests := []struct {
		name string
		link Link
		want error
	}{
		{
			name: "no expiry no cap → active",
			link: Link{Slug: "ok"},
			want: nil,
		},
		{
			name: "expired",
			link: Link{Slug: "x", ExpiresAt: &past},
			want: ErrExpired,
		},
		{
			name: "expiry in future → active",
			link: Link{Slug: "x", ExpiresAt: &future},
			want: nil,
		},
		{
			name: "max_clicks reached → exhausted",
			link: Link{Slug: "x", ClickCount: 5, MaxClicks: ptr64(5)},
			want: ErrExhausted,
		},
		{
			name: "max_clicks not yet reached → active",
			link: Link{Slug: "x", ClickCount: 4, MaxClicks: ptr64(5)},
			want: nil,
		},
		{
			name: "expired beats exhausted",
			link: Link{Slug: "x", ClickCount: 9, MaxClicks: ptr64(5), ExpiresAt: &past},
			want: ErrExpired,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsLinkActive(tt.link, now)
			if !errors.Is(got, tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidateTargetURL(t *testing.T) {
	cases := map[string]error{
		"":                                     ErrInvalidTargetURL,
		"not a url":                            ErrInvalidTargetURL,
		"ftp://example.com":                    ErrInvalidTargetURL,
		"http://":                              ErrInvalidTargetURL,
		"http://example.com":                   nil,
		"https://example.com/path?q=1":         nil,
		"https://news.ycombinator.com/item?id": nil,
	}
	for in, want := range cases {
		t.Run(in, func(t *testing.T) {
			if got := validateTargetURL(in); !errors.Is(got, want) {
				t.Fatalf("validateTargetURL(%q): got %v, want %v", in, got, want)
			}
		})
	}
}

func TestGenerateSlug(t *testing.T) {
	for length := 4; length <= 16; length += 4 {
		s := generateSlug(length, "0123456789abcdef")
		if len(s) != length {
			t.Fatalf("len=%d got %d", length, len(s))
		}
		for _, r := range s {
			if !strings.ContainsRune("0123456789abcdef", r) {
				t.Fatalf("rune %q not in alphabet", r)
			}
		}
	}
}

func ptr64(v int64) *int64 { return &v }
