package setup

import (
	"crypto/subtle"
	"errors"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/opendesk-remote/opendesk-remote/api/internal/audit"
	"github.com/opendesk-remote/opendesk-remote/api/internal/auth"
	"github.com/opendesk-remote/opendesk-remote/api/internal/httpx"
	"github.com/opendesk-remote/opendesk-remote/api/internal/models"
	"github.com/opendesk-remote/opendesk-remote/api/internal/repository"
)

type Config struct {
	SetupToken string
}

type Handler struct {
	repo   Repository
	config Config
}

type Repository interface {
	repository.UserRepository
	audit.Repository
}

type CreateAdminRequest struct {
	Email       string `json:"email"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Password    string `json:"password"`
	SetupToken  string `json:"setup_token"`
}

func NewHandler(repo Repository, config Config) Handler {
	return Handler{repo: repo, config: config}
}

func (h Handler) Status(w http.ResponseWriter, r *http.Request) {
	if !httpx.Method(w, r, http.MethodGet) {
		return
	}
	count, err := h.repo.CountUsers(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "SETUP_STATUS_FAILED", "setup status failed")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{
		"setup_required": count == 0,
		"users_count":    count,
	})
}

func (h Handler) Admin(w http.ResponseWriter, r *http.Request) {
	if !httpx.Method(w, r, http.MethodPost) {
		return
	}
	var req CreateAdminRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "INVALID_JSON", "invalid JSON body")
		return
	}
	if !h.setupAllowed(r, req.SetupToken) {
		httpx.Error(w, http.StatusForbidden, "SETUP_NOT_ALLOWED", "setup requires a loopback request or a valid setup token")
		return
	}
	count, err := h.repo.CountUsers(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "SETUP_STATUS_FAILED", "setup status failed")
		return
	}
	if count > 0 {
		httpx.Error(w, http.StatusConflict, "SETUP_ALREADY_COMPLETE", "initial administrator already exists")
		return
	}
	if err := validateCreateAdmin(req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "INVALID_SETUP_ADMIN", err.Error())
		return
	}
	passwordHash, err := auth.HashPassword(req.Password)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "PASSWORD_HASH_FAILED", "password hash failed")
		return
	}
	username := strings.TrimSpace(req.Username)
	if username == "" {
		username, _, _ = strings.Cut(strings.TrimSpace(req.Email), "@")
	}
	if username == "" {
		username = "admin"
	}
	displayName := strings.TrimSpace(req.DisplayName)
	if displayName == "" {
		displayName = "Initial Admin"
	}
	created, err := h.repo.CreateUser(r.Context(), models.User{
		Email:        strings.TrimSpace(req.Email),
		Username:     username,
		DisplayName:  displayName,
		PasswordHash: passwordHash,
		Status:       models.UserStatusActive,
		Source:       "local",
	})
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "CREATE_SETUP_ADMIN_FAILED", err.Error())
		return
	}
	if err := (audit.RepositoryWriter{Repo: h.repo}).Write(r.Context(), audit.Event{
		ActorType:    "system",
		Action:       "setup_initial_admin",
		ResourceType: "user",
		ResourceID:   strconv.FormatInt(created.ID, 10),
		IP:           requestIP(r),
		UserAgent:    r.UserAgent(),
		Metadata: map[string]any{
			"email":    created.Email,
			"username": created.Username,
			"source":   "setup_endpoint",
		},
	}); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "AUDIT_WRITE_FAILED", "failed to record setup")
		return
	}
	httpx.JSON(w, http.StatusCreated, created)
}

func (h Handler) setupAllowed(r *http.Request, bodyToken string) bool {
	if isLoopbackRequest(r) {
		return true
	}
	expected := strings.TrimSpace(h.config.SetupToken)
	if expected == "" {
		return false
	}
	return constantTimeEqual(expected, setupTokenFromRequest(r, bodyToken))
}

func validateCreateAdmin(req CreateAdminRequest) error {
	if strings.TrimSpace(req.Email) == "" {
		return errors.New("email is required")
	}
	if weakPassword(req.Password) {
		return errors.New("password must be at least 12 characters and not a known placeholder")
	}
	return nil
}

func weakPassword(value string) bool {
	trimmed := strings.TrimSpace(strings.ToLower(value))
	switch trimmed {
	case "", "admin", "password", "test1234", "change-this-in-production":
		return true
	default:
		return len(trimmed) < 12
	}
}

func setupTokenFromRequest(r *http.Request, bodyToken string) string {
	if token := strings.TrimSpace(bodyToken); token != "" {
		return token
	}
	if token := strings.TrimSpace(r.Header.Get("X-OpenDesk-Setup-Token")); token != "" {
		return token
	}
	authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
	if strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
		return strings.TrimSpace(authHeader[7:])
	}
	return ""
}

func constantTimeEqual(expected, got string) bool {
	if expected == "" || got == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(expected), []byte(got)) == 1
}

func isLoopbackRequest(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip := net.ParseIP(strings.TrimSpace(host))
	return ip != nil && ip.IsLoopback()
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
