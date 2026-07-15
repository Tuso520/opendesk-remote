package relaygrant

import (
	"context"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/opendesk-remote/opendesk-remote/api/internal/audit"
	apiauth "github.com/opendesk-remote/opendesk-remote/api/internal/auth"
	"github.com/opendesk-remote/opendesk-remote/api/internal/httpx"
	"github.com/opendesk-remote/opendesk-remote/api/internal/models"
)

type ConnectionLogWriter interface {
	CreateConnectionLog(ctx context.Context, log models.ConnectionLog) (models.ConnectionLog, error)
}

type Handler struct {
	service             *Service
	auditWriter         audit.Writer
	connectionLogWriter ConnectionLogWriter
}

func NewHandler(service *Service, auditWriters ...audit.Writer) Handler {
	var writer audit.Writer
	if len(auditWriters) > 0 {
		writer = auditWriters[0]
	}
	return Handler{service: service, auditWriter: writer}
}

func (h Handler) WithConnectionLogWriter(writer ConnectionLogWriter) Handler {
	h.connectionLogWriter = writer
	return h
}

func (h Handler) Issue(w http.ResponseWriter, r *http.Request) {
	if !httpx.Method(w, r, http.MethodPost) {
		return
	}
	var req IssueRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "INVALID_JSON", "invalid JSON body")
		return
	}
	if session, ok := apiauth.SessionFromContext(r.Context()); ok {
		req.Authenticated = true
		if req.UserID == nil {
			userID := session.User.ID
			req.UserID = &userID
		}
	}
	resp, err := h.service.IssueWithContext(r.Context(), req)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "INVALID_RELAY_GRANT_REQUEST", err.Error())
		return
	}
	if err := h.writeAudit(r, audit.Event{
		Action:       "issue_relay_grant",
		ResourceType: "relay_grant",
		ResourceID:   resp.GrantID,
		Metadata: map[string]any{
			"controller_device_id": req.ControllerDeviceID,
			"target_device_id":     req.TargetDeviceID,
			"target_rustdesk_id":   req.TargetRustDeskID,
			"relay_id":             req.RelayID,
			"allowed_relays":       req.AllowedRelays,
			"expires_at":           resp.ExpiresAt,
		},
	}); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "AUDIT_WRITE_FAILED", "failed to record relay grant issue")
		return
	}
	httpx.JSON(w, http.StatusCreated, resp)
}

func (h Handler) Validate(w http.ResponseWriter, r *http.Request) {
	if !httpx.Method(w, r, http.MethodPost) {
		return
	}
	var req ValidateRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "INVALID_JSON", "invalid JSON body")
		return
	}
	resp, err := h.service.ValidateWithContext(r.Context(), req)
	if err != nil {
		if strings.TrimSpace(req.Token) == "" {
			resp.Reason = "relay_auth_required"
		}
		if auditErr := h.writeAudit(r, audit.Event{
			Action:       "validate_relay_grant_failed",
			ResourceType: "relay_grant",
			ResourceID:   resp.GrantID,
			Metadata: map[string]any{
				"reason":             resp.Reason,
				"status":             resp.Status,
				"relay":              req.Relay,
				"target_device_id":   req.TargetDeviceID,
				"target_rustdesk_id": req.TargetRustDeskID,
			},
		}); auditErr != nil {
			httpx.Error(w, http.StatusInternalServerError, "AUDIT_WRITE_FAILED", "failed to record relay grant validation failure")
			return
		}
		if logErr := h.writeDeniedConnectionLog(r.Context(), req, resp); logErr != nil {
			httpx.Error(w, http.StatusInternalServerError, "CONNECTION_LOG_WRITE_FAILED", "failed to record relay denial")
			return
		}
		httpx.JSON(w, http.StatusUnauthorized, resp)
		return
	}
	httpx.JSON(w, http.StatusOK, resp)
}

func (h Handler) RevokeByPath(w http.ResponseWriter, r *http.Request) {
	if !httpx.Method(w, r, http.MethodPost) {
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/relay-grants/")
	grantID, ok := strings.CutSuffix(path, "/revoke")
	if !ok || strings.TrimSpace(grantID) == "" {
		httpx.Error(w, http.StatusNotFound, "NOT_FOUND", "relay grant route not found")
		return
	}
	if !h.service.RevokeWithContext(r.Context(), grantID) {
		httpx.Error(w, http.StatusNotFound, "RELAY_GRANT_NOT_FOUND", "relay grant not found")
		return
	}
	if err := h.writeAudit(r, audit.Event{
		Action:       "revoke_relay_grant",
		ResourceType: "relay_grant",
		ResourceID:   grantID,
	}); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "AUDIT_WRITE_FAILED", "failed to record relay grant revoke")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]bool{"revoked": true})
}

func (h Handler) Revoke(w http.ResponseWriter, r *http.Request) {
	if !httpx.Method(w, r, http.MethodPost) {
		return
	}
	var req RevokeRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "INVALID_JSON", "invalid JSON body")
		return
	}
	if !h.service.RevokeWithContext(r.Context(), req.GrantID) {
		httpx.Error(w, http.StatusNotFound, "RELAY_GRANT_NOT_FOUND", "relay grant not found")
		return
	}
	if err := h.writeAudit(r, audit.Event{
		Action:       "revoke_relay_grant",
		ResourceType: "relay_grant",
		ResourceID:   req.GrantID,
	}); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "AUDIT_WRITE_FAILED", "failed to record relay grant revoke")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]bool{"revoked": true})
}

func (h Handler) writeAudit(r *http.Request, event audit.Event) error {
	if h.auditWriter == nil {
		return nil
	}
	event.ActorType = "system"
	if session, ok := apiauth.SessionFromContext(r.Context()); ok {
		event.ActorType = apiauth.ActorType(session)
		userID := session.User.ID
		event.ActorUserID = &userID
	}
	event.IP = requestIP(r)
	event.UserAgent = r.UserAgent()
	return h.auditWriter.Write(r.Context(), event)
}

func (h Handler) writeDeniedConnectionLog(ctx context.Context, req ValidateRequest, resp ValidateResponse) error {
	if h.connectionLogWriter == nil {
		return nil
	}
	denyReason := "invalid_relay_grant"
	if resp.Reason == "relay_auth_required" {
		denyReason = "relay_auth_required"
	}
	_, err := h.connectionLogWriter.CreateConnectionLog(ctx, models.ConnectionLog{
		TargetDeviceID: req.TargetDeviceID,
		ConnectionType: "relay",
		StartedAt:      time.Now().UTC(),
		Status:         "denied",
		DenyReason:     denyReason,
		Metadata: map[string]any{
			"grant_id":           resp.GrantID,
			"reason":             resp.Reason,
			"relay":              req.Relay,
			"target_rustdesk_id": req.TargetRustDeskID,
		},
	})
	return err
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
