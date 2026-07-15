package apitokens

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	apiauth "github.com/opendesk-remote/opendesk-remote/api/internal/auth"
	"github.com/opendesk-remote/opendesk-remote/api/internal/models"
	"github.com/opendesk-remote/opendesk-remote/api/internal/repository"
)

func TestCreateAPITokenReturnsRawTokenOnceAndWritesAudit(t *testing.T) {
	store := repository.NewMemory()
	handler := NewHandler(store, "test-session-signing-key")
	request := jsonRequest(t, http.MethodPost, "/api/v1/api-tokens", CreateRequest{
		Name:   "relay automation",
		Scopes: []string{"relay:grant"},
	})
	request = request.WithContext(apiauth.WithSession(request.Context(), testSession()))
	response := httptest.NewRecorder()

	handler.Collection(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", response.Code, response.Body.String())
	}
	var body struct {
		Data CreateResponse `json:"data"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !strings.HasPrefix(body.Data.Token, "odrt_") {
		t.Fatalf("expected raw token prefix, got %q", body.Data.Token)
	}
	stored, err := store.ListAPITokens(context.Background())
	if err != nil {
		t.Fatalf("list tokens: %v", err)
	}
	if len(stored) != 1 {
		t.Fatalf("expected one stored token, got %d", len(stored))
	}
	if stored[0].TokenHash == body.Data.Token || strings.Contains(stored[0].TokenHash, "odrt_") {
		t.Fatalf("stored token must be hashed, got %q", stored[0].TokenHash)
	}
	events, err := store.ListAuditEvents(context.Background(), repository.AuditLogFilter{Action: "create_api_token"})
	if err != nil {
		t.Fatalf("list audit events: %v", err)
	}
	if len(events) != 1 || events[0].ResourceType != "api_token" {
		t.Fatalf("expected create_api_token audit event, got %+v", events)
	}
}

func TestAPITokenAuthenticatesAndRevocationRejectsIt(t *testing.T) {
	store := repository.NewMemory()
	handler := NewHandler(store, "test-session-signing-key")
	createRequest := jsonRequest(t, http.MethodPost, "/api/v1/api-tokens", CreateRequest{
		Name:   "relay automation",
		Scopes: []string{"relay:grant"},
	})
	createRequest = createRequest.WithContext(apiauth.WithSession(createRequest.Context(), testSession()))
	createResponse := httptest.NewRecorder()
	handler.Collection(createResponse, createRequest)
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", createResponse.Code, createResponse.Body.String())
	}
	var body struct {
		Data CreateResponse `json:"data"`
	}
	if err := json.NewDecoder(createResponse.Body).Decode(&body); err != nil {
		t.Fatalf("decode create response: %v", err)
	}

	service := apiauth.NewService(store, apiauth.Config{SigningKey: []byte("test-session-signing-key")})
	authRequest := httptest.NewRequest(http.MethodPost, "/api/v1/relay-grants", nil)
	authRequest.Header.Set("Authorization", "Bearer "+body.Data.Token)
	session, err := service.AuthenticateRequest(authRequest)
	if err != nil {
		t.Fatalf("authenticate API token: %v", err)
	}
	if session.ActorType != "api_token" || session.APIToken == nil {
		t.Fatalf("expected API token actor session, got %+v", session)
	}

	revokeRequest := httptest.NewRequest(http.MethodPost, "/api/v1/api-tokens/"+strconvID(body.Data.ID)+"/revoke", nil)
	revokeRequest = revokeRequest.WithContext(apiauth.WithSession(revokeRequest.Context(), testSession()))
	revokeResponse := httptest.NewRecorder()
	handler.Item(revokeResponse, revokeRequest)
	if revokeResponse.Code != http.StatusOK {
		t.Fatalf("expected revoke 200, got %d: %s", revokeResponse.Code, revokeResponse.Body.String())
	}
	if _, err := service.AuthenticateRequest(authRequest); err == nil {
		t.Fatal("expected revoked API token to be rejected")
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

func testSession() apiauth.Session {
	return apiauth.Session{
		User: models.User{
			ID:     1,
			Email:  "admin@example.com",
			Status: models.UserStatusActive,
		},
		ActorType: "user",
	}
}

func strconvID(id int64) string {
	return strconv.FormatInt(id, 10)
}
