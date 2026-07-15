package relays

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

func TestRelayHeartbeatUpdatesMemoryRelay(t *testing.T) {
	handler := NewHandler(repository.NewMemory())
	body := `{"current_sessions":7,"status":"degraded"}`
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/relays/1/heartbeat", strings.NewReader(body))

	handler.Item(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	var envelope struct {
		Data models.Relay `json:"data"`
	}
	if err := json.NewDecoder(recorder.Body).Decode(&envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if envelope.Data.CurrentSessions != 7 {
		t.Fatalf("expected sessions to update, got %+v", envelope.Data)
	}
	if envelope.Data.Status != "degraded" {
		t.Fatalf("expected degraded status, got %+v", envelope.Data)
	}
	if envelope.Data.LastHealthAt == nil {
		t.Fatalf("expected last_health_at to be set, got %+v", envelope.Data)
	}
}

func TestRelayHeartbeatRejectsInvalidID(t *testing.T) {
	handler := NewHandler(repository.NewMemory())
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/relays/not-a-number/heartbeat", strings.NewReader(`{}`))

	handler.Item(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestRelayHeartbeatRejectsUnknownRelay(t *testing.T) {
	handler := NewHandler(repository.NewMemory())
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/relays/999/heartbeat", strings.NewReader(`{}`))

	handler.Item(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestRelayUpdateChangesRelayAndWritesAudit(t *testing.T) {
	store := repository.NewMemory()
	handler := NewHandler(store)
	body := `{"name":"hbbr-relay-east","region":"us-east","host":"east.example.com","port":21117,"ws_port":21119,"status":"active","public_key_fingerprint":"fp"}`
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/api/v1/relays/1", strings.NewReader(body))

	handler.Item(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	var envelope struct {
		Data models.Relay `json:"data"`
	}
	if err := json.NewDecoder(recorder.Body).Decode(&envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if envelope.Data.Name != "hbbr-relay-east" || envelope.Data.Region != "us-east" || envelope.Data.Host != "east.example.com" {
		t.Fatalf("expected relay update, got %+v", envelope.Data)
	}
	events, err := store.ListAuditEvents(context.Background(), repository.AuditLogFilter{Action: "update_relay"})
	if err != nil {
		t.Fatalf("list audit events: %v", err)
	}
	if len(events) != 1 || events[0].ResourceType != "relay" {
		t.Fatalf("expected update_relay audit event, got %+v", events)
	}
}

func TestRelayUpdateRejectsInvalidStatus(t *testing.T) {
	handler := NewHandler(repository.NewMemory())
	body := `{"name":"relay","region":"region-a","host":"relay.example.com","status":"unknown"}`
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/api/v1/relays/1", strings.NewReader(body))

	handler.Item(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestRelayDisableUpdatesStatusAndWritesAudit(t *testing.T) {
	store := repository.NewMemory()
	handler := NewHandler(store)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/relays/1/disable", nil)

	handler.Item(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	var envelope struct {
		Data models.Relay `json:"data"`
	}
	if err := json.NewDecoder(recorder.Body).Decode(&envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if envelope.Data.Status != "disabled" {
		t.Fatalf("expected disabled relay, got %+v", envelope.Data)
	}
	events, err := store.ListAuditEvents(context.Background(), repository.AuditLogFilter{Action: "disable_relay"})
	if err != nil {
		t.Fatalf("list audit events: %v", err)
	}
	if len(events) != 1 || events[0].ResourceType != "relay" {
		t.Fatalf("expected disable_relay audit event, got %+v", events)
	}
}
