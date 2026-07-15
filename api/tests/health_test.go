package tests

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/opendesk-remote/opendesk-remote/api/internal/app"
	"github.com/opendesk-remote/opendesk-remote/api/internal/config"
)

func TestHealthEndpoint(t *testing.T) {
	cfg := config.Config{
		HTTPAddr:              ":0",
		RelayGrantSigningKey:  "test-signing-key",
		RelayAuthRequired:     true,
		RelayGrantTTL:         time.Minute,
		AllowedCORSOrigins:    []string{"http://localhost:5173"},
		BrandingAssetMaxBytes: 1024,
		StorageDriver:         "local",
		StorageLocalPath:      t.TempDir(),
	}
	server := httptest.NewServer(app.NewRouter(cfg, slog.Default()))
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/v1/health")
	if err != nil {
		t.Fatalf("health request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestReadyEndpoint(t *testing.T) {
	cfg := config.Config{
		HTTPAddr:              ":0",
		RelayGrantSigningKey:  "test-signing-key",
		RelayAuthRequired:     true,
		RelayGrantTTL:         time.Minute,
		AllowedCORSOrigins:    []string{"http://localhost:5173"},
		BrandingAssetMaxBytes: 1024,
		StorageDriver:         "local",
		StorageLocalPath:      t.TempDir(),
	}
	server := httptest.NewServer(app.NewRouter(cfg, slog.Default()))
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/v1/ready")
	if err != nil {
		t.Fatalf("ready request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var envelope struct {
		Data struct {
			Status string            `json:"status"`
			Checks map[string]string `json:"checks"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		t.Fatalf("decode ready: %v", err)
	}
	if envelope.Data.Status != "degraded" || envelope.Data.Checks["redis"] == "ok" {
		t.Fatalf("expected development ready to report degraded redis, got %+v", envelope.Data)
	}
}

func TestReadyEndpointRequiresRedisInProduction(t *testing.T) {
	cfg := config.Config{
		Env:                   "production",
		HTTPAddr:              ":0",
		MySQLDSN:              "configured-for-readiness",
		RedisAddr:             "127.0.0.1:1",
		JWTSecret:             "local-dev-session-signing-key-with-enough-length",
		RelayGrantSigningKey:  "test-signing-key",
		RelayAuthRequired:     true,
		RelayGrantTTL:         time.Minute,
		AllowedCORSOrigins:    []string{"http://localhost:5173"},
		BrandingAssetMaxBytes: 1024,
		StorageDriver:         "local",
		StorageLocalPath:      t.TempDir(),
	}
	server := httptest.NewServer(app.NewRouter(cfg, slog.Default()))
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/v1/ready")
	if err != nil {
		t.Fatalf("ready request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("expected production redis failure 503, got %d", resp.StatusCode)
	}
}
