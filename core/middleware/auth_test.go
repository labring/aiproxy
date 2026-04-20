package middleware

import (
	"net/http"
	"testing"
)

func TestRequestToken(t *testing.T) {
	tests := []struct {
		name string
		set  func(http.Header)
		want string
	}{
		{
			name: "authorization takes precedence",
			set: func(headers http.Header) {
				headers.Set("Authorization", "Bearer auth-token")
				headers.Set("X-Api-Key", "api-token")
				headers.Set("X-Goog-Api-Key", "goog-token")
			},
			want: "Bearer auth-token",
		},
		{
			name: "x api key fallback",
			set: func(headers http.Header) {
				headers.Set("X-Api-Key", "api-token")
				headers.Set("X-Goog-Api-Key", "goog-token")
			},
			want: "api-token",
		},
		{
			name: "x goog api key fallback",
			set: func(headers http.Header) {
				headers.Set("X-Goog-Api-Key", "goog-token")
			},
			want: "goog-token",
		},
		{
			name: "empty when missing",
			set:  func(http.Header) {},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			headers := make(http.Header)
			tt.set(headers)

			if got := requestToken(headers); got != tt.want {
				t.Fatalf("requestToken() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNormalizeTokenKey(t *testing.T) {
	tests := []struct {
		name string
		key  string
		want string
	}{
		{
			name: "bearer token",
			key:  "Bearer token-value",
			want: "token-value",
		},
		{
			name: "sk token",
			key:  "sk-token-value",
			want: "token-value",
		},
		{
			name: "bearer sk token",
			key:  "Bearer sk-token-value",
			want: "token-value",
		},
		{
			name: "plain token",
			key:  "token-value",
			want: "token-value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeTokenKey(tt.key); got != tt.want {
				t.Fatalf("normalizeTokenKey() = %q, want %q", got, tt.want)
			}
		})
	}
}
