package policies

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/opendesk-remote/opendesk-remote/api/internal/access"
	"github.com/opendesk-remote/opendesk-remote/api/internal/audit"
	apiauth "github.com/opendesk-remote/opendesk-remote/api/internal/auth"
	"github.com/opendesk-remote/opendesk-remote/api/internal/controlrole"
	"github.com/opendesk-remote/opendesk-remote/api/internal/httpx"
	"github.com/opendesk-remote/opendesk-remote/api/internal/models"
	"github.com/opendesk-remote/opendesk-remote/api/internal/repository"
)

type Handler struct {
	repo Repository
}

type Repository interface {
	repository.PolicyRepository
	audit.Repository
}

type CreateAccessRuleRequest struct {
	SubjectType string `json:"subject_type"`
	SubjectID   int64  `json:"subject_id"`
	TargetType  string `json:"target_type"`
	TargetID    int64  `json:"target_id"`
	Effect      string `json:"effect"`
	Priority    int    `json:"priority"`
	Enabled     *bool  `json:"enabled,omitempty"`
}

type CreateControlRoleRequest struct {
	Name        string                         `json:"name"`
	Description string                         `json:"description"`
	Enabled     *bool                          `json:"enabled,omitempty"`
	Permissions []models.ControlRolePermission `json:"permissions"`
}

type CreateStrategyRequest struct {
	Name         string                      `json:"name"`
	Description  string                      `json:"description"`
	Enabled      *bool                       `json:"enabled,omitempty"`
	SettingsJSON json.RawMessage             `json:"settings_json"`
	Assignments  []models.StrategyAssignment `json:"assignments"`
}

type StrategyAssignmentRequest struct {
	TargetType string `json:"target_type"`
	TargetID   int64  `json:"target_id"`
}

type EvaluateAccessRequest struct {
	User   access.User   `json:"user"`
	Target access.Device `json:"target"`
}

func NewHandler(repo Repository) Handler {
	return Handler{repo: repo}
}

func (h Handler) AccessRules(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		rules, err := h.repo.ListAccessRules(r.Context())
		if err != nil {
			httpx.Error(w, http.StatusInternalServerError, "LIST_ACCESS_RULES_FAILED", "list access rules failed")
			return
		}
		httpx.JSON(w, http.StatusOK, rules)
	case http.MethodPost:
		var req CreateAccessRuleRequest
		if err := httpx.DecodeJSON(r, &req); err != nil {
			httpx.Error(w, http.StatusBadRequest, "INVALID_JSON", "invalid JSON body")
			return
		}
		if err := validateAccessRule(req); err != nil {
			httpx.Error(w, http.StatusBadRequest, "INVALID_ACCESS_RULE", err.Error())
			return
		}
		rule, err := h.repo.CreateAccessRule(r.Context(), models.AccessRule{
			SubjectType: req.SubjectType,
			SubjectID:   req.SubjectID,
			TargetType:  req.TargetType,
			TargetID:    req.TargetID,
			Effect:      req.Effect,
			Priority:    req.Priority,
			Enabled:     defaultTrue(req.Enabled),
		})
		if err != nil {
			httpx.Error(w, http.StatusBadRequest, "INVALID_ACCESS_RULE", err.Error())
			return
		}
		if err := h.writeAudit(r, audit.Event{
			Action:       "create_access_rule",
			ResourceType: "access_rule",
			ResourceID:   strconv.FormatInt(rule.ID, 10),
			Metadata: map[string]any{
				"subject_type": rule.SubjectType,
				"subject_id":   rule.SubjectID,
				"target_type":  rule.TargetType,
				"target_id":    rule.TargetID,
				"effect":       rule.Effect,
				"priority":     rule.Priority,
				"enabled":      rule.Enabled,
			},
		}); err != nil {
			httpx.Error(w, http.StatusInternalServerError, "AUDIT_WRITE_FAILED", "failed to record access rule creation")
			return
		}
		httpx.JSON(w, http.StatusCreated, rule)
	default:
		httpx.Error(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
	}
}

func (h Handler) AccessRuleItem(w http.ResponseWriter, r *http.Request) {
	id, ok := parseAccessRulePath(r.URL.Path)
	if !ok {
		httpx.Error(w, http.StatusNotFound, "NOT_FOUND", "access rule route not found")
		return
	}
	switch r.Method {
	case http.MethodPut:
		h.updateAccessRule(w, r, id)
	case http.MethodDelete:
		h.deleteAccessRule(w, r, id)
	default:
		httpx.Error(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
	}
}

func (h Handler) updateAccessRule(w http.ResponseWriter, r *http.Request, id int64) {
	var req CreateAccessRuleRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "INVALID_JSON", "invalid JSON body")
		return
	}
	if err := validateAccessRule(req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "INVALID_ACCESS_RULE", err.Error())
		return
	}
	rule, err := h.repo.UpdateAccessRule(r.Context(), id, models.AccessRule{
		SubjectType: req.SubjectType,
		SubjectID:   req.SubjectID,
		TargetType:  req.TargetType,
		TargetID:    req.TargetID,
		Effect:      req.Effect,
		Priority:    req.Priority,
		Enabled:     defaultTrue(req.Enabled),
	})
	if errors.Is(err, repository.ErrNotFound) {
		httpx.Error(w, http.StatusNotFound, "ACCESS_RULE_NOT_FOUND", "access rule not found")
		return
	}
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "INVALID_ACCESS_RULE", err.Error())
		return
	}
	if err := h.writeAudit(r, audit.Event{
		Action:       "update_access_rule",
		ResourceType: "access_rule",
		ResourceID:   strconv.FormatInt(rule.ID, 10),
		Metadata: map[string]any{
			"subject_type": rule.SubjectType,
			"subject_id":   rule.SubjectID,
			"target_type":  rule.TargetType,
			"target_id":    rule.TargetID,
			"effect":       rule.Effect,
			"priority":     rule.Priority,
			"enabled":      rule.Enabled,
		},
	}); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "AUDIT_WRITE_FAILED", "failed to record access rule update")
		return
	}
	httpx.JSON(w, http.StatusOK, rule)
}

func (h Handler) deleteAccessRule(w http.ResponseWriter, r *http.Request, id int64) {
	if err := h.repo.DeleteAccessRule(r.Context(), id); errors.Is(err, repository.ErrNotFound) {
		httpx.Error(w, http.StatusNotFound, "ACCESS_RULE_NOT_FOUND", "access rule not found")
		return
	} else if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "DELETE_ACCESS_RULE_FAILED", "delete access rule failed")
		return
	}
	if err := h.writeAudit(r, audit.Event{
		Action:       "delete_access_rule",
		ResourceType: "access_rule",
		ResourceID:   strconv.FormatInt(id, 10),
	}); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "AUDIT_WRITE_FAILED", "failed to record access rule deletion")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

func (h Handler) EvaluateAccess(w http.ResponseWriter, r *http.Request) {
	if !httpx.Method(w, r, http.MethodPost) {
		return
	}
	var req EvaluateAccessRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "INVALID_JSON", "invalid JSON body")
		return
	}
	if req.User.ID <= 0 || req.Target.ID <= 0 {
		httpx.Error(w, http.StatusBadRequest, "INVALID_ACCESS_EVALUATION", "user.id and target.id are required")
		return
	}
	if strings.TrimSpace(req.User.Status) == "" {
		req.User.Status = "active"
	}
	if strings.TrimSpace(req.Target.Status) == "" {
		req.Target.Status = "online"
	}
	rules, err := h.repo.ListAccessRules(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "LIST_ACCESS_RULES_FAILED", "list access rules failed")
		return
	}
	decision := access.Evaluator{Rules: accessRules(rules)}.Evaluate(req.User, req.Target)
	httpx.JSON(w, http.StatusOK, decision)
}

func (h Handler) ControlRoles(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		roles, err := h.repo.ListControlRoles(r.Context())
		if err != nil {
			httpx.Error(w, http.StatusInternalServerError, "LIST_CONTROL_ROLES_FAILED", "list control roles failed")
			return
		}
		httpx.JSON(w, http.StatusOK, roles)
	case http.MethodPost:
		var req CreateControlRoleRequest
		if err := httpx.DecodeJSON(r, &req); err != nil {
			httpx.Error(w, http.StatusBadRequest, "INVALID_JSON", "invalid JSON body")
			return
		}
		if err := validateControlRole(req); err != nil {
			httpx.Error(w, http.StatusBadRequest, "INVALID_CONTROL_ROLE", err.Error())
			return
		}
		role, err := h.repo.CreateControlRole(r.Context(), models.ControlRole{
			Name:        strings.TrimSpace(req.Name),
			Description: strings.TrimSpace(req.Description),
			Enabled:     defaultTrue(req.Enabled),
			Permissions: normalizedPermissions(req.Permissions),
		})
		if err != nil {
			httpx.Error(w, http.StatusBadRequest, "INVALID_CONTROL_ROLE", err.Error())
			return
		}
		if err := h.writeAudit(r, audit.Event{
			Action:       "create_control_role",
			ResourceType: "control_role",
			ResourceID:   strconv.FormatInt(role.ID, 10),
			Metadata: map[string]any{
				"name":               role.Name,
				"enabled":            role.Enabled,
				"permission_count":   len(role.Permissions),
				"default_sensitive":  "terminal,tcp_tunnel,remote_config_modification",
				"created_permission": normalizedPermissionKeys(role.Permissions),
			},
		}); err != nil {
			httpx.Error(w, http.StatusInternalServerError, "AUDIT_WRITE_FAILED", "failed to record control role creation")
			return
		}
		httpx.JSON(w, http.StatusCreated, role)
	default:
		httpx.Error(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
	}
}

func (h Handler) ControlRoleItem(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePolicyItemPath(r.URL.Path, "/api/v1/control-roles/")
	if !ok {
		httpx.Error(w, http.StatusNotFound, "NOT_FOUND", "control role route not found")
		return
	}
	switch r.Method {
	case http.MethodPut:
		h.updateControlRole(w, r, id)
	case http.MethodDelete:
		h.deleteControlRole(w, r, id)
	default:
		httpx.Error(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
	}
}

func (h Handler) updateControlRole(w http.ResponseWriter, r *http.Request, id int64) {
	var req CreateControlRoleRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "INVALID_JSON", "invalid JSON body")
		return
	}
	if err := validateControlRole(req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "INVALID_CONTROL_ROLE", err.Error())
		return
	}
	role, err := h.repo.UpdateControlRole(r.Context(), id, models.ControlRole{
		Name:        strings.TrimSpace(req.Name),
		Description: strings.TrimSpace(req.Description),
		Enabled:     defaultTrue(req.Enabled),
		Permissions: normalizedPermissions(req.Permissions),
	})
	if errors.Is(err, repository.ErrNotFound) {
		httpx.Error(w, http.StatusNotFound, "CONTROL_ROLE_NOT_FOUND", "control role not found")
		return
	}
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "INVALID_CONTROL_ROLE", err.Error())
		return
	}
	if err := h.writeAudit(r, audit.Event{
		Action:       "update_control_role",
		ResourceType: "control_role",
		ResourceID:   strconv.FormatInt(role.ID, 10),
		Metadata: map[string]any{
			"name":             role.Name,
			"enabled":          role.Enabled,
			"permission_count": len(role.Permissions),
			"permissions":      normalizedPermissionKeys(role.Permissions),
		},
	}); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "AUDIT_WRITE_FAILED", "failed to record control role update")
		return
	}
	httpx.JSON(w, http.StatusOK, role)
}

func (h Handler) deleteControlRole(w http.ResponseWriter, r *http.Request, id int64) {
	if err := h.repo.DeleteControlRole(r.Context(), id); errors.Is(err, repository.ErrNotFound) {
		httpx.Error(w, http.StatusNotFound, "CONTROL_ROLE_NOT_FOUND", "control role not found")
		return
	} else if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "DELETE_CONTROL_ROLE_FAILED", "delete control role failed")
		return
	}
	if err := h.writeAudit(r, audit.Event{
		Action:       "delete_control_role",
		ResourceType: "control_role",
		ResourceID:   strconv.FormatInt(id, 10),
	}); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "AUDIT_WRITE_FAILED", "failed to record control role deletion")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

func (h Handler) Strategies(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		strategies, err := h.repo.ListStrategies(r.Context())
		if err != nil {
			httpx.Error(w, http.StatusInternalServerError, "LIST_STRATEGIES_FAILED", "list strategies failed")
			return
		}
		httpx.JSON(w, http.StatusOK, strategies)
	case http.MethodPost:
		var req CreateStrategyRequest
		if err := httpx.DecodeJSON(r, &req); err != nil {
			httpx.Error(w, http.StatusBadRequest, "INVALID_JSON", "invalid JSON body")
			return
		}
		settings, err := normalizeSettings(req.SettingsJSON)
		if err != nil {
			httpx.Error(w, http.StatusBadRequest, "INVALID_STRATEGY", err.Error())
			return
		}
		if err := validateStrategy(req, settings); err != nil {
			httpx.Error(w, http.StatusBadRequest, "INVALID_STRATEGY", err.Error())
			return
		}
		strategy, err := h.repo.CreateStrategy(r.Context(), models.Strategy{
			Name:         strings.TrimSpace(req.Name),
			Description:  strings.TrimSpace(req.Description),
			Enabled:      defaultTrue(req.Enabled),
			SettingsJSON: string(settings),
			Assignments:  normalizedAssignments(req.Assignments),
		})
		if err != nil {
			httpx.Error(w, http.StatusBadRequest, "INVALID_STRATEGY", err.Error())
			return
		}
		if err := h.writeAudit(r, audit.Event{
			Action:       "create_strategy",
			ResourceType: "strategy",
			ResourceID:   strconv.FormatInt(strategy.ID, 10),
			Metadata: map[string]any{
				"name":             strategy.Name,
				"enabled":          strategy.Enabled,
				"assignment_count": len(strategy.Assignments),
			},
		}); err != nil {
			httpx.Error(w, http.StatusInternalServerError, "AUDIT_WRITE_FAILED", "failed to record strategy creation")
			return
		}
		httpx.JSON(w, http.StatusCreated, strategy)
	default:
		httpx.Error(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
	}
}

func (h Handler) StrategyItem(w http.ResponseWriter, r *http.Request) {
	id, assignmentID, hasAssignmentID, action, ok := parseStrategyPath(r.URL.Path)
	if !ok {
		httpx.Error(w, http.StatusNotFound, "NOT_FOUND", "strategy route not found")
		return
	}
	if action == "assignments" {
		switch {
		case !hasAssignmentID && r.Method == http.MethodPost:
			h.addStrategyAssignment(w, r, id)
		case hasAssignmentID && r.Method == http.MethodDelete:
			h.deleteStrategyAssignment(w, r, id, assignmentID)
		default:
			httpx.Error(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
		}
		return
	}
	if action != "" {
		httpx.Error(w, http.StatusNotFound, "NOT_FOUND", "strategy route not found")
		return
	}
	switch r.Method {
	case http.MethodPut:
		h.updateStrategy(w, r, id)
	case http.MethodDelete:
		h.deleteStrategy(w, r, id)
	default:
		httpx.Error(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
	}
}

func (h Handler) updateStrategy(w http.ResponseWriter, r *http.Request, id int64) {
	var req CreateStrategyRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "INVALID_JSON", "invalid JSON body")
		return
	}
	settings, err := normalizeSettings(req.SettingsJSON)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "INVALID_STRATEGY", err.Error())
		return
	}
	if err := validateStrategy(req, settings); err != nil {
		httpx.Error(w, http.StatusBadRequest, "INVALID_STRATEGY", err.Error())
		return
	}
	strategy, err := h.repo.UpdateStrategy(r.Context(), id, models.Strategy{
		Name:         strings.TrimSpace(req.Name),
		Description:  strings.TrimSpace(req.Description),
		Enabled:      defaultTrue(req.Enabled),
		SettingsJSON: string(settings),
		Assignments:  normalizedAssignments(req.Assignments),
	})
	if errors.Is(err, repository.ErrNotFound) {
		httpx.Error(w, http.StatusNotFound, "STRATEGY_NOT_FOUND", "strategy not found")
		return
	}
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "INVALID_STRATEGY", err.Error())
		return
	}
	if err := h.writeAudit(r, audit.Event{
		Action:       "update_strategy",
		ResourceType: "strategy",
		ResourceID:   strconv.FormatInt(strategy.ID, 10),
		Metadata: map[string]any{
			"name":             strategy.Name,
			"enabled":          strategy.Enabled,
			"assignment_count": len(strategy.Assignments),
		},
	}); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "AUDIT_WRITE_FAILED", "failed to record strategy update")
		return
	}
	httpx.JSON(w, http.StatusOK, strategy)
}

func (h Handler) addStrategyAssignment(w http.ResponseWriter, r *http.Request, id int64) {
	var req StrategyAssignmentRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "INVALID_JSON", "invalid JSON body")
		return
	}
	if err := validateStrategyAssignment(req.TargetType, req.TargetID); err != nil {
		httpx.Error(w, http.StatusBadRequest, "INVALID_STRATEGY_ASSIGNMENT", err.Error())
		return
	}
	strategy, err := h.repo.AddStrategyAssignment(r.Context(), id, models.StrategyAssignment{
		TargetType: req.TargetType,
		TargetID:   req.TargetID,
	})
	if errors.Is(err, repository.ErrNotFound) {
		httpx.Error(w, http.StatusNotFound, "STRATEGY_NOT_FOUND", "strategy not found")
		return
	}
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "INVALID_STRATEGY_ASSIGNMENT", err.Error())
		return
	}
	if err := h.writeAudit(r, audit.Event{
		Action:       "assign_strategy",
		ResourceType: "strategy",
		ResourceID:   strconv.FormatInt(strategy.ID, 10),
		Metadata: map[string]any{
			"target_type":      req.TargetType,
			"target_id":        req.TargetID,
			"assignment_count": len(strategy.Assignments),
		},
	}); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "AUDIT_WRITE_FAILED", "failed to record strategy assignment")
		return
	}
	httpx.JSON(w, http.StatusCreated, strategy)
}

func (h Handler) deleteStrategyAssignment(w http.ResponseWriter, r *http.Request, id int64, assignmentID int64) {
	strategy, err := h.repo.RemoveStrategyAssignment(r.Context(), id, assignmentID)
	if errors.Is(err, repository.ErrNotFound) {
		httpx.Error(w, http.StatusNotFound, "STRATEGY_ASSIGNMENT_NOT_FOUND", "strategy assignment not found")
		return
	}
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "DELETE_STRATEGY_ASSIGNMENT_FAILED", "delete strategy assignment failed")
		return
	}
	if err := h.writeAudit(r, audit.Event{
		Action:       "delete_strategy_assignment",
		ResourceType: "strategy",
		ResourceID:   strconv.FormatInt(strategy.ID, 10),
		Metadata: map[string]any{
			"assignment_id":    assignmentID,
			"assignment_count": len(strategy.Assignments),
		},
	}); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "AUDIT_WRITE_FAILED", "failed to record strategy assignment deletion")
		return
	}
	httpx.JSON(w, http.StatusOK, strategy)
}

func (h Handler) deleteStrategy(w http.ResponseWriter, r *http.Request, id int64) {
	if err := h.repo.DeleteStrategy(r.Context(), id); errors.Is(err, repository.ErrNotFound) {
		httpx.Error(w, http.StatusNotFound, "STRATEGY_NOT_FOUND", "strategy not found")
		return
	} else if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "DELETE_STRATEGY_FAILED", "delete strategy failed")
		return
	}
	if err := h.writeAudit(r, audit.Event{
		Action:       "delete_strategy",
		ResourceType: "strategy",
		ResourceID:   strconv.FormatInt(id, 10),
	}); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "AUDIT_WRITE_FAILED", "failed to record strategy deletion")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

func validateAccessRule(req CreateAccessRuleRequest) error {
	if !oneOf(req.SubjectType, "user", "user_group") {
		return errors.New("subject_type must be user or user_group")
	}
	if req.SubjectID <= 0 {
		return errors.New("subject_id must be positive")
	}
	if !oneOf(req.TargetType, "device", "device_group") {
		return errors.New("target_type must be device or device_group")
	}
	if req.TargetID <= 0 {
		return errors.New("target_id must be positive")
	}
	if !oneOf(req.Effect, "allow", "deny") {
		return errors.New("effect must be allow or deny")
	}
	return nil
}

func validateControlRole(req CreateControlRoleRequest) error {
	if strings.TrimSpace(req.Name) == "" {
		return errors.New("name is required")
	}
	for _, permission := range req.Permissions {
		if !knownPermission(permission.PermissionKey) {
			return errors.New("unknown permission key: " + permission.PermissionKey)
		}
		if !oneOf(permission.Mode, string(controlrole.UseClientSettings), string(controlrole.Enable), string(controlrole.Disable)) {
			return errors.New("permission mode must be use_client_settings, enable, or disable")
		}
	}
	return nil
}

func validateStrategy(req CreateStrategyRequest, settings json.RawMessage) error {
	if strings.TrimSpace(req.Name) == "" {
		return errors.New("name is required")
	}
	if len(settings) == 0 {
		return errors.New("settings_json is required")
	}
	for _, assignment := range req.Assignments {
		if err := validateStrategyAssignment(assignment.TargetType, assignment.TargetID); err != nil {
			return err
		}
	}
	return nil
}

func validateStrategyAssignment(targetType string, targetID int64) error {
	if !oneOf(targetType, "device", "user", "device_group") {
		return errors.New("assignment target_type must be device, user, or device_group")
	}
	if targetID <= 0 {
		return errors.New("assignment target_id must be positive")
	}
	return nil
}

func normalizeSettings(raw json.RawMessage) (json.RawMessage, error) {
	if len(raw) == 0 || strings.TrimSpace(string(raw)) == "null" {
		return json.RawMessage(`{}`), nil
	}
	var object map[string]any
	if err := json.Unmarshal(raw, &object); err != nil {
		return nil, errors.New("settings_json must be a JSON object")
	}
	normalized, err := json.Marshal(object)
	if err != nil {
		return nil, err
	}
	return normalized, nil
}

func normalizedPermissions(in []models.ControlRolePermission) []models.ControlRolePermission {
	out := make([]models.ControlRolePermission, 0, len(in))
	for _, permission := range in {
		out = append(out, models.ControlRolePermission{
			PermissionKey: strings.TrimSpace(permission.PermissionKey),
			Mode:          strings.TrimSpace(permission.Mode),
		})
	}
	return out
}

func normalizedAssignments(in []models.StrategyAssignment) []models.StrategyAssignment {
	out := make([]models.StrategyAssignment, 0, len(in))
	for _, assignment := range in {
		out = append(out, models.StrategyAssignment{
			TargetType: strings.TrimSpace(assignment.TargetType),
			TargetID:   assignment.TargetID,
		})
	}
	return out
}

func knownPermission(key string) bool {
	for _, known := range controlrole.PermissionKeys {
		if known == key {
			return true
		}
	}
	return false
}

func defaultTrue(value *bool) bool {
	return value == nil || *value
}

func oneOf(value string, allowed ...string) bool {
	for _, item := range allowed {
		if value == item {
			return true
		}
	}
	return false
}

func parseAccessRulePath(path string) (int64, bool) {
	return parsePolicyItemPath(path, "/api/v1/access-rules/")
}

func parsePolicyItemPath(path, prefix string) (int64, bool) {
	rest := strings.TrimPrefix(path, prefix)
	parts := strings.Split(strings.Trim(rest, "/"), "/")
	if len(parts) != 1 {
		return 0, false
	}
	id, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || id <= 0 {
		return 0, false
	}
	return id, true
}

func parseStrategyPath(path string) (strategyID int64, assignmentID int64, hasAssignmentID bool, action string, ok bool) {
	rest := strings.TrimPrefix(path, "/api/v1/strategies/")
	parts := strings.Split(strings.Trim(rest, "/"), "/")
	if len(parts) != 1 && len(parts) != 2 && len(parts) != 3 {
		return 0, 0, false, "", false
	}
	strategyID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || strategyID <= 0 {
		return 0, 0, false, "", false
	}
	if len(parts) == 1 {
		return strategyID, 0, false, "", true
	}
	if parts[1] != "assignments" {
		return 0, 0, false, "", false
	}
	if len(parts) == 2 {
		return strategyID, 0, false, "assignments", true
	}
	assignmentID, err = strconv.ParseInt(parts[2], 10, 64)
	if err != nil || assignmentID <= 0 {
		return 0, 0, false, "", false
	}
	return strategyID, assignmentID, true, "assignments", true
}

func accessRules(in []models.AccessRule) []access.Rule {
	out := make([]access.Rule, 0, len(in))
	for _, rule := range in {
		out = append(out, access.Rule{
			ID:          rule.ID,
			SubjectType: rule.SubjectType,
			SubjectID:   rule.SubjectID,
			TargetType:  rule.TargetType,
			TargetID:    rule.TargetID,
			Effect:      rule.Effect,
			Priority:    rule.Priority,
			Enabled:     rule.Enabled,
		})
	}
	return out
}

func normalizedPermissionKeys(permissions []models.ControlRolePermission) []string {
	out := make([]string, 0, len(permissions))
	for _, permission := range permissions {
		out = append(out, permission.PermissionKey)
	}
	return out
}

func (h Handler) writeAudit(r *http.Request, event audit.Event) error {
	event.ActorType = "system"
	if session, ok := apiauth.SessionFromContext(r.Context()); ok {
		event.ActorType = apiauth.ActorType(session)
		userID := session.User.ID
		event.ActorUserID = &userID
	}
	event.IP = requestIP(r)
	event.UserAgent = r.UserAgent()
	return (audit.RepositoryWriter{Repo: h.repo}).Write(r.Context(), event)
}

func requestIP(r *http.Request) string {
	if forwarded := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); forwarded != "" {
		ip, _, _ := strings.Cut(forwarded, ",")
		return strings.TrimSpace(ip)
	}
	if realIP := strings.TrimSpace(r.Header.Get("X-Real-IP")); realIP != "" {
		return realIP
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}
