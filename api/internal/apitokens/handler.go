package apitokens

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
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
	tokenhash "github.com/opendesk-remote/opendesk-remote/api/internal/tokens"
)

type Repository interface {
	repository.UserRepository
	audit.Repository
}

type Handler struct {
	repo       Repository
	hashSecret string
	now        func() time.Time
}

type CreateRequest struct {
	Name      string     `json:"name"`
	Scopes    []string   `json:"scopes"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

type Response struct {
	ID         int64      `json:"id"`
	Name       string     `json:"name"`
	Scopes     []string   `json:"scopes"`
	UserID     *int64     `json:"user_id,omitempty"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty"`
}

type CreateResponse struct {
	Response
	Token string `json:"token"`
}

func NewHandler(repo Repository, hashSecret string) Handler {
	return Handler{
		repo:       repo,
		hashSecret: hashSecret,
		now: func() time.Time {
			return time.Now().UTC()
		},
	}
}

func (h Handler) Collection(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		tokens, err := h.repo.ListAPITokens(r.Context())
		if err != nil {
			httpx.Error(w, http.StatusInternalServerError, "LIST_API_TOKENS_FAILED", "list API tokens failed")
			return
		}
		out := make([]Response, 0, len(tokens))
		for _, token := range tokens {
			response, err := tokenResponse(token)
			if err != nil {
				httpx.Error(w, http.StatusInternalServerError, "INVALID_API_TOKEN_STATE", "stored API token has invalid scopes")
				return
			}
			out = append(out, response)
		}
		httpx.JSON(w, http.StatusOK, out)
	case http.MethodPost:
		h.create(w, r)
	default:
		httpx.Error(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
	}
}

func (h Handler) Item(w http.ResponseWriter, r *http.Request) {
	id, action, ok := parsePath(r.URL.Path)
	if !ok || action != "revoke" {
		httpx.Error(w, http.StatusNotFound, "NOT_FOUND", "API token route not found")
		return
	}
	if !httpx.Method(w, r, http.MethodPost) {
		return
	}
	session, ok := apiauth.SessionFromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "UNAUTHENTICATED", "authentication is required")
		return
	}
	revoked, err := h.repo.RevokeAPIToken(r.Context(), id, h.now())
	if errors.Is(err, repository.ErrNotFound) {
		httpx.Error(w, http.StatusNotFound, "API_TOKEN_NOT_FOUND", "API token not found")
		return
	}
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "REVOKE_API_TOKEN_FAILED", "revoke API token failed")
		return
	}
	if err := h.writeAudit(r, session, audit.Event{
		Action:       "revoke_api_token",
		ResourceType: "api_token",
		ResourceID:   strconv.FormatInt(revoked.ID, 10),
		Metadata: map[string]any{
			"name": revoked.Name,
		},
	}); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "AUDIT_WRITE_FAILED", "failed to record API token revoke")
		return
	}
	response, err := tokenResponse(revoked)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "INVALID_API_TOKEN_STATE", "stored API token has invalid scopes")
		return
	}
	httpx.JSON(w, http.StatusOK, response)
}

func (h Handler) create(w http.ResponseWriter, r *http.Request) {
	var req CreateRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "INVALID_JSON", "invalid JSON body")
		return
	}
	session, ok := apiauth.SessionFromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "UNAUTHENTICATED", "authentication is required")
		return
	}
	name := strings.TrimSpace(req.Name)
	scopes := normalizedScopes(req.Scopes)
	if name == "" {
		httpx.Error(w, http.StatusBadRequest, "INVALID_API_TOKEN", "name is required")
		return
	}
	if len(scopes) == 0 {
		httpx.Error(w, http.StatusBadRequest, "INVALID_API_TOKEN", "at least one scope is required")
		return
	}
	if req.ExpiresAt != nil && !req.ExpiresAt.After(h.now()) {
		httpx.Error(w, http.StatusBadRequest, "INVALID_API_TOKEN", "expires_at must be in the future")
		return
	}
	rawToken, err := generateToken()
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "API_TOKEN_GENERATION_FAILED", "generate API token failed")
		return
	}
	scopesJSON, err := json.Marshal(scopes)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "INVALID_API_TOKEN", "invalid scopes")
		return
	}
	userID := session.User.ID
	created, err := h.repo.CreateAPIToken(r.Context(), models.APIToken{
		Name:       name,
		TokenHash:  tokenhash.HashToken(h.hashSecret, rawToken),
		ScopesJSON: string(scopesJSON),
		UserID:     &userID,
		ExpiresAt:  req.ExpiresAt,
	})
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "INVALID_API_TOKEN", err.Error())
		return
	}
	if err := h.writeAudit(r, session, audit.Event{
		Action:       "create_api_token",
		ResourceType: "api_token",
		ResourceID:   strconv.FormatInt(created.ID, 10),
		Metadata: map[string]any{
			"name":       created.Name,
			"scopes":     scopes,
			"expires_at": created.ExpiresAt,
		},
	}); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "AUDIT_WRITE_FAILED", "failed to record API token creation")
		return
	}
	response, err := tokenResponse(created)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "INVALID_API_TOKEN_STATE", "stored API token has invalid scopes")
		return
	}
	httpx.JSON(w, http.StatusCreated, CreateResponse{Response: response, Token: rawToken})
}

func tokenResponse(token models.APIToken) (Response, error) {
	var scopes []string
	if err := json.Unmarshal([]byte(token.ScopesJSON), &scopes); err != nil {
		return Response{}, err
	}
	return Response{
		ID:         token.ID,
		Name:       token.Name,
		Scopes:     scopes,
		UserID:     token.UserID,
		ExpiresAt:  token.ExpiresAt,
		LastUsedAt: token.LastUsedAt,
		CreatedAt:  token.CreatedAt,
		RevokedAt:  token.RevokedAt,
	}, nil
}

func normalizedScopes(in []string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, scope := range in {
		scope = strings.TrimSpace(scope)
		if scope == "" || seen[scope] {
			continue
		}
		seen[scope] = true
		out = append(out, scope)
	}
	return out
}

func generateToken() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return "odrt_" + base64.RawURLEncoding.EncodeToString(b[:]), nil
}

func parsePath(path string) (int64, string, bool) {
	rest := strings.TrimPrefix(path, "/api/v1/api-tokens/")
	parts := strings.Split(strings.Trim(rest, "/"), "/")
	if len(parts) != 2 {
		return 0, "", false
	}
	id, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || id <= 0 {
		return 0, "", false
	}
	return id, parts[1], true
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
