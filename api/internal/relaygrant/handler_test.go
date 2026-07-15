package relaygrant

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/opendesk-remote/opendesk-remote/api/internal/audit"
	"github.com/opendesk-remote/opendesk-remote/api/internal/repository"
)

func TestHandlerIssueWritesAuditEvent(t *testing.T) {
	store := repository.NewMemory()
	handler := NewHandler(NewService([]byte("test-signing-key"), time.Minute), audit.RepositoryWriter{Repo: store})
	userID := int64(1)
	targetID := int64(20)
	request := jsonRequest(t, http.MethodPost, "/api/v1/relay-grants", IssueRequest{
		UserID:         &userID,
		TargetDeviceID: &targetID,
		AllowedRelays:  []string{"relay-a"},
		TTLSeconds:     60,
	})
	response := httptest.NewRecorder()

	handler.Issue(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", response.Code, response.Body.String())
	}
	events, err := store.ListAuditEvents(context.Background(), repository.AuditLogFilter{Action: "issue_relay_grant"})
	if err != nil {
		t.Fatalf("list audit events: %v", err)
	}
	if len(events) != 1 || events[0].ResourceType != "relay_grant" || events[0].ResourceID == nil {
		t.Fatalf("expected relay grant audit event, got %+v", events)
	}
}

func TestHandlerValidateFailureWritesAuditEvent(t *testing.T) {
	store := repository.NewMemory()
	service := NewService([]byte("test-signing-key"), time.Minute)
	handler := NewHandler(service, audit.RepositoryWriter{Repo: store}).WithConnectionLogWriter(store)
	userID := int64(1)
	targetID := int64(20)
	issued, err := service.Issue(IssueRequest{UserID: &userID, TargetDeviceID: &targetID, AllowedRelays: []string{"relay-a"}, TTLSeconds: 60})
	if err != nil {
		t.Fatalf("issue grant: %v", err)
	}
	request := jsonRequest(t, http.MethodPost, "/api/v1/relay-grants/validate", ValidateRequest{
		Token: issued.Token,
		Relay: "relay-b",
	})
	response := httptest.NewRecorder()

	handler.Validate(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", response.Code, response.Body.String())
	}
	events, err := store.ListAuditEvents(context.Background(), repository.AuditLogFilter{Action: "validate_relay_grant_failed"})
	if err != nil {
		t.Fatalf("list audit events: %v", err)
	}
	if len(events) != 1 || events[0].Metadata["reason"] != "relay_not_allowed" {
		t.Fatalf("expected validation failure audit event, got %+v", events)
	}
	logs, err := store.ListConnectionLogs(context.Background(), repository.ConnectionLogFilter{Status: "denied", ConnectionType: "relay"})
	if err != nil {
		t.Fatalf("list connection logs: %v", err)
	}
	if len(logs) != 1 || logs[0].DenyReason != "invalid_relay_grant" || logs[0].Metadata["reason"] != "relay_not_allowed" {
		t.Fatalf("expected relay denial connection log, got %+v", logs)
	}
}

func TestHandlerValidateMissingGrantWritesRelayAuthRequiredConnectionLog(t *testing.T) {
	store := repository.NewMemory()
	handler := NewHandler(NewService([]byte("test-signing-key"), time.Minute), audit.RepositoryWriter{Repo: store}).WithConnectionLogWriter(store)
	targetID := int64(20)
	request := jsonRequest(t, http.MethodPost, "/api/v1/relay-grants/validate", ValidateRequest{
		TargetDeviceID: &targetID,
		Relay:          "relay-a",
	})
	response := httptest.NewRecorder()

	handler.Validate(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", response.Code, response.Body.String())
	}
	logs, err := store.ListConnectionLogs(context.Background(), repository.ConnectionLogFilter{Status: "denied", ConnectionType: "relay"})
	if err != nil {
		t.Fatalf("list connection logs: %v", err)
	}
	if len(logs) != 1 {
		t.Fatalf("expected one relay denial log, got %+v", logs)
	}
	if logs[0].DenyReason != "relay_auth_required" || logs[0].TargetDeviceID == nil || *logs[0].TargetDeviceID != targetID {
		t.Fatalf("unexpected missing-grant denial log: %+v", logs[0])
	}
}

func jsonRequest(t *testing.T, method string, path string, body any) *http.Request {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	request := httptest.NewRequest(method, path, bytes.NewReader(raw))
	request.Header.Set("Content-Type", "application/json")
	return request
}
