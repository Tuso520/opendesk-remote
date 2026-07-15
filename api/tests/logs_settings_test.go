package tests

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/opendesk-remote/opendesk-remote/api/internal/app"
)

func TestLogsAndSettingsEndpoints(t *testing.T) {
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

	for _, endpoint := range []string{
		"/api/v1/logs/audit",
		"/api/v1/logs/connections",
		"/api/v1/logs/file-transfers",
		"/api/v1/logs/logins",
	} {
		resp, err := client.Get(server.URL + endpoint)
		if err != nil {
			t.Fatalf("GET %s failed: %v", endpoint, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected %s 200, got %d", endpoint, resp.StatusCode)
		}
	}

	logins, err := client.Get(server.URL + "/api/v1/logs/logins")
	if err != nil {
		t.Fatalf("login logs failed: %v", err)
	}
	defer logins.Body.Close()
	var loginEnvelope struct {
		Data []struct {
			UserID int64  `json:"user_id"`
			Email  string `json:"email"`
		} `json:"data"`
	}
	if err := json.NewDecoder(logins.Body).Decode(&loginEnvelope); err != nil {
		t.Fatalf("decode login logs: %v", err)
	}
	if len(loginEnvelope.Data) != 1 || loginEnvelope.Data[0].UserID != 1 || loginEnvelope.Data[0].Email != "admin@example.com" {
		t.Fatalf("unexpected login logs: %+v", loginEnvelope.Data)
	}

	filtered, err := client.Get(server.URL + "/api/v1/logs/logins?email=admin@example.com&limit=1")
	if err != nil {
		t.Fatalf("filtered login logs failed: %v", err)
	}
	defer filtered.Body.Close()
	if filtered.StatusCode != http.StatusOK {
		t.Fatalf("expected filtered login logs 200, got %d", filtered.StatusCode)
	}
	var filteredEnvelope struct {
		Data []struct {
			Email string `json:"email"`
		} `json:"data"`
	}
	if err := json.NewDecoder(filtered.Body).Decode(&filteredEnvelope); err != nil {
		t.Fatalf("decode filtered login logs: %v", err)
	}
	if len(filteredEnvelope.Data) != 1 || filteredEnvelope.Data[0].Email != "admin@example.com" {
		t.Fatalf("unexpected filtered login logs: %+v", filteredEnvelope.Data)
	}

	offsetFiltered, err := client.Get(server.URL + "/api/v1/logs/logins?offset=1&limit=1")
	if err != nil {
		t.Fatalf("offset login logs failed: %v", err)
	}
	defer offsetFiltered.Body.Close()
	if offsetFiltered.StatusCode != http.StatusOK {
		t.Fatalf("expected offset login logs 200, got %d", offsetFiltered.StatusCode)
	}
	var offsetEnvelope struct {
		Data []struct{} `json:"data"`
	}
	if err := json.NewDecoder(offsetFiltered.Body).Decode(&offsetEnvelope); err != nil {
		t.Fatalf("decode offset login logs: %v", err)
	}
	if len(offsetEnvelope.Data) != 0 {
		t.Fatalf("expected offset login logs to be empty, got %+v", offsetEnvelope.Data)
	}

	future := url.QueryEscape(time.Now().UTC().Add(time.Hour).Format(time.RFC3339))
	timeFiltered, err := client.Get(server.URL + "/api/v1/logs/logins?from=" + future)
	if err != nil {
		t.Fatalf("time filtered login logs failed: %v", err)
	}
	defer timeFiltered.Body.Close()
	if timeFiltered.StatusCode != http.StatusOK {
		t.Fatalf("expected time filtered login logs 200, got %d", timeFiltered.StatusCode)
	}
	var timeEnvelope struct {
		Data []struct{} `json:"data"`
	}
	if err := json.NewDecoder(timeFiltered.Body).Decode(&timeEnvelope); err != nil {
		t.Fatalf("decode time filtered login logs: %v", err)
	}
	if len(timeEnvelope.Data) != 0 {
		t.Fatalf("expected future login logs to be empty, got %+v", timeEnvelope.Data)
	}

	emptyFiltered, err := client.Get(server.URL + "/api/v1/logs/logins?email=missing@example.com")
	if err != nil {
		t.Fatalf("empty filtered login logs failed: %v", err)
	}
	defer emptyFiltered.Body.Close()
	var emptyEnvelope struct {
		Data []struct{} `json:"data"`
	}
	if err := json.NewDecoder(emptyFiltered.Body).Decode(&emptyEnvelope); err != nil {
		t.Fatalf("decode empty filtered login logs: %v", err)
	}
	if len(emptyEnvelope.Data) != 0 {
		t.Fatalf("expected no filtered login logs, got %+v", emptyEnvelope.Data)
	}

	settings := putJSON(t, client, server.URL+"/api/v1/settings", `{"settings":{"web_client_enabled":true,"audit_retention_days":365}}`)
	defer settings.Body.Close()
	if settings.StatusCode != http.StatusOK {
		t.Fatalf("expected settings update 200, got %d", settings.StatusCode)
	}
	var settingsEnvelope struct {
		Data []struct {
			Key    string `json:"key"`
			Value  any    `json:"value"`
			Source string `json:"source"`
		} `json:"data"`
	}
	if err := json.NewDecoder(settings.Body).Decode(&settingsEnvelope); err != nil {
		t.Fatalf("decode settings: %v", err)
	}
	foundWeb := false
	foundAudit := false
	for _, setting := range settingsEnvelope.Data {
		if setting.Key == "web_client_enabled" {
			foundWeb = setting.Value == true && setting.Source == "stored"
		}
		if setting.Key == "audit_retention_days" {
			foundAudit = setting.Value == float64(365) && setting.Source == "stored"
		}
	}
	if !foundWeb || !foundAudit {
		t.Fatalf("expected stored settings in response: %+v", settingsEnvelope.Data)
	}
}

func TestLoginLogsIncludeFailedAttempts(t *testing.T) {
	server := httptest.NewServer(app.NewRouter(testConfig(t), slog.Default()))
	defer server.Close()

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookie jar: %v", err)
	}
	client := &http.Client{Jar: jar}
	failed := postJSON(t, client, server.URL+"/api/v1/auth/login", `{"email":"admin@example.com","password":"wrong-password"}`)
	defer failed.Body.Close()
	if failed.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected failed login 401, got %d", failed.StatusCode)
	}
	login := postJSON(t, client, server.URL+"/api/v1/auth/login", `{"email":"admin@example.com","password":"admin-password-12345"}`)
	defer login.Body.Close()
	if login.StatusCode != http.StatusOK {
		t.Fatalf("expected login 200, got %d", login.StatusCode)
	}

	failedLogs, err := client.Get(server.URL + "/api/v1/logs/logins?status=failed&email=admin@example.com")
	if err != nil {
		t.Fatalf("failed login logs request failed: %v", err)
	}
	defer failedLogs.Body.Close()
	if failedLogs.StatusCode != http.StatusOK {
		t.Fatalf("expected failed login logs 200, got %d", failedLogs.StatusCode)
	}
	var failedEnvelope struct {
		Data []struct {
			Email         string `json:"email"`
			Status        string `json:"status"`
			FailureReason string `json:"failure_reason"`
			IP            string `json:"ip"`
			UserAgent     string `json:"user_agent"`
		} `json:"data"`
	}
	if err := json.NewDecoder(failedLogs.Body).Decode(&failedEnvelope); err != nil {
		t.Fatalf("decode failed login logs: %v", err)
	}
	if len(failedEnvelope.Data) != 1 {
		t.Fatalf("expected one failed login log, got %+v", failedEnvelope.Data)
	}
	got := failedEnvelope.Data[0]
	if got.Email != "admin@example.com" || got.Status != "failed" || got.FailureReason != "invalid_credentials" {
		t.Fatalf("unexpected failed login log: %+v", got)
	}
	if got.IP == "" || got.UserAgent == "" {
		t.Fatalf("expected failed login IP and user agent, got %+v", got)
	}

	invalid, err := client.Get(server.URL + "/api/v1/logs/logins?status=maybe")
	if err != nil {
		t.Fatalf("invalid login status request failed: %v", err)
	}
	defer invalid.Body.Close()
	if invalid.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected invalid status 400, got %d", invalid.StatusCode)
	}
}

func TestSettingsRejectInvalidInput(t *testing.T) {
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

	empty := putJSON(t, client, server.URL+"/api/v1/settings", `{"settings":{}}`)
	defer empty.Body.Close()
	if empty.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected empty settings 400, got %d", empty.StatusCode)
	}

	malformed := putJSON(t, client, server.URL+"/api/v1/settings", `{"settings":{"x":`)
	defer malformed.Body.Close()
	if malformed.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected malformed JSON 400, got %d", malformed.StatusCode)
	}
}

func TestLogFiltersRejectInvalidInput(t *testing.T) {
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

	invalidLimit, err := client.Get(server.URL + "/api/v1/logs/logins?limit=0")
	if err != nil {
		t.Fatalf("invalid limit request failed: %v", err)
	}
	defer invalidLimit.Body.Close()
	if invalidLimit.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected invalid limit 400, got %d", invalidLimit.StatusCode)
	}

	invalidStatus, err := client.Get(server.URL + "/api/v1/logs/connections?status=unknown")
	if err != nil {
		t.Fatalf("invalid status request failed: %v", err)
	}
	defer invalidStatus.Body.Close()
	if invalidStatus.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected invalid status 400, got %d", invalidStatus.StatusCode)
	}

	invalidOffset, err := client.Get(server.URL + "/api/v1/logs/audit?offset=-1")
	if err != nil {
		t.Fatalf("invalid offset request failed: %v", err)
	}
	defer invalidOffset.Body.Close()
	if invalidOffset.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected invalid offset 400, got %d", invalidOffset.StatusCode)
	}

	invalidTime, err := client.Get(server.URL + "/api/v1/logs/audit?from=not-a-time")
	if err != nil {
		t.Fatalf("invalid time request failed: %v", err)
	}
	defer invalidTime.Body.Close()
	if invalidTime.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected invalid time 400, got %d", invalidTime.StatusCode)
	}
}

func TestSettingsSections(t *testing.T) {
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

	for _, tc := range []struct {
		endpoint string
		body     string
		key      string
		want     any
	}{
		{endpoint: "/api/v1/settings/oidc", body: `{"settings":{"enabled":true,"issuer_url":"https://issuer.example.com","client_id":"opendesk","scopes":"openid email","auto_create_users":true}}`, key: "issuer_url", want: "https://issuer.example.com"},
		{endpoint: "/api/v1/settings/ldap", body: `{"settings":{"enabled":true,"url":"ldaps://ldap.example.com","base_dn":"dc=example,dc=com","bind_dn":"cn=opendesk,dc=example,dc=com","user_filter":"(uid={username})","group_filter":"(member={dn})","start_tls":false}}`, key: "url", want: "ldaps://ldap.example.com"},
		{endpoint: "/api/v1/settings/smtp", body: `{"settings":{"enabled":true,"host":"smtp.example.com","port":465,"username":"mailer","from_address":"noreply@example.com","tls":true}}`, key: "port", want: float64(465)},
	} {
		updated := putJSON(t, client, server.URL+tc.endpoint, tc.body)
		defer updated.Body.Close()
		if updated.StatusCode != http.StatusOK {
			t.Fatalf("expected %s update 200, got %d", tc.endpoint, updated.StatusCode)
		}
		var envelope struct {
			Data []struct {
				Key     string `json:"key"`
				Value   any    `json:"value"`
				Section string `json:"section"`
				Source  string `json:"source"`
			} `json:"data"`
		}
		if err := json.NewDecoder(updated.Body).Decode(&envelope); err != nil {
			t.Fatalf("decode %s: %v", tc.endpoint, err)
		}
		found := false
		for _, setting := range envelope.Data {
			if setting.Key == tc.key {
				found = setting.Value == tc.want && setting.Source == "stored" && setting.Section != ""
			}
		}
		if !found {
			t.Fatalf("expected stored %s in %s response: %+v", tc.key, tc.endpoint, envelope.Data)
		}

		listed, err := client.Get(server.URL + tc.endpoint)
		if err != nil {
			t.Fatalf("GET %s failed: %v", tc.endpoint, err)
		}
		defer listed.Body.Close()
		if listed.StatusCode != http.StatusOK {
			t.Fatalf("expected %s list 200, got %d", tc.endpoint, listed.StatusCode)
		}
	}

	unknown := putJSON(t, client, server.URL+"/api/v1/settings/oidc", `{"settings":{"host":"wrong-section"}}`)
	defer unknown.Body.Close()
	if unknown.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected unknown oidc key 400, got %d", unknown.StatusCode)
	}

	wrongType := putJSON(t, client, server.URL+"/api/v1/settings/smtp", `{"settings":{"port":"465"}}`)
	defer wrongType.Body.Close()
	if wrongType.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected wrong smtp port type 400, got %d", wrongType.StatusCode)
	}
}

func putJSON(t *testing.T, client *http.Client, url string, body string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPut, url, strings.NewReader(body))
	if err != nil {
		t.Fatalf("create PUT request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("PUT %s failed: %v", url, err)
	}
	return resp
}
