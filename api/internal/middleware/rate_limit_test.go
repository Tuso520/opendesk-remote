package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRateLimitRejectsOverLimit(t *testing.T) {
	nextCalls := 0
	handler := RateLimit(
		RateLimitConfig{Enabled: true, Requests: 2, Window: time.Minute},
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			nextCalls++
			w.WriteHeader(http.StatusNoContent)
		}),
	)

	for i := 0; i < 2; i++ {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/health", nil))
		if response.Code != http.StatusNoContent {
			t.Fatalf("request %d: expected 204, got %d", i+1, response.Code)
		}
	}

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/health", nil))
	if response.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", response.Code)
	}
	if nextCalls != 2 {
		t.Fatalf("rate-limited request should not reach next handler, calls=%d", nextCalls)
	}
	if response.Header().Get("Retry-After") == "" {
		t.Fatal("expected Retry-After header")
	}
}

func TestMemoryRateLimiterResetsWindow(t *testing.T) {
	limiter := newMemoryRateLimiter(1, time.Minute)
	now := time.Date(2026, 7, 9, 0, 0, 0, 0, time.UTC)
	if allowed, _, _ := limiter.allow("client", now); !allowed {
		t.Fatal("first request should be allowed")
	}
	if allowed, _, _ := limiter.allow("client", now.Add(time.Second)); allowed {
		t.Fatal("second request in same window should be rejected")
	}
	if allowed, _, _ := limiter.allow("client", now.Add(time.Minute)); !allowed {
		t.Fatal("request after reset should be allowed")
	}
}

func TestRateLimitKeyUsesForwardedFor(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.RemoteAddr = "192.0.2.10:12345"
	request.Header.Set("X-Forwarded-For", "203.0.113.20, 198.51.100.10")

	if got := rateLimitKey(request); got != "203.0.113.20" {
		t.Fatalf("unexpected rate limit key: %q", got)
	}
}

func TestRateLimitDisabledByConfig(t *testing.T) {
	nextCalls := 0
	handler := RateLimit(
		RateLimitConfig{Enabled: false, Requests: 1, Window: time.Minute},
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			nextCalls++
			w.WriteHeader(http.StatusNoContent)
		}),
	)

	for i := 0; i < 3; i++ {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/health", nil))
		if response.Code != http.StatusNoContent {
			t.Fatalf("request %d: expected 204, got %d", i+1, response.Code)
		}
	}
	if nextCalls != 3 {
		t.Fatalf("disabled rate limiter should pass all requests, calls=%d", nextCalls)
	}
}
