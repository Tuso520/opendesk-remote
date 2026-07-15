package settings

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/opendesk-remote/opendesk-remote/api/internal/audit"
	apiauth "github.com/opendesk-remote/opendesk-remote/api/internal/auth"
	"github.com/opendesk-remote/opendesk-remote/api/internal/httpx"
	"github.com/opendesk-remote/opendesk-remote/api/internal/models"
	"github.com/opendesk-remote/opendesk-remote/api/internal/repository"
)

type Handler struct {
	repo Repository
}

type Repository interface {
	repository.SettingsRepository
	audit.Repository
}

type UpdateRequest struct {
	Settings map[string]json.RawMessage `json:"settings"`
}

type SettingResponse struct {
	Key       string     `json:"key"`
	Value     any        `json:"value"`
	ValueJSON string     `json:"value_json"`
	Section   string     `json:"section,omitempty"`
	Source    string     `json:"source"`
	UpdatedBy *int64     `json:"updated_by,omitempty"`
	UpdatedAt *time.Time `json:"updated_at,omitempty"`
}

type sectionDefinition struct {
	Name     string
	Prefix   string
	Defaults map[string]json.RawMessage
}

var baseDefaults = map[string]json.RawMessage{
	"audit_retention_days":          json.RawMessage(`180`),
	"connection_log_retention_days": json.RawMessage(`90`),
	"file_transfer_enabled_default": json.RawMessage(`true`),
	"public_rustdesk_fallback":      json.RawMessage(`false`),
	"relay_auth_required":           json.RawMessage(`true`),
	"terminal_enabled_default":      json.RawMessage(`false`),
	"web_client_enabled":            json.RawMessage(`false`),
}

var sections = map[string]sectionDefinition{
	"oidc": {
		Name:   "oidc",
		Prefix: "oidc.",
		Defaults: map[string]json.RawMessage{
			"enabled":           json.RawMessage(`false`),
			"issuer_url":        json.RawMessage(`""`),
			"client_id":         json.RawMessage(`""`),
			"scopes":            json.RawMessage(`"openid profile email"`),
			"auto_create_users": json.RawMessage(`false`),
		},
	},
	"ldap": {
		Name:   "ldap",
		Prefix: "ldap.",
		Defaults: map[string]json.RawMessage{
			"enabled":      json.RawMessage(`false`),
			"url":          json.RawMessage(`""`),
			"base_dn":      json.RawMessage(`""`),
			"bind_dn":      json.RawMessage(`""`),
			"user_filter":  json.RawMessage(`"(uid={username})"`),
			"group_filter": json.RawMessage(`""`),
			"start_tls":    json.RawMessage(`true`),
		},
	},
	"smtp": {
		Name:   "smtp",
		Prefix: "smtp.",
		Defaults: map[string]json.RawMessage{
			"enabled":      json.RawMessage(`false`),
			"host":         json.RawMessage(`""`),
			"port":         json.RawMessage(`587`),
			"username":     json.RawMessage(`""`),
			"from_address": json.RawMessage(`""`),
			"tls":          json.RawMessage(`true`),
		},
	},
}

func NewHandler(repo Repository) Handler {
	return Handler{repo: repo}
}

func (h Handler) Collection(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		settings, err := h.mergedSettings(r, sectionDefinition{Name: "general", Defaults: baseDefaults})
		if err != nil {
			httpx.Error(w, http.StatusInternalServerError, "LIST_SETTINGS_FAILED", "list settings failed")
			return
		}
		httpx.JSON(w, http.StatusOK, settings)
	case http.MethodPut:
		var req UpdateRequest
		if err := httpx.DecodeJSON(r, &req); err != nil {
			httpx.Error(w, http.StatusBadRequest, "INVALID_JSON", "invalid JSON body")
			return
		}
		updates, err := normalizeUpdates(req.Settings, sectionDefinition{Name: "general", Defaults: baseDefaults})
		if err != nil {
			httpx.Error(w, http.StatusBadRequest, "INVALID_SETTINGS", err.Error())
			return
		}
		session, ok := apiauth.SessionFromContext(r.Context())
		if !ok {
			httpx.Error(w, http.StatusUnauthorized, "UNAUTHENTICATED", "authentication is required")
			return
		}
		if err := h.repo.UpsertSystemSettings(r.Context(), updates, session.User.ID); err != nil {
			httpx.Error(w, http.StatusBadRequest, "UPDATE_SETTINGS_FAILED", err.Error())
			return
		}
		if err := h.writeAudit(r, session, audit.Event{
			Action:       "update_system_settings",
			ResourceType: "settings",
			ResourceID:   "general",
			Metadata: map[string]any{
				"section": "general",
				"keys":    settingKeys(updates),
			},
		}); err != nil {
			httpx.Error(w, http.StatusInternalServerError, "AUDIT_WRITE_FAILED", "failed to record settings update")
			return
		}
		settings, err := h.mergedSettings(r, sectionDefinition{Name: "general", Defaults: baseDefaults})
		if err != nil {
			httpx.Error(w, http.StatusInternalServerError, "LIST_SETTINGS_FAILED", "list settings failed")
			return
		}
		httpx.JSON(w, http.StatusOK, settings)
	default:
		httpx.Error(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
	}
}

func (h Handler) OIDC(w http.ResponseWriter, r *http.Request) {
	h.section(w, r, "oidc")
}

func (h Handler) LDAP(w http.ResponseWriter, r *http.Request) {
	h.section(w, r, "ldap")
}

func (h Handler) SMTP(w http.ResponseWriter, r *http.Request) {
	h.section(w, r, "smtp")
}

func (h Handler) section(w http.ResponseWriter, r *http.Request, name string) {
	definition, ok := sections[name]
	if !ok {
		httpx.Error(w, http.StatusNotFound, "SETTINGS_SECTION_NOT_FOUND", "settings section not found")
		return
	}
	switch r.Method {
	case http.MethodGet:
		settings, err := h.mergedSettings(r, definition)
		if err != nil {
			httpx.Error(w, http.StatusInternalServerError, "LIST_SETTINGS_FAILED", "list settings failed")
			return
		}
		httpx.JSON(w, http.StatusOK, settings)
	case http.MethodPut:
		var req UpdateRequest
		if err := httpx.DecodeJSON(r, &req); err != nil {
			httpx.Error(w, http.StatusBadRequest, "INVALID_JSON", "invalid JSON body")
			return
		}
		updates, err := normalizeUpdates(req.Settings, definition)
		if err != nil {
			httpx.Error(w, http.StatusBadRequest, "INVALID_SETTINGS", err.Error())
			return
		}
		session, ok := apiauth.SessionFromContext(r.Context())
		if !ok {
			httpx.Error(w, http.StatusUnauthorized, "UNAUTHENTICATED", "authentication is required")
			return
		}
		if err := h.repo.UpsertSystemSettings(r.Context(), updates, session.User.ID); err != nil {
			httpx.Error(w, http.StatusBadRequest, "UPDATE_SETTINGS_FAILED", err.Error())
			return
		}
		if err := h.writeAudit(r, session, audit.Event{
			Action:       "update_system_settings",
			ResourceType: "settings",
			ResourceID:   definition.Name,
			Metadata: map[string]any{
				"section": definition.Name,
				"keys":    settingKeys(updates),
			},
		}); err != nil {
			httpx.Error(w, http.StatusInternalServerError, "AUDIT_WRITE_FAILED", "failed to record settings update")
			return
		}
		settings, err := h.mergedSettings(r, definition)
		if err != nil {
			httpx.Error(w, http.StatusInternalServerError, "LIST_SETTINGS_FAILED", "list settings failed")
			return
		}
		httpx.JSON(w, http.StatusOK, settings)
	default:
		httpx.Error(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
	}
}

func (h Handler) mergedSettings(r *http.Request, definition sectionDefinition) ([]SettingResponse, error) {
	stored, err := h.repo.ListSystemSettings(r.Context())
	if err != nil {
		return nil, err
	}
	merged := map[string]SettingResponse{}
	for key, raw := range definition.Defaults {
		value, err := decodeValue(raw)
		if err != nil {
			return nil, err
		}
		merged[key] = SettingResponse{
			Key:       key,
			Value:     value,
			ValueJSON: string(raw),
			Section:   definition.Name,
			Source:    "default",
		}
	}
	for _, setting := range stored {
		responseKey, ok := responseKeyForStored(setting.Key, definition)
		if !ok {
			continue
		}
		raw := json.RawMessage(setting.ValueJSON)
		value, err := decodeValue(raw)
		if err != nil {
			return nil, err
		}
		updatedAt := setting.UpdatedAt
		merged[responseKey] = SettingResponse{
			Key:       responseKey,
			Value:     value,
			ValueJSON: string(raw),
			Section:   definition.Name,
			Source:    "stored",
			UpdatedBy: setting.UpdatedBy,
			UpdatedAt: &updatedAt,
		}
	}
	out := make([]SettingResponse, 0, len(merged))
	for _, setting := range merged {
		out = append(out, setting)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Key < out[j].Key
	})
	return out, nil
}

func normalizeUpdates(in map[string]json.RawMessage, definition sectionDefinition) ([]models.SystemSetting, error) {
	if len(in) == 0 {
		return nil, errors.New("settings object is required")
	}
	out := make([]models.SystemSetting, 0, len(in))
	for key, raw := range in {
		key = strings.TrimSpace(key)
		if key == "" {
			return nil, errors.New("setting key is required")
		}
		defaultRaw, ok := definition.Defaults[key]
		if !ok {
			return nil, errors.New("unknown setting key: " + key)
		}
		normalized, err := normalizeJSON(raw, defaultRaw, key)
		if err != nil {
			return nil, err
		}
		out = append(out, models.SystemSetting{
			Key:       definition.Prefix + key,
			ValueJSON: string(normalized),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Key < out[j].Key
	})
	return out, nil
}

func normalizeJSON(raw json.RawMessage, defaultRaw json.RawMessage, key string) (json.RawMessage, error) {
	if len(raw) == 0 {
		return nil, errors.New("setting value is required")
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, errors.New("setting value must be valid JSON")
	}
	var defaultValue any
	if err := json.Unmarshal(defaultRaw, &defaultValue); err != nil {
		return nil, err
	}
	if !sameJSONKind(value, defaultValue) {
		return nil, errors.New(key + " has invalid value type")
	}
	normalized, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return normalized, nil
}

func decodeValue(raw json.RawMessage) (any, error) {
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, err
	}
	return value, nil
}

func responseKeyForStored(storedKey string, definition sectionDefinition) (string, bool) {
	if definition.Prefix == "" {
		if _, ok := definition.Defaults[storedKey]; !ok {
			return "", false
		}
		return storedKey, true
	}
	if !strings.HasPrefix(storedKey, definition.Prefix) {
		return "", false
	}
	key := strings.TrimPrefix(storedKey, definition.Prefix)
	if _, ok := definition.Defaults[key]; !ok {
		return "", false
	}
	return key, true
}

func sameJSONKind(value any, defaultValue any) bool {
	switch defaultValue.(type) {
	case bool:
		_, ok := value.(bool)
		return ok
	case float64:
		_, ok := value.(float64)
		return ok
	case string:
		_, ok := value.(string)
		return ok
	case []any:
		_, ok := value.([]any)
		return ok
	case map[string]any:
		_, ok := value.(map[string]any)
		return ok
	case nil:
		return value == nil
	default:
		return false
	}
}

func settingKeys(settings []models.SystemSetting) []string {
	keys := make([]string, 0, len(settings))
	for _, setting := range settings {
		keys = append(keys, setting.Key)
	}
	sort.Strings(keys)
	return keys
}

func (h Handler) writeAudit(r *http.Request, session apiauth.Session, event audit.Event) error {
	event.ActorType = apiauth.ActorType(session)
	userID := session.User.ID
	event.ActorUserID = &userID
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
