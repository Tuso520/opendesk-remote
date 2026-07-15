package tests

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"testing"

	"github.com/opendesk-remote/opendesk-remote/api/internal/app"
	"github.com/opendesk-remote/opendesk-remote/api/internal/config"
	"github.com/opendesk-remote/opendesk-remote/api/internal/repository"
)

func TestSetupCreatesInitialAdminWhenNoUsersExist(t *testing.T) {
	cfg := testConfig(t)
	cfg.InitialAdminEmail = ""
	cfg.InitialAdminPassword = ""
	store := repository.NewMemoryWithoutInitialAdmin()
	server := httptest.NewServer(app.NewRouterWithStore(cfg, slog.Default(), store))
	defer server.Close()

	status, err := http.Get(server.URL + "/api/v1/setup/status")
	if err != nil {
		t.Fatalf("setup status failed: %v", err)
	}
	defer status.Body.Close()
	if status.StatusCode != http.StatusOK {
		t.Fatalf("expected setup status 200, got %d", status.StatusCode)
	}
	var statusEnvelope struct {
		Data struct {
			SetupRequired bool `json:"setup_required"`
			UsersCount    int  `json:"users_count"`
		} `json:"data"`
	}
	if err := json.NewDecoder(status.Body).Decode(&statusEnvelope); err != nil {
		t.Fatalf("decode setup status: %v", err)
	}
	if !statusEnvelope.Data.SetupRequired || statusEnvelope.Data.UsersCount != 0 {
		t.Fatalf("unexpected setup status: %+v", statusEnvelope.Data)
	}

	client := &http.Client{}
	create := postJSON(t, client, server.URL+"/api/v1/setup/admin", `{"email":"owner@example.com","password":"owner-password-12345","display_name":"Owner"}`)
	defer create.Body.Close()
	if create.StatusCode != http.StatusCreated {
		t.Fatalf("expected setup admin 201, got %d", create.StatusCode)
	}

	events, err := store.ListAuditEvents(context.Background(), repository.AuditLogFilter{Action: "setup_initial_admin"})
	if err != nil {
		t.Fatalf("list setup audit events: %v", err)
	}
	if len(events) != 1 || events[0].ResourceType != "user" {
		t.Fatalf("expected setup audit event, got %+v", events)
	}

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookie jar: %v", err)
	}
	loginClient := &http.Client{Jar: jar}
	login := postJSON(t, loginClient, server.URL+"/api/v1/auth/login", `{"email":"owner@example.com","password":"owner-password-12345"}`)
	defer login.Body.Close()
	if login.StatusCode != http.StatusOK {
		t.Fatalf("expected setup-created admin login 200, got %d", login.StatusCode)
	}

	second := postJSON(t, client, server.URL+"/api/v1/setup/admin", `{"email":"other@example.com","password":"owner-password-12345"}`)
	defer second.Body.Close()
	if second.StatusCode != http.StatusConflict {
		t.Fatalf("expected second setup 409, got %d", second.StatusCode)
	}
}

func TestNewStoreDoesNotCreateImplicitDefaultAdmin(t *testing.T) {
	cfg := config.Config{
		StorageDriver:        "local",
		RelayGrantSigningKey: "test-relay-signing-key",
		RelayAuthRequired:    true,
		StorageLocalPath:     t.TempDir(),
	}
	store, cleanup, err := app.NewStore(context.Background(), cfg, slog.Default())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	defer cleanup()
	count, err := store.CountUsers(context.Background())
	if err != nil {
		t.Fatalf("count users: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected no implicit default admin, got %d users", count)
	}
}
