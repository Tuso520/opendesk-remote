package setup

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/opendesk-remote/opendesk-remote/api/internal/repository"
)

func TestAdminRejectsRemoteRequestWithoutSetupToken(t *testing.T) {
	handler := NewHandler(repository.NewMemoryWithoutInitialAdmin(), Config{SetupToken: "very-long-setup-token-for-tests"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/setup/admin", strings.NewReader(`{"email":"owner@example.com","password":"owner-password-12345"}`))
	req.RemoteAddr = "203.0.113.10:12345"
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.Admin(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden without setup token, got %d", rec.Code)
	}
}

func TestAdminAllowsRemoteRequestWithSetupToken(t *testing.T) {
	handler := NewHandler(repository.NewMemoryWithoutInitialAdmin(), Config{SetupToken: "very-long-setup-token-for-tests"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/setup/admin", strings.NewReader(`{"email":"owner@example.com","password":"owner-password-12345"}`))
	req.RemoteAddr = "203.0.113.10:12345"
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-OpenDesk-Setup-Token", "very-long-setup-token-for-tests")
	rec := httptest.NewRecorder()

	handler.Admin(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected setup creation with token, got %d", rec.Code)
	}
}
