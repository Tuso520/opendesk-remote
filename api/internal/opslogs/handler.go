package opslogs

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/opendesk-remote/opendesk-remote/api/internal/httpx"
	"github.com/opendesk-remote/opendesk-remote/api/internal/repository"
)

type Handler struct {
	repo repository.LogsRepository
}

func NewHandler(repo repository.LogsRepository) Handler {
	return Handler{repo: repo}
}

func (h Handler) Audit(w http.ResponseWriter, r *http.Request) {
	if !httpx.Method(w, r, http.MethodGet) {
		return
	}
	filter, err := auditFilter(r)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "INVALID_LOG_FILTER", err.Error())
		return
	}
	events, err := h.repo.ListAuditEvents(r.Context(), filter)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "LIST_AUDIT_LOGS_FAILED", "list audit logs failed")
		return
	}
	httpx.JSON(w, http.StatusOK, events)
}

func (h Handler) Connections(w http.ResponseWriter, r *http.Request) {
	if !httpx.Method(w, r, http.MethodGet) {
		return
	}
	filter, err := connectionFilter(r)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "INVALID_LOG_FILTER", err.Error())
		return
	}
	logs, err := h.repo.ListConnectionLogs(r.Context(), filter)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "LIST_CONNECTION_LOGS_FAILED", "list connection logs failed")
		return
	}
	httpx.JSON(w, http.StatusOK, logs)
}

func (h Handler) FileTransfers(w http.ResponseWriter, r *http.Request) {
	if !httpx.Method(w, r, http.MethodGet) {
		return
	}
	filter, err := fileTransferFilter(r)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "INVALID_LOG_FILTER", err.Error())
		return
	}
	logs, err := h.repo.ListFileTransferLogs(r.Context(), filter)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "LIST_FILE_TRANSFER_LOGS_FAILED", "list file transfer logs failed")
		return
	}
	httpx.JSON(w, http.StatusOK, logs)
}

func (h Handler) Logins(w http.ResponseWriter, r *http.Request) {
	if !httpx.Method(w, r, http.MethodGet) {
		return
	}
	filter, err := loginFilter(r)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "INVALID_LOG_FILTER", err.Error())
		return
	}
	logs, err := h.repo.ListLoginLogs(r.Context(), filter)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "LIST_LOGIN_LOGS_FAILED", "list login logs failed")
		return
	}
	httpx.JSON(w, http.StatusOK, logs)
}

func auditFilter(r *http.Request) (repository.AuditLogFilter, error) {
	limit, err := limitParam(r)
	if err != nil {
		return repository.AuditLogFilter{}, err
	}
	offset, err := offsetParam(r)
	if err != nil {
		return repository.AuditLogFilter{}, err
	}
	from, to, err := timeRangeParams(r)
	if err != nil {
		return repository.AuditLogFilter{}, err
	}
	return repository.AuditLogFilter{
		ActorType:    queryString(r, "actor_type"),
		Action:       queryString(r, "action"),
		ResourceType: queryString(r, "resource_type"),
		ResourceID:   queryString(r, "resource_id"),
		From:         from,
		To:           to,
		Limit:        limit,
		Offset:       offset,
	}, nil
}

func connectionFilter(r *http.Request) (repository.ConnectionLogFilter, error) {
	limit, err := limitParam(r)
	if err != nil {
		return repository.ConnectionLogFilter{}, err
	}
	status := queryString(r, "status")
	if status != "" && !oneOf(status, "started", "ended", "failed", "denied") {
		return repository.ConnectionLogFilter{}, errors.New("status must be started, ended, failed, or denied")
	}
	connectionType := queryString(r, "connection_type")
	if connectionType != "" && !oneOf(connectionType, "direct", "relay", "websocket") {
		return repository.ConnectionLogFilter{}, errors.New("connection_type must be direct, relay, or websocket")
	}
	offset, err := offsetParam(r)
	if err != nil {
		return repository.ConnectionLogFilter{}, err
	}
	from, to, err := timeRangeParams(r)
	if err != nil {
		return repository.ConnectionLogFilter{}, err
	}
	return repository.ConnectionLogFilter{Status: status, ConnectionType: connectionType, From: from, To: to, Limit: limit, Offset: offset}, nil
}

func fileTransferFilter(r *http.Request) (repository.FileTransferLogFilter, error) {
	limit, err := limitParam(r)
	if err != nil {
		return repository.FileTransferLogFilter{}, err
	}
	direction := queryString(r, "direction")
	if direction != "" && !oneOf(direction, "upload", "download") {
		return repository.FileTransferLogFilter{}, errors.New("direction must be upload or download")
	}
	offset, err := offsetParam(r)
	if err != nil {
		return repository.FileTransferLogFilter{}, err
	}
	from, to, err := timeRangeParams(r)
	if err != nil {
		return repository.FileTransferLogFilter{}, err
	}
	return repository.FileTransferLogFilter{
		Direction: direction,
		Status:    queryString(r, "status"),
		From:      from,
		To:        to,
		Limit:     limit,
		Offset:    offset,
	}, nil
}

func loginFilter(r *http.Request) (repository.LoginLogFilter, error) {
	limit, err := limitParam(r)
	if err != nil {
		return repository.LoginLogFilter{}, err
	}
	status := queryString(r, "status")
	if status != "" && !oneOf(status, "succeeded", "failed", "denied") {
		return repository.LoginLogFilter{}, errors.New("status must be succeeded, failed, or denied")
	}
	offset, err := offsetParam(r)
	if err != nil {
		return repository.LoginLogFilter{}, err
	}
	from, to, err := timeRangeParams(r)
	if err != nil {
		return repository.LoginLogFilter{}, err
	}
	return repository.LoginLogFilter{Email: strings.ToLower(queryString(r, "email")), Status: status, From: from, To: to, Limit: limit, Offset: offset}, nil
}

func limitParam(r *http.Request) (int, error) {
	raw := strings.TrimSpace(r.URL.Query().Get("limit"))
	if raw == "" {
		return 500, nil
	}
	limit, err := strconv.Atoi(raw)
	if err != nil || limit <= 0 || limit > 500 {
		return 0, errors.New("limit must be between 1 and 500")
	}
	return limit, nil
}

func offsetParam(r *http.Request) (int, error) {
	raw := strings.TrimSpace(r.URL.Query().Get("offset"))
	if raw == "" {
		return 0, nil
	}
	offset, err := strconv.Atoi(raw)
	if err != nil || offset < 0 {
		return 0, errors.New("offset must be zero or positive")
	}
	return offset, nil
}

func timeRangeParams(r *http.Request) (*time.Time, *time.Time, error) {
	from, err := optionalTimeParam(r, "from")
	if err != nil {
		return nil, nil, err
	}
	to, err := optionalTimeParam(r, "to")
	if err != nil {
		return nil, nil, err
	}
	if from != nil && to != nil && from.After(*to) {
		return nil, nil, errors.New("from must be before or equal to to")
	}
	return from, to, nil
}

func optionalTimeParam(r *http.Request, key string) (*time.Time, error) {
	raw := strings.TrimSpace(r.URL.Query().Get(key))
	if raw == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return nil, errors.New(key + " must be an RFC3339 timestamp")
	}
	parsed = parsed.UTC()
	return &parsed, nil
}

func queryString(r *http.Request, key string) string {
	return strings.TrimSpace(r.URL.Query().Get(key))
}

func oneOf(value string, allowed ...string) bool {
	for _, item := range allowed {
		if value == item {
			return true
		}
	}
	return false
}
