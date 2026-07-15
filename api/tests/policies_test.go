package tests

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"testing"

	"github.com/opendesk-remote/opendesk-remote/api/internal/app"
)

func TestPolicyManagementAPIs(t *testing.T) {
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

	accessRule := postJSON(t, client, server.URL+"/api/v1/access-rules", `{"subject_type":"user","subject_id":1,"target_type":"device","target_id":1,"effect":"allow","priority":100}`)
	defer accessRule.Body.Close()
	if accessRule.StatusCode != http.StatusCreated {
		t.Fatalf("expected access rule 201, got %d", accessRule.StatusCode)
	}
	var ruleEnvelope struct {
		Data struct {
			ID      int64  `json:"id"`
			Effect  string `json:"effect"`
			Enabled bool   `json:"enabled"`
		} `json:"data"`
	}
	if err := json.NewDecoder(accessRule.Body).Decode(&ruleEnvelope); err != nil {
		t.Fatalf("decode access rule: %v", err)
	}
	if ruleEnvelope.Data.ID == 0 || ruleEnvelope.Data.Effect != "allow" || !ruleEnvelope.Data.Enabled {
		t.Fatalf("unexpected access rule: %+v", ruleEnvelope.Data)
	}
	updateAccessRule := putJSON(t, client, server.URL+"/api/v1/access-rules/"+idString(ruleEnvelope.Data.ID), `{"subject_type":"user","subject_id":1,"target_type":"device","target_id":1,"effect":"deny","priority":200,"enabled":true}`)
	defer updateAccessRule.Body.Close()
	if updateAccessRule.StatusCode != http.StatusOK {
		t.Fatalf("expected access rule update 200, got %d", updateAccessRule.StatusCode)
	}
	var updatedRuleEnvelope struct {
		Data struct {
			ID       int64  `json:"id"`
			Effect   string `json:"effect"`
			Priority int    `json:"priority"`
			Enabled  bool   `json:"enabled"`
		} `json:"data"`
	}
	if err := json.NewDecoder(updateAccessRule.Body).Decode(&updatedRuleEnvelope); err != nil {
		t.Fatalf("decode updated access rule: %v", err)
	}
	if updatedRuleEnvelope.Data.ID != ruleEnvelope.Data.ID || updatedRuleEnvelope.Data.Effect != "deny" || updatedRuleEnvelope.Data.Priority != 200 {
		t.Fatalf("unexpected updated access rule: %+v", updatedRuleEnvelope.Data)
	}
	updateAccessRule = putJSON(t, client, server.URL+"/api/v1/access-rules/"+idString(ruleEnvelope.Data.ID), `{"subject_type":"user","subject_id":1,"target_type":"device","target_id":1,"effect":"allow","priority":100,"enabled":true}`)
	defer updateAccessRule.Body.Close()
	if updateAccessRule.StatusCode != http.StatusOK {
		t.Fatalf("expected access rule restore 200, got %d", updateAccessRule.StatusCode)
	}
	evaluate := postJSON(t, client, server.URL+"/api/v1/access/evaluate", `{"user":{"id":1,"status":"active"},"target":{"id":1,"status":"online"}}`)
	defer evaluate.Body.Close()
	if evaluate.StatusCode != http.StatusOK {
		t.Fatalf("expected access evaluate 200, got %d", evaluate.StatusCode)
	}
	var evaluationEnvelope struct {
		Data struct {
			Allowed bool   `json:"allowed"`
			Reason  string `json:"reason"`
		} `json:"data"`
	}
	if err := json.NewDecoder(evaluate.Body).Decode(&evaluationEnvelope); err != nil {
		t.Fatalf("decode access evaluation: %v", err)
	}
	if !evaluationEnvelope.Data.Allowed || evaluationEnvelope.Data.Reason != "matched_allow_rule" {
		t.Fatalf("unexpected access evaluation: %+v", evaluationEnvelope.Data)
	}

	controlRole := postJSON(t, client, server.URL+"/api/v1/control-roles", `{"name":"Support Operator","permissions":[{"permission_key":"file_transfer","mode":"disable"},{"permission_key":"audio","mode":"enable"}]}`)
	defer controlRole.Body.Close()
	if controlRole.StatusCode != http.StatusCreated {
		t.Fatalf("expected control role 201, got %d", controlRole.StatusCode)
	}
	var roleEnvelope struct {
		Data struct {
			ID          int64 `json:"id"`
			Enabled     bool  `json:"enabled"`
			Permissions []struct {
				PermissionKey string `json:"permission_key"`
				Mode          string `json:"mode"`
			} `json:"permissions"`
		} `json:"data"`
	}
	if err := json.NewDecoder(controlRole.Body).Decode(&roleEnvelope); err != nil {
		t.Fatalf("decode control role: %v", err)
	}
	if roleEnvelope.Data.ID == 0 || !roleEnvelope.Data.Enabled || len(roleEnvelope.Data.Permissions) != 2 {
		t.Fatalf("unexpected control role: %+v", roleEnvelope.Data)
	}
	updateControlRole := putJSON(t, client, server.URL+"/api/v1/control-roles/"+idString(roleEnvelope.Data.ID), `{"name":"Support Operator Updated","enabled":false,"permissions":[{"permission_key":"terminal","mode":"disable"}]}`)
	defer updateControlRole.Body.Close()
	if updateControlRole.StatusCode != http.StatusOK {
		t.Fatalf("expected control role update 200, got %d", updateControlRole.StatusCode)
	}
	var updatedRoleEnvelope struct {
		Data struct {
			ID          int64  `json:"id"`
			Name        string `json:"name"`
			Enabled     bool   `json:"enabled"`
			Permissions []struct {
				PermissionKey string `json:"permission_key"`
				Mode          string `json:"mode"`
			} `json:"permissions"`
		} `json:"data"`
	}
	if err := json.NewDecoder(updateControlRole.Body).Decode(&updatedRoleEnvelope); err != nil {
		t.Fatalf("decode updated control role: %v", err)
	}
	if updatedRoleEnvelope.Data.ID != roleEnvelope.Data.ID || updatedRoleEnvelope.Data.Name != "Support Operator Updated" || updatedRoleEnvelope.Data.Enabled || len(updatedRoleEnvelope.Data.Permissions) != 1 {
		t.Fatalf("unexpected updated control role: %+v", updatedRoleEnvelope.Data)
	}

	strategy := postJSON(t, client, server.URL+"/api/v1/strategies", `{"name":"Default Secure","settings_json":{"verification-method":"use-both-passwords","allow-remote-config-modification":"N"},"assignments":[{"target_type":"device","target_id":1}]}`)
	defer strategy.Body.Close()
	if strategy.StatusCode != http.StatusCreated {
		t.Fatalf("expected strategy 201, got %d", strategy.StatusCode)
	}
	var strategyEnvelope struct {
		Data struct {
			ID           int64  `json:"id"`
			SettingsJSON string `json:"settings_json"`
			Assignments  []struct {
				TargetType string `json:"target_type"`
				TargetID   int64  `json:"target_id"`
			} `json:"assignments"`
		} `json:"data"`
	}
	if err := json.NewDecoder(strategy.Body).Decode(&strategyEnvelope); err != nil {
		t.Fatalf("decode strategy: %v", err)
	}
	if strategyEnvelope.Data.ID == 0 || strategyEnvelope.Data.SettingsJSON == "" || len(strategyEnvelope.Data.Assignments) != 1 {
		t.Fatalf("unexpected strategy: %+v", strategyEnvelope.Data)
	}
	updateStrategy := putJSON(t, client, server.URL+"/api/v1/strategies/"+idString(strategyEnvelope.Data.ID), `{"name":"Default Secure Updated","enabled":true,"settings_json":{"verification-method":"temporary-password","allow-remote-config-modification":"N"},"assignments":[{"target_type":"user","target_id":1}]}`)
	defer updateStrategy.Body.Close()
	if updateStrategy.StatusCode != http.StatusOK {
		t.Fatalf("expected strategy update 200, got %d", updateStrategy.StatusCode)
	}
	var updatedStrategyEnvelope struct {
		Data struct {
			ID           int64  `json:"id"`
			Name         string `json:"name"`
			SettingsJSON string `json:"settings_json"`
			Assignments  []struct {
				TargetType string `json:"target_type"`
				TargetID   int64  `json:"target_id"`
			} `json:"assignments"`
		} `json:"data"`
	}
	if err := json.NewDecoder(updateStrategy.Body).Decode(&updatedStrategyEnvelope); err != nil {
		t.Fatalf("decode updated strategy: %v", err)
	}
	if updatedStrategyEnvelope.Data.ID != strategyEnvelope.Data.ID || updatedStrategyEnvelope.Data.Name != "Default Secure Updated" || len(updatedStrategyEnvelope.Data.Assignments) != 1 || updatedStrategyEnvelope.Data.Assignments[0].TargetType != "user" {
		t.Fatalf("unexpected updated strategy: %+v", updatedStrategyEnvelope.Data)
	}
	assignStrategy := postJSON(t, client, server.URL+"/api/v1/strategies/"+idString(strategyEnvelope.Data.ID)+"/assignments", `{"target_type":"device","target_id":1}`)
	defer assignStrategy.Body.Close()
	if assignStrategy.StatusCode != http.StatusCreated {
		t.Fatalf("expected strategy assignment 201, got %d", assignStrategy.StatusCode)
	}
	var assignedStrategyEnvelope struct {
		Data struct {
			Assignments []struct {
				ID         int64  `json:"id"`
				TargetType string `json:"target_type"`
				TargetID   int64  `json:"target_id"`
			} `json:"assignments"`
		} `json:"data"`
	}
	if err := json.NewDecoder(assignStrategy.Body).Decode(&assignedStrategyEnvelope); err != nil {
		t.Fatalf("decode assigned strategy: %v", err)
	}
	var deviceAssignmentID int64
	for _, assignment := range assignedStrategyEnvelope.Data.Assignments {
		if assignment.TargetType == "device" && assignment.TargetID == 1 {
			deviceAssignmentID = assignment.ID
			break
		}
	}
	if deviceAssignmentID == 0 {
		t.Fatalf("expected device strategy assignment, got %+v", assignedStrategyEnvelope.Data.Assignments)
	}
	deleteAssignment := deleteJSON(t, client, server.URL+"/api/v1/strategies/"+idString(strategyEnvelope.Data.ID)+"/assignments/"+idString(deviceAssignmentID))
	defer deleteAssignment.Body.Close()
	if deleteAssignment.StatusCode != http.StatusOK {
		t.Fatalf("expected strategy assignment delete 200, got %d", deleteAssignment.StatusCode)
	}
	effectiveStrategy, err := client.Get(server.URL + "/api/v1/devices/1/effective-strategy?user_id=1")
	if err != nil {
		t.Fatalf("get effective strategy: %v", err)
	}
	defer effectiveStrategy.Body.Close()
	if effectiveStrategy.StatusCode != http.StatusOK {
		t.Fatalf("expected effective strategy 200, got %d", effectiveStrategy.StatusCode)
	}
	var effectiveStrategyEnvelope struct {
		Data struct {
			Settings map[string]string `json:"settings"`
		} `json:"data"`
	}
	if err := json.NewDecoder(effectiveStrategy.Body).Decode(&effectiveStrategyEnvelope); err != nil {
		t.Fatalf("decode effective strategy: %v", err)
	}
	if effectiveStrategyEnvelope.Data.Settings["verification-method"] != "temporary-password" || effectiveStrategyEnvelope.Data.Settings["allow-remote-config-modification"] != "N" {
		t.Fatalf("unexpected effective strategy settings: %+v", effectiveStrategyEnvelope.Data.Settings)
	}

	for _, endpoint := range []string{"/api/v1/access-rules", "/api/v1/control-roles", "/api/v1/strategies"} {
		resp, err := client.Get(server.URL + endpoint)
		if err != nil {
			t.Fatalf("get %s: %v", endpoint, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected %s 200, got %d", endpoint, resp.StatusCode)
		}
	}

	deleteAccessRule := deleteJSON(t, client, server.URL+"/api/v1/access-rules/"+idString(ruleEnvelope.Data.ID))
	defer deleteAccessRule.Body.Close()
	if deleteAccessRule.StatusCode != http.StatusOK {
		t.Fatalf("expected access rule delete 200, got %d", deleteAccessRule.StatusCode)
	}
	deleteMissingAccessRule := deleteJSON(t, client, server.URL+"/api/v1/access-rules/"+idString(ruleEnvelope.Data.ID))
	defer deleteMissingAccessRule.Body.Close()
	if deleteMissingAccessRule.StatusCode != http.StatusNotFound {
		t.Fatalf("expected missing access rule delete 404, got %d", deleteMissingAccessRule.StatusCode)
	}

	deleteControlRole := deleteJSON(t, client, server.URL+"/api/v1/control-roles/"+idString(roleEnvelope.Data.ID))
	defer deleteControlRole.Body.Close()
	if deleteControlRole.StatusCode != http.StatusOK {
		t.Fatalf("expected control role delete 200, got %d", deleteControlRole.StatusCode)
	}
	deleteStrategy := deleteJSON(t, client, server.URL+"/api/v1/strategies/"+idString(strategyEnvelope.Data.ID))
	defer deleteStrategy.Body.Close()
	if deleteStrategy.StatusCode != http.StatusOK {
		t.Fatalf("expected strategy delete 200, got %d", deleteStrategy.StatusCode)
	}
}

func TestPolicyManagementRejectsInvalidValues(t *testing.T) {
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

	invalidRule := postJSON(t, client, server.URL+"/api/v1/access-rules", `{"subject_type":"user","subject_id":1,"target_type":"device","target_id":1,"effect":"maybe"}`)
	defer invalidRule.Body.Close()
	if invalidRule.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected invalid access rule 400, got %d", invalidRule.StatusCode)
	}
	invalidRuleUpdate := putJSON(t, client, server.URL+"/api/v1/access-rules/1", `{"subject_type":"user","subject_id":1,"target_type":"device","target_id":1,"effect":"maybe"}`)
	defer invalidRuleUpdate.Body.Close()
	if invalidRuleUpdate.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected invalid access rule update 400, got %d", invalidRuleUpdate.StatusCode)
	}
	invalidRole := postJSON(t, client, server.URL+"/api/v1/control-roles", `{"name":"Bad Role","permissions":[{"permission_key":"unknown","mode":"enable"}]}`)
	defer invalidRole.Body.Close()
	if invalidRole.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected invalid control role 400, got %d", invalidRole.StatusCode)
	}
	invalidRoleUpdate := putJSON(t, client, server.URL+"/api/v1/control-roles/1", `{"name":"Bad Role","permissions":[{"permission_key":"unknown","mode":"enable"}]}`)
	defer invalidRoleUpdate.Body.Close()
	if invalidRoleUpdate.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected invalid control role update 400, got %d", invalidRoleUpdate.StatusCode)
	}
	invalidStrategy := postJSON(t, client, server.URL+"/api/v1/strategies", `{"name":"Bad Strategy","settings_json":[]}`)
	defer invalidStrategy.Body.Close()
	if invalidStrategy.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected invalid strategy 400, got %d", invalidStrategy.StatusCode)
	}
	invalidStrategyUpdate := putJSON(t, client, server.URL+"/api/v1/strategies/1", `{"name":"Bad Strategy","settings_json":[]}`)
	defer invalidStrategyUpdate.Body.Close()
	if invalidStrategyUpdate.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected invalid strategy update 400, got %d", invalidStrategyUpdate.StatusCode)
	}
	invalidStrategyAssignment := postJSON(t, client, server.URL+"/api/v1/strategies/1/assignments", `{"target_type":"bad","target_id":1}`)
	defer invalidStrategyAssignment.Body.Close()
	if invalidStrategyAssignment.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected invalid strategy assignment 400, got %d", invalidStrategyAssignment.StatusCode)
	}
}
