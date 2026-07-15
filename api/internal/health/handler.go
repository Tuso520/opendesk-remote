package health

import (
	"context"
	"net/http"
	"time"

	"github.com/opendesk-remote/opendesk-remote/api/internal/httpx"
)

type RedisChecker interface {
	Ping(ctx context.Context) error
}

type Handler struct {
	Service       string
	Version       string
	Started       time.Time
	Redis         RedisChecker
	RedisRequired bool
	DatabaseReady bool
	StorageDriver string
}

func NewHandler(version string) Handler {
	return Handler{Service: "opendesk-api", Version: version, Started: time.Now().UTC(), StorageDriver: "local"}
}

func NewHandlerWithDeps(version string, redis RedisChecker, redisRequired bool, databaseReady bool, storageDriver string) Handler {
	handler := NewHandler(version)
	handler.Redis = redis
	handler.RedisRequired = redisRequired
	handler.DatabaseReady = databaseReady
	if storageDriver != "" {
		handler.StorageDriver = storageDriver
	}
	return handler
}

func (h Handler) Health(w http.ResponseWriter, r *http.Request) {
	if !httpx.Method(w, r, http.MethodGet) {
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"service": h.Service,
		"version": h.Version,
		"time":    time.Now().UTC().Format(time.RFC3339),
	})
}

func (h Handler) VersionHandler(w http.ResponseWriter, r *http.Request) {
	if !httpx.Method(w, r, http.MethodGet) {
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{
		"service":    h.Service,
		"version":    h.Version,
		"started_at": h.Started.Format(time.RFC3339),
	})
}

func (h Handler) Ready(w http.ResponseWriter, r *http.Request) {
	if !httpx.Method(w, r, http.MethodGet) {
		return
	}
	checks := map[string]string{
		"api":      "ok",
		"database": h.databaseStatus(),
		"redis":    h.redisStatus(r.Context()),
		"storage":  h.StorageDriver,
	}
	status := "ready"
	code := http.StatusOK
	if h.RedisRequired && checks["redis"] != "ok" {
		status = "not_ready"
		code = http.StatusServiceUnavailable
	} else if checks["redis"] != "ok" {
		status = "degraded"
	}
	httpx.JSON(w, code, map[string]any{
		"status": status,
		"checks": checks,
		"time":   time.Now().UTC().Format(time.RFC3339),
	})
}

func (h Handler) databaseStatus() string {
	if h.DatabaseReady {
		return "ok"
	}
	return "memory"
}

func (h Handler) redisStatus(ctx context.Context) string {
	if h.Redis == nil {
		return "not_configured"
	}
	pingCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	if err := h.Redis.Ping(pingCtx); err != nil {
		return "unavailable"
	}
	return "ok"
}
