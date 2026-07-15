package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

type requestIDKey struct{}

func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id == "" {
			var b [16]byte
			_, _ = rand.Read(b[:])
			id = hex.EncodeToString(b[:])
		}
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestIDKey{}, id)))
	})
}

func Logger(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		logger.Info("request", "method", r.Method, "path", r.URL.Path, "request_id", RequestIDFrom(r.Context()), "duration_ms", time.Since(start).Milliseconds())
	})
}

func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}

type RateLimitConfig struct {
	Enabled  bool
	Requests int
	Window   time.Duration
}

func RateLimit(cfg RateLimitConfig, next http.Handler) http.Handler {
	if !cfg.Enabled || cfg.Requests <= 0 || cfg.Window <= 0 {
		return next
	}
	limiter := newMemoryRateLimiter(cfg.Requests, cfg.Window)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		allowed, remaining, retryAfter := limiter.allow(rateLimitKey(r), time.Now())
		w.Header().Set("X-RateLimit-Limit", strconv.Itoa(cfg.Requests))
		w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))
		if !allowed {
			w.Header().Set("Retry-After", strconv.Itoa(retryAfterSeconds(retryAfter)))
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":{"code":"RATE_LIMITED","message":"too many requests"}}`))
			return
		}
		next.ServeHTTP(w, r)
	})
}

type rateBucket struct {
	resetAt time.Time
	used    int
}

type memoryRateLimiter struct {
	mu       sync.Mutex
	limit    int
	window   time.Duration
	requests map[string]rateBucket
}

func newMemoryRateLimiter(limit int, window time.Duration) *memoryRateLimiter {
	return &memoryRateLimiter{
		limit:    limit,
		window:   window,
		requests: map[string]rateBucket{},
	}
}

func (l *memoryRateLimiter) allow(key string, now time.Time) (bool, int, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	bucket, ok := l.requests[key]
	if !ok || !now.Before(bucket.resetAt) {
		bucket = rateBucket{resetAt: now.Add(l.window)}
	}
	bucket.used++
	l.requests[key] = bucket
	if len(l.requests) > 4096 {
		l.cleanup(now)
	}
	remaining := l.limit - bucket.used
	if remaining < 0 {
		remaining = 0
	}
	if bucket.used > l.limit {
		return false, remaining, bucket.resetAt.Sub(now)
	}
	return true, remaining, 0
}

func (l *memoryRateLimiter) cleanup(now time.Time) {
	for key, bucket := range l.requests {
		if !now.Before(bucket.resetAt) {
			delete(l.requests, key)
		}
	}
}

func rateLimitKey(r *http.Request) string {
	if forwardedFor := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); forwardedFor != "" {
		first, _, _ := strings.Cut(forwardedFor, ",")
		if trimmed := strings.TrimSpace(first); trimmed != "" {
			return trimmed
		}
	}
	if realIP := strings.TrimSpace(r.Header.Get("X-Real-IP")); realIP != "" {
		return realIP
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && host != "" {
		return host
	}
	if strings.TrimSpace(r.RemoteAddr) != "" {
		return r.RemoteAddr
	}
	return "unknown"
}

func retryAfterSeconds(duration time.Duration) int {
	if duration <= 0 {
		return 1
	}
	seconds := int(duration / time.Second)
	if duration%time.Second != 0 {
		seconds++
	}
	if seconds < 1 {
		return 1
	}
	return seconds
}

func CORS(allowedOrigins []string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && slices.Contains(allowedOrigins, origin) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Request-ID")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func RequestIDFrom(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey{}).(string)
	return id
}
