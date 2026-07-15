package auth

import (
	"errors"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/opendesk-remote/opendesk-remote/api/internal/httpx"
)

type Handler struct {
	service      *Service
	cookieSecure bool
}

func NewHandler(service *Service, cookieSecure bool) Handler {
	return Handler{service: service, cookieSecure: cookieSecure}
}

func (h Handler) Login(w http.ResponseWriter, r *http.Request) {
	if !httpx.Method(w, r, http.MethodPost) {
		return
	}
	var req LoginRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "INVALID_JSON", "invalid JSON body")
		return
	}
	req.IP = requestIP(r)
	req.UserAgent = r.UserAgent()
	resp, err := h.service.Login(r.Context(), req)
	if errors.Is(err, ErrInvalidCredentials) || errors.Is(err, ErrDisabledUser) {
		httpx.Error(w, http.StatusUnauthorized, "INVALID_CREDENTIALS", "invalid email or password")
		return
	}
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "LOGIN_FAILED", "login failed")
		return
	}
	h.setSessionCookie(w, resp.AccessToken, resp.ExpiresAt)
	httpx.JSON(w, http.StatusOK, resp)
}

func (h Handler) Logout(w http.ResponseWriter, r *http.Request) {
	if !httpx.Method(w, r, http.MethodPost) {
		return
	}
	if err := h.service.Logout(r.Context(), sessionTokenFromRequest(r)); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "LOGOUT_FAILED", "logout failed")
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.cookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
	httpx.JSON(w, http.StatusOK, map[string]bool{"logged_out": true})
}

func (h Handler) Me(w http.ResponseWriter, r *http.Request) {
	if !httpx.Method(w, r, http.MethodGet) {
		return
	}
	session, ok := SessionFromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "UNAUTHENTICATED", "authentication is required")
		return
	}
	httpx.JSON(w, http.StatusOK, MeResponse{User: session.User, ExpiresAt: session.ExpiresAt})
}

func (h Handler) setSessionCookie(w http.ResponseWriter, token string, expiresAt time.Time) {
	maxAge := int(time.Until(expiresAt).Seconds())
	if maxAge < 0 {
		maxAge = 0
	}
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   maxAge,
		Expires:  expiresAt,
		HttpOnly: true,
		Secure:   h.cookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}

func sessionTokenFromRequest(r *http.Request) string {
	if cookie, err := r.Cookie(SessionCookieName); err == nil && strings.TrimSpace(cookie.Value) != "" {
		return strings.TrimSpace(cookie.Value)
	}
	return bearerToken(r.Header.Get("Authorization"))
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
