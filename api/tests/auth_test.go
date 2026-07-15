package tests

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/opendesk-remote/opendesk-remote/api/internal/app"
	"github.com/opendesk-remote/opendesk-remote/api/internal/config"
)

func TestAuthFlowProtectsManagementEndpoints(t *testing.T) {
	server := httptest.NewServer(app.NewRouter(testConfig(t), slog.Default()))
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/v1/users")
	if err != nil {
		t.Fatalf("users request failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected unauthenticated users request to return 401, got %d", resp.StatusCode)
	}

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookie jar: %v", err)
	}
	client := &http.Client{Jar: jar}
	loginResp := postJSON(t, client, server.URL+"/api/v1/auth/login", `{"email":"admin@example.com","password":"admin-password-12345"}`)
	defer loginResp.Body.Close()
	if loginResp.StatusCode != http.StatusOK {
		t.Fatalf("expected login 200, got %d", loginResp.StatusCode)
	}
	var loginEnvelope struct {
		Data struct {
			AccessToken string `json:"access_token"`
		} `json:"data"`
	}
	if err := json.NewDecoder(loginResp.Body).Decode(&loginEnvelope); err != nil {
		t.Fatalf("decode login response: %v", err)
	}
	if loginEnvelope.Data.AccessToken == "" {
		t.Fatal("expected login access token")
	}
	if cookies := jar.Cookies(loginResp.Request.URL); len(cookies) == 0 {
		t.Fatal("expected session cookie after login")
	}

	usersResp, err := client.Get(server.URL + "/api/v1/users")
	if err != nil {
		t.Fatalf("authenticated users request failed: %v", err)
	}
	defer usersResp.Body.Close()
	if usersResp.StatusCode != http.StatusOK {
		t.Fatalf("expected authenticated users request 200, got %d", usersResp.StatusCode)
	}

	logoutResp := postJSON(t, client, server.URL+"/api/v1/auth/logout", `{}`)
	defer logoutResp.Body.Close()
	if logoutResp.StatusCode != http.StatusOK {
		t.Fatalf("expected logout 200, got %d", logoutResp.StatusCode)
	}
	bearerReq, err := http.NewRequest(http.MethodGet, server.URL+"/api/v1/users", nil)
	if err != nil {
		t.Fatalf("new bearer request: %v", err)
	}
	bearerReq.Header.Set("Authorization", "Bearer "+loginEnvelope.Data.AccessToken)
	revokedResp, err := http.DefaultClient.Do(bearerReq)
	if err != nil {
		t.Fatalf("revoked bearer request failed: %v", err)
	}
	defer revokedResp.Body.Close()
	if revokedResp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected revoked session bearer to return 401, got %d", revokedResp.StatusCode)
	}
}

func TestUserItemRoutesRequireAuthentication(t *testing.T) {
	server := httptest.NewServer(app.NewRouter(testConfig(t), slog.Default()))
	defer server.Close()

	unauth, err := http.Get(server.URL + "/api/v1/users/1")
	if err != nil {
		t.Fatalf("unauthenticated user get failed: %v", err)
	}
	unauth.Body.Close()
	if unauth.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected unauthenticated user get 401, got %d", unauth.StatusCode)
	}

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookie jar: %v", err)
	}
	client := &http.Client{Jar: jar}
	login := postJSON(t, client, server.URL+"/api/v1/auth/login", `{"email":"admin@example.com","password":"admin-password-12345"}`)
	defer login.Body.Close()
	if login.StatusCode != http.StatusOK {
		t.Fatalf("expected login 200, got %d", login.StatusCode)
	}

	userResp, err := client.Get(server.URL + "/api/v1/users/1")
	if err != nil {
		t.Fatalf("authenticated user get failed: %v", err)
	}
	defer userResp.Body.Close()
	if userResp.StatusCode != http.StatusOK {
		t.Fatalf("expected authenticated user get 200, got %d", userResp.StatusCode)
	}

	disabled := deleteJSON(t, client, server.URL+"/api/v1/users/1")
	defer disabled.Body.Close()
	if disabled.StatusCode != http.StatusOK {
		t.Fatalf("expected authenticated user delete/disable 200, got %d", disabled.StatusCode)
	}
}

func TestDeviceItemRoutesRequireAuthentication(t *testing.T) {
	server := httptest.NewServer(app.NewRouter(testConfig(t), slog.Default()))
	defer server.Close()

	unauth, err := http.Get(server.URL + "/api/v1/devices/1")
	if err != nil {
		t.Fatalf("unauthenticated device get failed: %v", err)
	}
	unauth.Body.Close()
	if unauth.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected unauthenticated device get 401, got %d", unauth.StatusCode)
	}
	unauthRegister := postJSON(t, http.DefaultClient, server.URL+"/api/v1/devices/register", `{"rustdesk_id":"200000002","name":"Registered Device"}`)
	defer unauthRegister.Body.Close()
	if unauthRegister.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected unauthenticated device register 401, got %d", unauthRegister.StatusCode)
	}

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookie jar: %v", err)
	}
	client := &http.Client{Jar: jar}
	login := postJSON(t, client, server.URL+"/api/v1/auth/login", `{"email":"admin@example.com","password":"admin-password-12345"}`)
	defer login.Body.Close()
	if login.StatusCode != http.StatusOK {
		t.Fatalf("expected login 200, got %d", login.StatusCode)
	}

	deviceResp, err := client.Get(server.URL + "/api/v1/devices/1")
	if err != nil {
		t.Fatalf("authenticated device get failed: %v", err)
	}
	defer deviceResp.Body.Close()
	if deviceResp.StatusCode != http.StatusOK {
		t.Fatalf("expected authenticated device get 200, got %d", deviceResp.StatusCode)
	}
	registerResp := postJSON(t, client, server.URL+"/api/v1/devices/register", `{"rustdesk_id":"200000002","name":"Registered Device","platform":"windows"}`)
	defer registerResp.Body.Close()
	if registerResp.StatusCode != http.StatusCreated {
		t.Fatalf("expected authenticated device register 201, got %d", registerResp.StatusCode)
	}

	disabled := postJSON(t, client, server.URL+"/api/v1/devices/1/disable", `{}`)
	defer disabled.Body.Close()
	if disabled.StatusCode != http.StatusOK {
		t.Fatalf("expected authenticated device disable 200, got %d", disabled.StatusCode)
	}
}

func TestRelayGrantIssueRequiresSessionAndValidateIsPublic(t *testing.T) {
	server := httptest.NewServer(app.NewRouter(testConfig(t), slog.Default()))
	defer server.Close()

	unauth := postJSON(t, http.DefaultClient, server.URL+"/api/v1/relay-grants", relayGrantBody())
	defer unauth.Body.Close()
	if unauth.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected unauthenticated relay grant issue to return 401, got %d", unauth.StatusCode)
	}

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookie jar: %v", err)
	}
	client := &http.Client{Jar: jar}
	login := postJSON(t, client, server.URL+"/api/v1/auth/login", `{"email":"admin@example.com","password":"admin-password-12345"}`)
	defer login.Body.Close()
	if login.StatusCode != http.StatusOK {
		t.Fatalf("expected login 200, got %d", login.StatusCode)
	}

	issued := postJSON(t, client, server.URL+"/api/v1/relay-grants", relayGrantBody())
	defer issued.Body.Close()
	if issued.StatusCode != http.StatusCreated {
		t.Fatalf("expected relay grant issue 201, got %d", issued.StatusCode)
	}
	var issueEnvelope struct {
		Data struct {
			Token string `json:"grant_token"`
		} `json:"data"`
	}
	if err := json.NewDecoder(issued.Body).Decode(&issueEnvelope); err != nil {
		t.Fatalf("decode issue response: %v", err)
	}
	if issueEnvelope.Data.Token == "" {
		t.Fatal("expected relay grant token")
	}

	validateBody := `{"grant_token":` + strconvQuote(issueEnvelope.Data.Token) + `,"relay":"relay-a","target_rustdesk_id":"100000001"}`
	validated := postJSON(t, http.DefaultClient, server.URL+"/api/v1/relay-grants/validate", validateBody)
	defer validated.Body.Close()
	if validated.StatusCode != http.StatusOK {
		t.Fatalf("expected public relay grant validate 200, got %d", validated.StatusCode)
	}
}

func TestRelayGrantValidateFailureWritesConnectionLog(t *testing.T) {
	server := httptest.NewServer(app.NewRouter(testConfig(t), slog.Default()))
	defer server.Close()

	denied := postJSON(t, http.DefaultClient, server.URL+"/api/v1/relay-grants/validate", `{"relay":"relay-a","target_device_id":1}`)
	defer denied.Body.Close()
	if denied.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected missing grant validate 401, got %d", denied.StatusCode)
	}

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookie jar: %v", err)
	}
	client := &http.Client{Jar: jar}
	login := postJSON(t, client, server.URL+"/api/v1/auth/login", `{"email":"admin@example.com","password":"admin-password-12345"}`)
	defer login.Body.Close()
	if login.StatusCode != http.StatusOK {
		t.Fatalf("expected login 200, got %d", login.StatusCode)
	}
	logsResp, err := client.Get(server.URL + "/api/v1/logs/connections?status=denied&connection_type=relay")
	if err != nil {
		t.Fatalf("connection logs request failed: %v", err)
	}
	defer logsResp.Body.Close()
	if logsResp.StatusCode != http.StatusOK {
		t.Fatalf("expected connection logs 200, got %d", logsResp.StatusCode)
	}
	var envelope struct {
		Data []struct {
			ConnectionType string `json:"connection_type"`
			Status         string `json:"status"`
			DenyReason     string `json:"deny_reason"`
		} `json:"data"`
	}
	if err := json.NewDecoder(logsResp.Body).Decode(&envelope); err != nil {
		t.Fatalf("decode connection logs: %v", err)
	}
	if len(envelope.Data) != 1 {
		t.Fatalf("expected one relay denial log, got %+v", envelope.Data)
	}
	got := envelope.Data[0]
	if got.ConnectionType != "relay" || got.Status != "denied" || got.DenyReason != "relay_auth_required" {
		t.Fatalf("unexpected relay denial log: %+v", got)
	}
}

func TestAPITokenCanIssueRelayGrant(t *testing.T) {
	server := httptest.NewServer(app.NewRouter(testConfig(t), slog.Default()))
	defer server.Close()

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookie jar: %v", err)
	}
	client := &http.Client{Jar: jar}
	login := postJSON(t, client, server.URL+"/api/v1/auth/login", `{"email":"admin@example.com","password":"admin-password-12345"}`)
	defer login.Body.Close()
	if login.StatusCode != http.StatusOK {
		t.Fatalf("expected login 200, got %d", login.StatusCode)
	}
	created := postJSON(t, client, server.URL+"/api/v1/api-tokens", `{"name":"relay automation","scopes":["relay:grant"]}`)
	defer created.Body.Close()
	if created.StatusCode != http.StatusCreated {
		t.Fatalf("expected API token create 201, got %d", created.StatusCode)
	}
	var tokenEnvelope struct {
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	if err := json.NewDecoder(created.Body).Decode(&tokenEnvelope); err != nil {
		t.Fatalf("decode API token response: %v", err)
	}
	if tokenEnvelope.Data.Token == "" {
		t.Fatal("expected raw API token")
	}

	issued := postJSONBearer(t, server.URL+"/api/v1/relay-grants", relayGrantBody(), tokenEnvelope.Data.Token)
	defer issued.Body.Close()
	if issued.StatusCode != http.StatusCreated {
		t.Fatalf("expected API-token-authenticated relay grant issue 201, got %d", issued.StatusCode)
	}
}

func TestRelayDisableRequiresAuthentication(t *testing.T) {
	server := httptest.NewServer(app.NewRouter(testConfig(t), slog.Default()))
	defer server.Close()

	unauth := postJSON(t, http.DefaultClient, server.URL+"/api/v1/relays/1/disable", `{}`)
	defer unauth.Body.Close()
	if unauth.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected unauthenticated relay disable to return 401, got %d", unauth.StatusCode)
	}

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookie jar: %v", err)
	}
	client := &http.Client{Jar: jar}
	login := postJSON(t, client, server.URL+"/api/v1/auth/login", `{"email":"admin@example.com","password":"admin-password-12345"}`)
	defer login.Body.Close()
	if login.StatusCode != http.StatusOK {
		t.Fatalf("expected login 200, got %d", login.StatusCode)
	}

	disabled := postJSON(t, client, server.URL+"/api/v1/relays/1/disable", `{}`)
	defer disabled.Body.Close()
	if disabled.StatusCode != http.StatusOK {
		t.Fatalf("expected authenticated relay disable 200, got %d", disabled.StatusCode)
	}
}

func TestRelayUpdateRequiresAuthentication(t *testing.T) {
	server := httptest.NewServer(app.NewRouter(testConfig(t), slog.Default()))
	defer server.Close()
	body := `{"name":"hbbr-relay-east","region":"region-a","host":"relay.example.com","status":"active"}`

	unauth := putJSON(t, http.DefaultClient, server.URL+"/api/v1/relays/1", body)
	defer unauth.Body.Close()
	if unauth.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected unauthenticated relay update to return 401, got %d", unauth.StatusCode)
	}

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookie jar: %v", err)
	}
	client := &http.Client{Jar: jar}
	login := postJSON(t, client, server.URL+"/api/v1/auth/login", `{"email":"admin@example.com","password":"admin-password-12345"}`)
	defer login.Body.Close()
	if login.StatusCode != http.StatusOK {
		t.Fatalf("expected login 200, got %d", login.StatusCode)
	}

	updated := putJSON(t, client, server.URL+"/api/v1/relays/1", body)
	defer updated.Body.Close()
	if updated.StatusCode != http.StatusOK {
		t.Fatalf("expected authenticated relay update 200, got %d", updated.StatusCode)
	}
}

func testConfig(t *testing.T) config.Config {
	t.Helper()
	return config.Config{
		HTTPAddr:              ":0",
		JWTSecret:             "test-session-signing-key-with-enough-length",
		RelayGrantSigningKey:  "test-relay-grant-signing-key",
		RelayAuthRequired:     true,
		InitialAdminEmail:     "admin@example.com",
		InitialAdminPassword:  "admin-password-12345",
		AuthTokenTTL:          15 * time.Minute,
		RelayGrantTTL:         2 * time.Minute,
		AllowedCORSOrigins:    []string{"http://localhost:5173", "http://127.0.0.1:5173"},
		BrandingAssetMaxBytes: 1024,
		StorageDriver:         "local",
		StorageLocalPath:      t.TempDir(),
	}
}

func postJSON(t *testing.T, client *http.Client, url string, body string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, url, strings.NewReader(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("post %s failed: %v", url, err)
	}
	return resp
}

func deleteJSON(t *testing.T, client *http.Client, url string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		t.Fatalf("new delete request: %v", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("delete %s failed: %v", url, err)
	}
	return resp
}

func postJSONBearer(t *testing.T, url string, body string, token string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, url, strings.NewReader(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("post %s failed: %v", url, err)
	}
	return resp
}

func relayGrantBody() string {
	return `{"target_rustdesk_id":"100000001","allowed_relays":["relay-a"],"ttl_seconds":60}`
}

func strconvQuote(value string) string {
	var buf bytes.Buffer
	_ = json.NewEncoder(&buf).Encode(value)
	return strings.TrimSpace(buf.String())
}
