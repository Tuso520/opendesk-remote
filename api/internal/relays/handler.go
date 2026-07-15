package relays

import (
	"errors"
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
)

type Handler struct {
	repo Repository
}

type Repository interface {
	repository.RelayRepository
	audit.Repository
}

func NewHandler(repo Repository) Handler {
	return Handler{repo: repo}
}

func (h Handler) Collection(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		relays, err := h.repo.ListRelays(r.Context())
		if err != nil {
			httpx.Error(w, http.StatusInternalServerError, "LIST_RELAYS_FAILED", "list relays failed")
			return
		}
		httpx.JSON(w, http.StatusOK, relays)
	case http.MethodPost:
		var relay models.Relay
		if err := httpx.DecodeJSON(r, &relay); err != nil {
			httpx.Error(w, http.StatusBadRequest, "INVALID_JSON", "invalid JSON body")
			return
		}
		created, err := h.repo.CreateRelay(r.Context(), relay)
		if err != nil {
			httpx.Error(w, http.StatusBadRequest, "INVALID_RELAY", err.Error())
			return
		}
		if err := h.writeAudit(r, audit.Event{
			Action:       "create_relay",
			ResourceType: "relay",
			ResourceID:   strconv.FormatInt(created.ID, 10),
			Metadata: map[string]any{
				"name":    created.Name,
				"region":  created.Region,
				"host":    created.Host,
				"port":    created.Port,
				"ws_port": created.WSPort,
				"status":  created.Status,
			},
		}); err != nil {
			httpx.Error(w, http.StatusInternalServerError, "AUDIT_WRITE_FAILED", "failed to record relay creation")
			return
		}
		httpx.JSON(w, http.StatusCreated, created)
	default:
		httpx.Error(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
	}
}

type HeartbeatRequest struct {
	CurrentSessions *int   `json:"current_sessions,omitempty"`
	Status          string `json:"status,omitempty"`
}

func (h Handler) Item(w http.ResponseWriter, r *http.Request) {
	id, action, ok := parseRelayPath(r.URL.Path)
	if !ok {
		httpx.Error(w, http.StatusNotFound, "NOT_FOUND", "relay route not found")
		return
	}
	if action == "" {
		if !httpx.Method(w, r, http.MethodPut) {
			return
		}
		h.update(w, r, id)
		return
	}
	if !httpx.Method(w, r, http.MethodPost) {
		return
	}
	switch action {
	case "heartbeat":
		h.heartbeat(w, r, id)
	case "disable":
		h.disable(w, r, id)
	default:
		httpx.Error(w, http.StatusNotFound, "NOT_FOUND", "relay route not found")
	}
}

func (h Handler) update(w http.ResponseWriter, r *http.Request, id int64) {
	var relay models.Relay
	if err := httpx.DecodeJSON(r, &relay); err != nil {
		httpx.Error(w, http.StatusBadRequest, "INVALID_JSON", "invalid JSON body")
		return
	}
	if err := validateRelayUpdate(relay); err != nil {
		httpx.Error(w, http.StatusBadRequest, "INVALID_RELAY", err.Error())
		return
	}
	updated, err := h.repo.UpdateRelay(r.Context(), id, relay)
	if errors.Is(err, repository.ErrNotFound) {
		httpx.Error(w, http.StatusNotFound, "RELAY_NOT_FOUND", "relay not found")
		return
	}
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "INVALID_RELAY", err.Error())
		return
	}
	if err := h.writeAudit(r, audit.Event{
		Action:       "update_relay",
		ResourceType: "relay",
		ResourceID:   strconv.FormatInt(updated.ID, 10),
		Metadata: map[string]any{
			"name":    updated.Name,
			"region":  updated.Region,
			"host":    updated.Host,
			"port":    updated.Port,
			"ws_port": updated.WSPort,
			"status":  updated.Status,
		},
	}); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "AUDIT_WRITE_FAILED", "failed to record relay update")
		return
	}
	httpx.JSON(w, http.StatusOK, updated)
}

func (h Handler) heartbeat(w http.ResponseWriter, r *http.Request, id int64) {
	var req HeartbeatRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "INVALID_JSON", "invalid JSON body")
		return
	}
	if req.CurrentSessions != nil && *req.CurrentSessions < 0 {
		httpx.Error(w, http.StatusBadRequest, "INVALID_RELAY_HEARTBEAT", "current_sessions cannot be negative")
		return
	}
	if req.Status != "" && !allowedHeartbeatStatus(req.Status) {
		httpx.Error(w, http.StatusBadRequest, "INVALID_RELAY_HEARTBEAT", "status must be active, degraded, or offline")
		return
	}
	relay, err := h.repo.RelayHeartbeat(r.Context(), id, req.CurrentSessions, req.Status)
	if errors.Is(err, repository.ErrNotFound) {
		httpx.Error(w, http.StatusNotFound, "RELAY_NOT_FOUND", "relay not found")
		return
	}
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "RELAY_HEARTBEAT_FAILED", "relay heartbeat failed")
		return
	}
	if err := h.writeAudit(r, audit.Event{
		Action:       "relay_heartbeat",
		ResourceType: "relay",
		ResourceID:   strconv.FormatInt(relay.ID, 10),
		Metadata: map[string]any{
			"name":             relay.Name,
			"region":           relay.Region,
			"status":           relay.Status,
			"current_sessions": relay.CurrentSessions,
		},
	}); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "AUDIT_WRITE_FAILED", "failed to record relay heartbeat")
		return
	}
	httpx.JSON(w, http.StatusOK, relay)
}

func (h Handler) disable(w http.ResponseWriter, r *http.Request, id int64) {
	relay, err := h.repo.DisableRelay(r.Context(), id, time.Now().UTC())
	if errors.Is(err, repository.ErrNotFound) {
		httpx.Error(w, http.StatusNotFound, "RELAY_NOT_FOUND", "relay not found")
		return
	}
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "DISABLE_RELAY_FAILED", "disable relay failed")
		return
	}
	if err := h.writeAudit(r, audit.Event{
		Action:       "disable_relay",
		ResourceType: "relay",
		ResourceID:   strconv.FormatInt(relay.ID, 10),
		Metadata: map[string]any{
			"name":   relay.Name,
			"region": relay.Region,
			"host":   relay.Host,
			"status": relay.Status,
		},
	}); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "AUDIT_WRITE_FAILED", "failed to record relay disable")
		return
	}
	httpx.JSON(w, http.StatusOK, relay)
}

func parseRelayPath(path string) (int64, string, bool) {
	rest := strings.TrimPrefix(path, "/api/v1/relays/")
	parts := strings.Split(strings.Trim(rest, "/"), "/")
	if len(parts) != 1 && len(parts) != 2 {
		return 0, "", false
	}
	id, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || id <= 0 {
		return 0, "", false
	}
	if len(parts) == 1 {
		return id, "", true
	}
	return id, parts[1], true
}

func validateRelayUpdate(relay models.Relay) error {
	if strings.TrimSpace(relay.Name) == "" || strings.TrimSpace(relay.Region) == "" || strings.TrimSpace(relay.Host) == "" {
		return errors.New("name, region, and host are required")
	}
	if relay.Port < 0 || relay.WSPort < 0 {
		return errors.New("port and ws_port cannot be negative")
	}
	if relay.MaxBandwidthMbps != nil && *relay.MaxBandwidthMbps < 0 {
		return errors.New("max_bandwidth_mbps cannot be negative")
	}
	if relay.Status != "" && !allowedRelayStatus(relay.Status) {
		return errors.New("status must be active, degraded, offline, or disabled")
	}
	return nil
}

func allowedRelayStatus(status string) bool {
	switch status {
	case "active", "degraded", "offline", "disabled":
		return true
	default:
		return false
	}
}

func allowedHeartbeatStatus(status string) bool {
	switch status {
	case "active", "degraded", "offline":
		return true
	default:
		return false
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
