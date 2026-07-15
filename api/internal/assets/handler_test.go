package assets

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/opendesk-remote/opendesk-remote/api/internal/audit"
	"github.com/opendesk-remote/opendesk-remote/api/internal/repository"
	"github.com/opendesk-remote/opendesk-remote/api/internal/storage"
)

func TestBrandingUploadStoresServerGeneratedAsset(t *testing.T) {
	root := t.TempDir()
	handler := NewHandler(storage.LocalStore{Root: root}, storage.AssetValidator{MaxBytes: 1024})
	request := multipartRequest(t, "logo.png", []byte("png-data"))
	response := httptest.NewRecorder()

	handler.Branding(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", response.Code, response.Body.String())
	}
	var body struct {
		Data BrandingAssetResponse `json:"data"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Data.ID == "" || body.Data.SHA256 == "" || body.Data.Size != int64(len("png-data")) {
		t.Fatalf("unexpected upload response: %+v", body.Data)
	}
	if strings.Contains(body.Data.Filename, "logo") || !strings.HasSuffix(body.Data.Filename, ".png") {
		t.Fatalf("filename should be server-generated with original extension: %q", body.Data.Filename)
	}
	savedPath := filepath.Join(root, "branding", body.Data.Filename)
	if _, err := os.Stat(savedPath); err != nil {
		t.Fatalf("expected asset saved at %s: %v", savedPath, err)
	}
}

func TestBrandingUploadRejectsExecutable(t *testing.T) {
	handler := NewHandler(storage.LocalStore{Root: t.TempDir()}, storage.AssetValidator{MaxBytes: 1024})
	request := multipartRequest(t, "payload.exe", []byte("MZ"))
	response := httptest.NewRecorder()

	handler.Branding(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), "executable upload rejected") {
		t.Fatalf("expected executable rejection, got %s", response.Body.String())
	}
}

func TestBrandingUploadWritesAuditEvent(t *testing.T) {
	root := t.TempDir()
	store := repository.NewMemory()
	handler := NewHandler(
		storage.LocalStore{Root: root},
		storage.AssetValidator{MaxBytes: 1024},
		audit.RepositoryWriter{Repo: store},
	)
	request := multipartRequest(t, "logo.png", []byte("png-data"))
	request.RemoteAddr = "192.0.2.10:12345"
	response := httptest.NewRecorder()

	handler.Branding(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", response.Code, response.Body.String())
	}
	events, err := store.ListAuditEvents(context.Background(), repository.AuditLogFilter{Action: "upload_branding_asset"})
	if err != nil {
		t.Fatalf("list audit events: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected one audit event, got %d", len(events))
	}
	if events[0].ActorType != "system" || events[0].ResourceType != "branding_asset" || events[0].ResourceID == nil {
		t.Fatalf("unexpected audit event: %+v", events[0])
	}
	if events[0].IP != "192.0.2.10" {
		t.Fatalf("expected remote address IP, got %q", events[0].IP)
	}
	if events[0].Metadata["sha256"] == "" || events[0].Metadata["filename"] == "" {
		t.Fatalf("expected asset metadata in audit event: %+v", events[0].Metadata)
	}
}

func TestBrandingUploadRejectsTraversalName(t *testing.T) {
	handler := NewHandler(storage.LocalStore{Root: t.TempDir()}, storage.AssetValidator{MaxBytes: 1024})
	request := multipartRequest(t, "../logo.png", []byte("png-data"))
	response := httptest.NewRecorder()

	handler.Branding(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("expected sanitized filename accepted with server-generated name, got %d: %s", response.Code, response.Body.String())
	}
	if strings.Contains(response.Body.String(), "..") {
		t.Fatalf("response must not trust traversal filename: %s", response.Body.String())
	}
}

func multipartRequest(t *testing.T, filename string, content []byte) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("create multipart file: %v", err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatalf("write multipart file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/assets/branding", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	return request
}
