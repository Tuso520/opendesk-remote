package devices

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

func TestDeviceItemGetUpdateAndDisable(t *testing.T) {
	store := repository.NewMemory()
	handler := NewHandler(store)

	getRecorder := httptest.NewRecorder()
	handler.Item(getRecorder, httptest.NewRequest(http.MethodGet, "/api/v1/devices/1", nil))
	if getRecorder.Code != http.StatusOK {
		t.Fatalf("expected get 200, got %d body=%s", getRecorder.Code, getRecorder.Body.String())
	}

	updateBody := `{"rustdesk_id":"100000001","name":"Demo Updated","alias":"Desk","status":"offline","platform":"windows","client_version":"1.0.0","opendesk_client_version":"0.1.0","last_ip":"192.0.2.10"}`
	updateRecorder := httptest.NewRecorder()
	handler.Item(updateRecorder, httptest.NewRequest(http.MethodPut, "/api/v1/devices/1", strings.NewReader(updateBody)))
	if updateRecorder.Code != http.StatusOK {
		t.Fatalf("expected update 200, got %d body=%s", updateRecorder.Code, updateRecorder.Body.String())
	}
	var updateEnvelope struct {
		Data models.Device `json:"data"`
	}
	if err := json.NewDecoder(updateRecorder.Body).Decode(&updateEnvelope); err != nil {
		t.Fatalf("decode update response: %v", err)
	}
	if updateEnvelope.Data.Name != "Demo Updated" || updateEnvelope.Data.Alias != "Desk" {
		t.Fatalf("expected updated device, got %+v", updateEnvelope.Data)
	}
	events, err := store.ListAuditEvents(context.Background(), repository.AuditLogFilter{Action: "update_device"})
	if err != nil {
		t.Fatalf("list audit events: %v", err)
	}
	if len(events) != 1 || events[0].ResourceType != "device" {
		t.Fatalf("expected update_device audit event, got %+v", events)
	}

	disableRecorder := httptest.NewRecorder()
	handler.Item(disableRecorder, httptest.NewRequest(http.MethodPost, "/api/v1/devices/1/disable", nil))
	if disableRecorder.Code != http.StatusOK {
		t.Fatalf("expected disable 200, got %d body=%s", disableRecorder.Code, disableRecorder.Body.String())
	}
	var disableEnvelope struct {
		Data models.Device `json:"data"`
	}
	if err := json.NewDecoder(disableRecorder.Body).Decode(&disableEnvelope); err != nil {
		t.Fatalf("decode disable response: %v", err)
	}
	if disableEnvelope.Data.Status != models.DeviceStatusDisabled {
		t.Fatalf("expected disabled device, got %+v", disableEnvelope.Data)
	}
	events, err = store.ListAuditEvents(context.Background(), repository.AuditLogFilter{Action: "disable_device"})
	if err != nil {
		t.Fatalf("list audit events: %v", err)
	}
	if len(events) != 1 || events[0].ResourceType != "device" {
		t.Fatalf("expected disable_device audit event, got %+v", events)
	}
}

func TestDeviceUpdateRejectsInvalidStatus(t *testing.T) {
	handler := NewHandler(repository.NewMemory())
	body := `{"rustdesk_id":"100000001","name":"Demo","status":"sleeping"}`
	recorder := httptest.NewRecorder()

	handler.Item(recorder, httptest.NewRequest(http.MethodPut, "/api/v1/devices/1", strings.NewReader(body)))

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", recorder.Code, recorder.Body.String())
	}
}
