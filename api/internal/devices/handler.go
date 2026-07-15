package devices

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/opendesk-remote/opendesk-remote/api/internal/audit"
	apiauth "github.com/opendesk-remote/opendesk-remote/api/internal/auth"
	"github.com/opendesk-remote/opendesk-remote/api/internal/httpx"
	"github.com/opendesk-remote/opendesk-remote/api/internal/models"
	"github.com/opendesk-remote/opendesk-remote/api/internal/repository"
	strategycalc "github.com/opendesk-remote/opendesk-remote/api/internal/strategy"
)

type Handler struct {
	repo Repository
}

type Repository interface {
	repository.DeviceRepository
	repository.PolicyRepository
	audit.Repository
}

func NewHandler(repo Repository) Handler {
	return Handler{repo: repo}
}

type CreateDeviceGroupRequest struct {
	Name            string  `json:"name"`
	Description     string  `json:"description"`
	MemberDeviceIDs []int64 `json:"member_device_ids"`
}

type AddDeviceGroupMemberRequest struct {
	DeviceID int64 `json:"device_id"`
}

func (h Handler) Item(w http.ResponseWriter, r *http.Request) {
	id, action, ok := parseDevicePath(r.URL.Path)
	if !ok {
		httpx.Error(w, http.StatusNotFound, "NOT_FOUND", "device route not found")
		return
	}
	if action == "disable" {
		if !httpx.Method(w, r, http.MethodPost) {
			return
		}
		h.disableDevice(w, r, id)
		return
	}
	if action == "effective-strategy" {
		if !httpx.Method(w, r, http.MethodGet) {
			return
		}
		h.effectiveStrategy(w, r, id)
		return
	}
	if action != "" {
		httpx.Error(w, http.StatusNotFound, "NOT_FOUND", "device route not found")
		return
	}
	switch r.Method {
	case http.MethodGet:
		device, err := h.repo.FindDeviceByID(r.Context(), id)
		if errors.Is(err, repository.ErrNotFound) {
			httpx.Error(w, http.StatusNotFound, "DEVICE_NOT_FOUND", "device not found")
			return
		}
		if err != nil {
			httpx.Error(w, http.StatusInternalServerError, "LOAD_DEVICE_FAILED", "load device failed")
			return
		}
		httpx.JSON(w, http.StatusOK, device)
	case http.MethodPut:
		h.updateDevice(w, r, id)
	default:
		httpx.Error(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
	}
}

func (h Handler) Collection(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		devices, err := h.repo.ListDevices(r.Context())
		if err != nil {
			httpx.Error(w, http.StatusInternalServerError, "LIST_DEVICES_FAILED", "list devices failed")
			return
		}
		httpx.JSON(w, http.StatusOK, devices)
	case http.MethodPost:
		var device models.Device
		if err := httpx.DecodeJSON(r, &device); err != nil {
			httpx.Error(w, http.StatusBadRequest, "INVALID_JSON", "invalid JSON body")
			return
		}
		created, err := h.repo.CreateDevice(r.Context(), device)
		if err != nil {
			httpx.Error(w, http.StatusBadRequest, "INVALID_DEVICE", err.Error())
			return
		}
		if err := h.writeAudit(r, audit.Event{
			Action:       "create_device",
			ResourceType: "device",
			ResourceID:   strconv.FormatInt(created.ID, 10),
			Metadata: map[string]any{
				"rustdesk_id": created.RustDeskID,
				"name":        created.Name,
				"platform":    created.Platform,
				"status":      created.Status,
			},
		}); err != nil {
			httpx.Error(w, http.StatusInternalServerError, "AUDIT_WRITE_FAILED", "failed to record device creation")
			return
		}
		httpx.JSON(w, http.StatusCreated, created)
	default:
		httpx.Error(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
	}
}

func (h Handler) Register(w http.ResponseWriter, r *http.Request) {
	if !httpx.Method(w, r, http.MethodPost) {
		return
	}
	var device models.Device
	if err := httpx.DecodeJSON(r, &device); err != nil {
		httpx.Error(w, http.StatusBadRequest, "INVALID_JSON", "invalid JSON body")
		return
	}
	created, err := h.repo.CreateDevice(r.Context(), device)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "INVALID_DEVICE", err.Error())
		return
	}
	if err := h.writeAudit(r, audit.Event{
		Action:       "register_device",
		ResourceType: "device",
		ResourceID:   strconv.FormatInt(created.ID, 10),
		Metadata: map[string]any{
			"rustdesk_id": created.RustDeskID,
			"name":        created.Name,
			"platform":    created.Platform,
			"status":      created.Status,
		},
	}); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "AUDIT_WRITE_FAILED", "failed to record device registration")
		return
	}
	httpx.JSON(w, http.StatusCreated, created)
}

type EffectiveStrategyResponse struct {
	DeviceID           int64             `json:"device_id"`
	UserID             *int64            `json:"user_id,omitempty"`
	DeviceGroupIDs     []int64           `json:"device_group_ids"`
	Settings           map[string]string `json:"settings"`
	AppliedStrategyIDs []int64           `json:"applied_strategy_ids"`
}

func (h Handler) effectiveStrategy(w http.ResponseWriter, r *http.Request, id int64) {
	device, err := h.repo.FindDeviceByID(r.Context(), id)
	if errors.Is(err, repository.ErrNotFound) {
		httpx.Error(w, http.StatusNotFound, "DEVICE_NOT_FOUND", "device not found")
		return
	}
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "LOAD_DEVICE_FAILED", "load device failed")
		return
	}
	userID := device.OwnerUserID
	if rawUserID := strings.TrimSpace(r.URL.Query().Get("user_id")); rawUserID != "" {
		parsed, err := strconv.ParseInt(rawUserID, 10, 64)
		if err != nil || parsed <= 0 {
			httpx.Error(w, http.StatusBadRequest, "INVALID_USER_ID", "user_id must be positive")
			return
		}
		userID = &parsed
	}
	deviceGroupIDs, err := h.deviceGroupIDs(r.Context(), id)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "LOAD_DEVICE_GROUPS_FAILED", "load device groups failed")
		return
	}
	strategies, err := h.repo.ListStrategies(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "LIST_STRATEGIES_FAILED", "list strategies failed")
		return
	}
	resolver, err := buildStrategyResolver(strategies)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "INVALID_STORED_STRATEGY", err.Error())
		return
	}
	result := resolver.Resolve(strategycalc.Context{
		DeviceID:       id,
		UserID:         userID,
		DeviceGroupIDs: deviceGroupIDs,
	})
	httpx.JSON(w, http.StatusOK, EffectiveStrategyResponse{
		DeviceID:           id,
		UserID:             userID,
		DeviceGroupIDs:     deviceGroupIDs,
		Settings:           result.Settings,
		AppliedStrategyIDs: result.AppliedStrategy,
	})
}

func (h Handler) deviceGroupIDs(ctx context.Context, deviceID int64) ([]int64, error) {
	groups, err := h.repo.ListDeviceGroups(ctx)
	if err != nil {
		return nil, err
	}
	out := []int64{}
	for _, group := range groups {
		members, err := h.repo.ListDeviceGroupMembers(ctx, group.ID)
		if errors.Is(err, repository.ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		for _, memberID := range members {
			if memberID == deviceID {
				out = append(out, group.ID)
				break
			}
		}
	}
	return out, nil
}

func buildStrategyResolver(in []models.Strategy) (strategycalc.Resolver, error) {
	resolver := strategycalc.Resolver{
		Default: strategycalc.Strategy{
			ID:      0,
			Name:    "default-secure",
			Enabled: true,
			Settings: map[string]string{
				"terminal":                         "N",
				"tcp_tunnel":                       "N",
				"allow-remote-config-modification": "N",
			},
		},
		Strategies:  []strategycalc.Strategy{},
		Assignments: []strategycalc.Assignment{},
	}
	for _, item := range in {
		settings, err := strategySettings(item.SettingsJSON)
		if err != nil {
			return strategycalc.Resolver{}, fmt.Errorf("strategy %d settings_json must be a JSON object", item.ID)
		}
		resolver.Strategies = append(resolver.Strategies, strategycalc.Strategy{
			ID:       item.ID,
			Name:     item.Name,
			Enabled:  item.Enabled,
			Settings: settings,
		})
		for _, assignment := range item.Assignments {
			resolver.Assignments = append(resolver.Assignments, strategycalc.Assignment{
				StrategyID: item.ID,
				TargetType: strategycalc.TargetType(assignment.TargetType),
				TargetID:   assignment.TargetID,
			})
		}
	}
	return resolver, nil
}

func strategySettings(raw string) (map[string]string, error) {
	var object map[string]any
	if err := json.Unmarshal([]byte(raw), &object); err != nil {
		return nil, err
	}
	out := map[string]string{}
	for key, value := range object {
		switch typed := value.(type) {
		case string:
			out[key] = typed
		case bool:
			out[key] = strconv.FormatBool(typed)
		case float64:
			out[key] = strconv.FormatFloat(typed, 'f', -1, 64)
		default:
			encoded, _ := json.Marshal(typed)
			out[key] = string(encoded)
		}
	}
	return out, nil
}

func (h Handler) updateDevice(w http.ResponseWriter, r *http.Request, id int64) {
	var device models.Device
	if err := httpx.DecodeJSON(r, &device); err != nil {
		httpx.Error(w, http.StatusBadRequest, "INVALID_JSON", "invalid JSON body")
		return
	}
	if err := validateDevice(device); err != nil {
		httpx.Error(w, http.StatusBadRequest, "INVALID_DEVICE", err.Error())
		return
	}
	updated, err := h.repo.UpdateDevice(r.Context(), id, device)
	if errors.Is(err, repository.ErrNotFound) {
		httpx.Error(w, http.StatusNotFound, "DEVICE_NOT_FOUND", "device not found")
		return
	}
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "INVALID_DEVICE", err.Error())
		return
	}
	if err := h.writeAudit(r, audit.Event{
		Action:       "update_device",
		ResourceType: "device",
		ResourceID:   strconv.FormatInt(updated.ID, 10),
		Metadata: map[string]any{
			"rustdesk_id": updated.RustDeskID,
			"name":        updated.Name,
			"platform":    updated.Platform,
			"status":      updated.Status,
		},
	}); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "AUDIT_WRITE_FAILED", "failed to record device update")
		return
	}
	httpx.JSON(w, http.StatusOK, updated)
}

func (h Handler) disableDevice(w http.ResponseWriter, r *http.Request, id int64) {
	disabled, err := h.repo.DisableDevice(r.Context(), id, time.Now().UTC())
	if errors.Is(err, repository.ErrNotFound) {
		httpx.Error(w, http.StatusNotFound, "DEVICE_NOT_FOUND", "device not found")
		return
	}
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "DISABLE_DEVICE_FAILED", "disable device failed")
		return
	}
	if err := h.writeAudit(r, audit.Event{
		Action:       "disable_device",
		ResourceType: "device",
		ResourceID:   strconv.FormatInt(disabled.ID, 10),
		Metadata: map[string]any{
			"rustdesk_id": disabled.RustDeskID,
			"name":        disabled.Name,
			"status":      disabled.Status,
		},
	}); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "AUDIT_WRITE_FAILED", "failed to record device disable")
		return
	}
	httpx.JSON(w, http.StatusOK, disabled)
}

func (h Handler) GroupItem(w http.ResponseWriter, r *http.Request) {
	groupID, deviceID, hasDeviceID, ok := parseDeviceGroupMemberPath(r.URL.Path)
	if !ok {
		httpx.Error(w, http.StatusNotFound, "NOT_FOUND", "device group route not found")
		return
	}

	switch {
	case !hasDeviceID && r.Method == http.MethodGet:
		members, err := h.repo.ListDeviceGroupMembers(r.Context(), groupID)
		if errors.Is(err, repository.ErrNotFound) {
			httpx.Error(w, http.StatusNotFound, "DEVICE_GROUP_NOT_FOUND", "device group not found")
			return
		}
		if err != nil {
			httpx.Error(w, http.StatusInternalServerError, "LIST_DEVICE_GROUP_MEMBERS_FAILED", "list device group members failed")
			return
		}
		httpx.JSON(w, http.StatusOK, members)
	case !hasDeviceID && r.Method == http.MethodPost:
		var req AddDeviceGroupMemberRequest
		if err := httpx.DecodeJSON(r, &req); err != nil {
			httpx.Error(w, http.StatusBadRequest, "INVALID_JSON", "invalid JSON body")
			return
		}
		if req.DeviceID <= 0 {
			httpx.Error(w, http.StatusBadRequest, "INVALID_DEVICE_GROUP_MEMBER", "device_id must be positive")
			return
		}
		group, err := h.repo.AddDeviceGroupMember(r.Context(), groupID, req.DeviceID)
		if errors.Is(err, repository.ErrNotFound) {
			httpx.Error(w, http.StatusNotFound, "DEVICE_GROUP_NOT_FOUND", "device group not found")
			return
		}
		if err != nil {
			httpx.Error(w, http.StatusBadRequest, "INVALID_DEVICE_GROUP_MEMBER", err.Error())
			return
		}
		if err := h.writeAudit(r, audit.Event{
			Action:       "add_device_group_member",
			ResourceType: "device_group",
			ResourceID:   strconv.FormatInt(group.ID, 10),
			Metadata: map[string]any{
				"group_name": group.Name,
				"device_id":  req.DeviceID,
			},
		}); err != nil {
			httpx.Error(w, http.StatusInternalServerError, "AUDIT_WRITE_FAILED", "failed to record device group membership change")
			return
		}
		httpx.JSON(w, http.StatusCreated, group)
	case hasDeviceID && r.Method == http.MethodDelete:
		group, err := h.repo.RemoveDeviceGroupMember(r.Context(), groupID, deviceID)
		if errors.Is(err, repository.ErrNotFound) {
			httpx.Error(w, http.StatusNotFound, "DEVICE_GROUP_NOT_FOUND", "device group not found")
			return
		}
		if err != nil {
			httpx.Error(w, http.StatusBadRequest, "INVALID_DEVICE_GROUP_MEMBER", err.Error())
			return
		}
		if err := h.writeAudit(r, audit.Event{
			Action:       "remove_device_group_member",
			ResourceType: "device_group",
			ResourceID:   strconv.FormatInt(group.ID, 10),
			Metadata: map[string]any{
				"group_name": group.Name,
				"device_id":  deviceID,
			},
		}); err != nil {
			httpx.Error(w, http.StatusInternalServerError, "AUDIT_WRITE_FAILED", "failed to record device group membership change")
			return
		}
		httpx.JSON(w, http.StatusOK, group)
	default:
		httpx.Error(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
	}
}

func parseDeviceGroupMemberPath(path string) (groupID int64, deviceID int64, hasDeviceID bool, ok bool) {
	rest := strings.TrimPrefix(path, "/api/v1/device-groups/")
	parts := strings.Split(strings.Trim(rest, "/"), "/")
	if len(parts) != 2 && len(parts) != 3 {
		return 0, 0, false, false
	}
	if parts[1] != "members" {
		return 0, 0, false, false
	}
	groupID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || groupID <= 0 {
		return 0, 0, false, false
	}
	if len(parts) == 2 {
		return groupID, 0, false, true
	}
	deviceID, err = strconv.ParseInt(parts[2], 10, 64)
	if err != nil || deviceID <= 0 {
		return 0, 0, false, false
	}
	return groupID, deviceID, true, true
}

func parseDevicePath(path string) (id int64, action string, ok bool) {
	rest := strings.Trim(strings.TrimPrefix(path, "/api/v1/devices/"), "/")
	if rest == "" {
		return 0, "", false
	}
	parts := strings.Split(rest, "/")
	if len(parts) != 1 && len(parts) != 2 {
		return 0, "", false
	}
	id, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || id <= 0 {
		return 0, "", false
	}
	if len(parts) == 2 {
		return id, parts[1], true
	}
	return id, "", true
}

func validateDevice(device models.Device) error {
	if strings.TrimSpace(device.RustDeskID) == "" || strings.TrimSpace(device.Name) == "" {
		return errors.New("rustdesk_id and name are required")
	}
	if device.Status != "" {
		switch device.Status {
		case models.DeviceStatusOnline, models.DeviceStatusOffline, models.DeviceStatusDisabled:
		default:
			return errors.New("status must be online, offline, or disabled")
		}
	}
	return nil
}

func (h Handler) Groups(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		groups, err := h.repo.ListDeviceGroups(r.Context())
		if err != nil {
			httpx.Error(w, http.StatusInternalServerError, "LIST_DEVICE_GROUPS_FAILED", "list device groups failed")
			return
		}
		httpx.JSON(w, http.StatusOK, groups)
	case http.MethodPost:
		var req CreateDeviceGroupRequest
		if err := httpx.DecodeJSON(r, &req); err != nil {
			httpx.Error(w, http.StatusBadRequest, "INVALID_JSON", "invalid JSON body")
			return
		}
		if strings.TrimSpace(req.Name) == "" {
			httpx.Error(w, http.StatusBadRequest, "INVALID_DEVICE_GROUP", "name is required")
			return
		}
		created, err := h.repo.CreateDeviceGroup(r.Context(), models.DeviceGroup{
			Name:            strings.TrimSpace(req.Name),
			Description:     strings.TrimSpace(req.Description),
			MemberDeviceIDs: req.MemberDeviceIDs,
		})
		if err != nil {
			httpx.Error(w, http.StatusBadRequest, "INVALID_DEVICE_GROUP", err.Error())
			return
		}
		if err := h.writeAudit(r, audit.Event{
			Action:       "create_device_group",
			ResourceType: "device_group",
			ResourceID:   strconv.FormatInt(created.ID, 10),
			Metadata: map[string]any{
				"name":         created.Name,
				"member_count": len(created.MemberDeviceIDs),
			},
		}); err != nil {
			httpx.Error(w, http.StatusInternalServerError, "AUDIT_WRITE_FAILED", "failed to record device group creation")
			return
		}
		httpx.JSON(w, http.StatusCreated, created)
	default:
		httpx.Error(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
	}
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
