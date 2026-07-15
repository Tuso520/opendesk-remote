package users

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/opendesk-remote/opendesk-remote/api/internal/models"
	"github.com/opendesk-remote/opendesk-remote/api/internal/repository"
)

func TestUserItemGetUpdateAndDisable(t *testing.T) {
	store := repository.NewMemory()
	handler := NewHandler(store)

	getRecorder := httptest.NewRecorder()
	handler.Item(getRecorder, httptest.NewRequest(http.MethodGet, "/api/v1/users/1", nil))
	if getRecorder.Code != http.StatusOK {
		t.Fatalf("expected get 200, got %d body=%s", getRecorder.Code, getRecorder.Body.String())
	}

	updateBody := `{"email":"admin@example.com","username":"admin","display_name":"Administrator","status":"active","source":"local","mfa_enabled":true}`
	updateRecorder := httptest.NewRecorder()
	handler.Item(updateRecorder, httptest.NewRequest(http.MethodPut, "/api/v1/users/1", strings.NewReader(updateBody)))
	if updateRecorder.Code != http.StatusOK {
		t.Fatalf("expected update 200, got %d body=%s", updateRecorder.Code, updateRecorder.Body.String())
	}
	var updateEnvelope struct {
		Data models.User `json:"data"`
	}
	if err := json.NewDecoder(updateRecorder.Body).Decode(&updateEnvelope); err != nil {
		t.Fatalf("decode update response: %v", err)
	}
	if updateEnvelope.Data.DisplayName != "Administrator" || !updateEnvelope.Data.MFAEnabled {
		t.Fatalf("expected updated user, got %+v", updateEnvelope.Data)
	}
	events, err := store.ListAuditEvents(context.Background(), repository.AuditLogFilter{Action: "update_user"})
	if err != nil {
		t.Fatalf("list audit events: %v", err)
	}
	if len(events) != 1 || events[0].ResourceType != "user" {
		t.Fatalf("expected update_user audit event, got %+v", events)
	}

	disableRecorder := httptest.NewRecorder()
	handler.Item(disableRecorder, httptest.NewRequest(http.MethodDelete, "/api/v1/users/1", nil))
	if disableRecorder.Code != http.StatusOK {
		t.Fatalf("expected delete/disable 200, got %d body=%s", disableRecorder.Code, disableRecorder.Body.String())
	}
	var disableEnvelope struct {
		Data models.User `json:"data"`
	}
	if err := json.NewDecoder(disableRecorder.Body).Decode(&disableEnvelope); err != nil {
		t.Fatalf("decode disable response: %v", err)
	}
	if disableEnvelope.Data.Status != models.UserStatusDisabled {
		t.Fatalf("expected disabled user, got %+v", disableEnvelope.Data)
	}
	events, err = store.ListAuditEvents(context.Background(), repository.AuditLogFilter{Action: "disable_user"})
	if err != nil {
		t.Fatalf("list audit events: %v", err)
	}
	if len(events) != 1 || events[0].ResourceType != "user" {
		t.Fatalf("expected disable_user audit event, got %+v", events)
	}
}

func TestUserUpdateRejectsInvalidStatus(t *testing.T) {
	handler := NewHandler(repository.NewMemory())
	body := `{"email":"admin@example.com","username":"admin","status":"pending"}`
	recorder := httptest.NewRecorder()

	handler.Item(recorder, httptest.NewRequest(http.MethodPut, "/api/v1/users/1", strings.NewReader(body)))

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", recorder.Code, recorder.Body.String())
	}
}
