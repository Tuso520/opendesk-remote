package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/opendesk-remote/opendesk-remote/api/internal/httpx"
	"github.com/opendesk-remote/opendesk-remote/api/internal/models"
	"github.com/opendesk-remote/opendesk-remote/api/internal/repository"
	tokenhash "github.com/opendesk-remote/opendesk-remote/api/internal/tokens"
)

const SessionCookieName = "opendesk_session"

var (
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrInvalidToken       = errors.New("invalid session token")
	ErrExpiredToken       = errors.New("expired session token")
	ErrDisabledUser       = errors.New("user is not active")
)

type UserStore interface {
	FindUserByEmail(ctx context.Context, email string) (models.User, error)
	FindUserByID(ctx context.Context, id int64) (models.User, error)
	RecordUserLogin(ctx context.Context, id int64, at time.Time) (models.User, error)
	FindAPITokenByHash(ctx context.Context, tokenHash string) (models.APIToken, error)
	RecordAPITokenUsed(ctx context.Context, id int64, at time.Time) (models.APIToken, error)
	CreateSession(ctx context.Context, session models.Session) (models.Session, error)
	FindSessionByTokenHash(ctx context.Context, tokenHash string) (models.Session, error)
	RevokeSession(ctx context.Context, tokenHash string, at time.Time) (models.Session, error)
	CreateLoginLog(ctx context.Context, log models.LoginLog) (models.LoginLog, error)
}

type Config struct {
	SigningKey []byte
	TokenTTL   time.Duration
}

type Service struct {
	store UserStore
	key   []byte
	ttl   time.Duration
	now   func() time.Time
}

type LoginRequest struct {
	Email     string `json:"email"`
	Password  string `json:"password"`
	IP        string `json:"-"`
	UserAgent string `json:"-"`
}

type LoginResponse struct {
	User        models.User `json:"user"`
	AccessToken string      `json:"access_token"`
	ExpiresAt   time.Time   `json:"expires_at"`
}

type MeResponse struct {
	User      models.User `json:"user"`
	ExpiresAt time.Time   `json:"expires_at"`
}

type Session struct {
	ID        int64
	User      models.User
	Claims    Claims
	Token     string
	ExpiresAt time.Time
	ActorType string
	APIToken  *models.APIToken
}

type Claims struct {
	UserID    int64  `json:"uid"`
	Email     string `json:"email"`
	Role      string `json:"role"`
	IssuedAt  int64  `json:"iat"`
	ExpiresAt int64  `json:"exp"`
	Nonce     string `json:"nonce"`
}

type contextKey struct{}

func NewService(store UserStore, cfg Config) *Service {
	ttl := cfg.TokenTTL
	if ttl == 0 {
		ttl = 15 * time.Minute
	}
	key := cfg.SigningKey
	if len(key) == 0 {
		key = []byte("dev-only-opendesk-session-signing-key-change-me")
	}
	return &Service{
		store: store,
		key:   append([]byte(nil), key...),
		ttl:   ttl,
		now: func() time.Time {
			return time.Now().UTC()
		},
	}
}

func (s *Service) Login(ctx context.Context, req LoginRequest) (LoginResponse, error) {
	email := strings.ToLower(strings.TrimSpace(req.Email))
	if email == "" || req.Password == "" {
		s.recordLoginLog(ctx, models.LoginLog{
			Email:         emailOrUnknown(email),
			Status:        "failed",
			FailureReason: "invalid_credentials",
			IP:            req.IP,
			UserAgent:     req.UserAgent,
			CreatedAt:     s.now(),
		})
		return LoginResponse{}, ErrInvalidCredentials
	}
	user, err := s.store.FindUserByEmail(ctx, email)
	if errors.Is(err, repository.ErrNotFound) {
		s.recordLoginLog(ctx, models.LoginLog{
			Email:         email,
			Status:        "failed",
			FailureReason: "invalid_credentials",
			IP:            req.IP,
			UserAgent:     req.UserAgent,
			CreatedAt:     s.now(),
		})
		return LoginResponse{}, ErrInvalidCredentials
	}
	if err != nil {
		return LoginResponse{}, err
	}
	if !VerifyPassword(user.PasswordHash, req.Password) {
		s.recordLoginLog(ctx, loginLogForUser(user, "failed", "invalid_credentials", req, s.now()))
		return LoginResponse{}, ErrInvalidCredentials
	}
	if user.Status != models.UserStatusActive {
		s.recordLoginLog(ctx, loginLogForUser(user, "denied", "user_not_active", req, s.now()))
		return LoginResponse{}, ErrDisabledUser
	}
	updated, err := s.store.RecordUserLogin(ctx, user.ID, s.now())
	if err == nil {
		user = updated
	}
	token, expiresAt, err := s.IssueToken(user)
	if err != nil {
		return LoginResponse{}, err
	}
	if _, err := s.store.CreateSession(ctx, models.Session{
		UserID:    user.ID,
		TokenHash: s.hashToken(token),
		IP:        req.IP,
		UserAgent: req.UserAgent,
		ExpiresAt: expiresAt,
	}); err != nil {
		s.recordLoginLog(ctx, loginLogForUser(user, "failed", "session_create_failed", req, s.now()))
		return LoginResponse{}, err
	}
	s.recordLoginLog(ctx, loginLogForUser(user, "succeeded", "", req, s.now()))
	return LoginResponse{User: user, AccessToken: token, ExpiresAt: expiresAt}, nil
}

func (s *Service) IssueToken(user models.User) (string, time.Time, error) {
	now := s.now()
	expiresAt := now.Add(s.ttl)
	claims := Claims{
		UserID:    user.ID,
		Email:     user.Email,
		Role:      "admin",
		IssuedAt:  now.Unix(),
		ExpiresAt: expiresAt.Unix(),
		Nonce:     randomID("sess"),
	}
	raw, err := json.Marshal(claims)
	if err != nil {
		return "", time.Time{}, err
	}
	encoded := base64.RawURLEncoding.EncodeToString(raw)
	mac := hmac.New(sha256.New, s.key)
	mac.Write([]byte(encoded))
	signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return encoded + "." + signature, expiresAt, nil
}

func (s *Service) AuthenticateRequest(r *http.Request) (Session, error) {
	token := bearerToken(r.Header.Get("Authorization"))
	if token != "" {
		session, err := s.authenticateSessionToken(r.Context(), token)
		if err == nil {
			return session, nil
		}
		apiSession, apiErr := s.authenticateAPIToken(r.Context(), token)
		if apiErr == nil {
			return apiSession, nil
		}
		return Session{}, err
	}
	cookie, err := r.Cookie(SessionCookieName)
	if err != nil || strings.TrimSpace(cookie.Value) == "" {
		return Session{}, ErrInvalidToken
	}
	return s.authenticateSessionToken(r.Context(), strings.TrimSpace(cookie.Value))
}

func (s *Service) authenticateSessionToken(ctx context.Context, token string) (Session, error) {
	claims, err := s.ParseToken(token)
	if err != nil {
		return Session{}, err
	}
	storedSession, err := s.store.FindSessionByTokenHash(ctx, s.hashToken(token))
	if errors.Is(err, repository.ErrNotFound) {
		return Session{}, ErrInvalidToken
	}
	if err != nil {
		return Session{}, err
	}
	if storedSession.UserID != claims.UserID {
		return Session{}, ErrInvalidToken
	}
	if storedSession.RevokedAt != nil {
		return Session{}, ErrInvalidToken
	}
	if !storedSession.ExpiresAt.After(s.now()) {
		return Session{}, ErrExpiredToken
	}
	user, err := s.store.FindUserByID(ctx, claims.UserID)
	if err != nil {
		return Session{}, ErrInvalidToken
	}
	if user.Status != models.UserStatusActive {
		return Session{}, ErrDisabledUser
	}
	return Session{
		ID:        storedSession.ID,
		User:      user,
		Claims:    claims,
		Token:     token,
		ExpiresAt: storedSession.ExpiresAt,
		ActorType: "user",
	}, nil
}

func (s *Service) authenticateAPIToken(ctx context.Context, rawToken string) (Session, error) {
	apiToken, err := s.store.FindAPITokenByHash(ctx, tokenhash.HashToken(string(s.key), rawToken))
	if errors.Is(err, repository.ErrNotFound) {
		return Session{}, ErrInvalidToken
	}
	if err != nil {
		return Session{}, err
	}
	now := s.now()
	if apiToken.RevokedAt != nil {
		return Session{}, ErrInvalidToken
	}
	if apiToken.ExpiresAt != nil && !apiToken.ExpiresAt.After(now) {
		return Session{}, ErrExpiredToken
	}
	if apiToken.UserID == nil {
		return Session{}, ErrInvalidToken
	}
	user, err := s.store.FindUserByID(ctx, *apiToken.UserID)
	if err != nil {
		return Session{}, ErrInvalidToken
	}
	if user.Status != models.UserStatusActive {
		return Session{}, ErrDisabledUser
	}
	updated, err := s.store.RecordAPITokenUsed(ctx, apiToken.ID, now)
	if err == nil {
		apiToken = updated
	}
	return Session{
		User:      user,
		Token:     rawToken,
		ExpiresAt: zeroOrTime(apiToken.ExpiresAt),
		ActorType: "api_token",
		APIToken:  &apiToken,
	}, nil
}

func (s *Service) Logout(ctx context.Context, token string) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil
	}
	if _, err := s.ParseToken(token); err != nil {
		return nil
	}
	_, err := s.store.RevokeSession(ctx, s.hashToken(token), s.now())
	if errors.Is(err, repository.ErrNotFound) {
		return nil
	}
	return err
}

func (s *Service) ParseToken(token string) (Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return Claims{}, ErrInvalidToken
	}
	mac := hmac.New(sha256.New, s.key)
	mac.Write([]byte(parts[0]))
	expected := mac.Sum(nil)
	got, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || !hmac.Equal(expected, got) {
		return Claims{}, ErrInvalidToken
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return Claims{}, ErrInvalidToken
	}
	var claims Claims
	if err := json.Unmarshal(raw, &claims); err != nil {
		return Claims{}, ErrInvalidToken
	}
	if claims.UserID <= 0 || claims.ExpiresAt <= 0 {
		return Claims{}, ErrInvalidToken
	}
	if s.now().Unix() > claims.ExpiresAt {
		return Claims{}, ErrExpiredToken
	}
	return claims, nil
}

func (s *Service) Require(next http.Handler, allowPublic func(*http.Request) bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions || (allowPublic != nil && allowPublic(r)) {
			next.ServeHTTP(w, r)
			return
		}
		session, err := s.AuthenticateRequest(r)
		if err != nil {
			httpx.Error(w, http.StatusUnauthorized, "UNAUTHENTICATED", "authentication is required")
			return
		}
		next.ServeHTTP(w, r.WithContext(WithSession(r.Context(), session)))
	})
}

func WithSession(ctx context.Context, session Session) context.Context {
	return context.WithValue(ctx, contextKey{}, session)
}

func SessionFromContext(ctx context.Context) (Session, bool) {
	session, ok := ctx.Value(contextKey{}).(Session)
	return session, ok
}

func ActorType(session Session) string {
	if session.ActorType != "" {
		return session.ActorType
	}
	return "user"
}

func bearerToken(value string) string {
	parts := strings.Fields(strings.TrimSpace(value))
	if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
		return parts[1]
	}
	return ""
}

func randomID(prefix string) string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return prefix + "_" + base64.RawURLEncoding.EncodeToString(b[:])
}

func zeroOrTime(value *time.Time) time.Time {
	if value == nil {
		return time.Time{}
	}
	return *value
}

func (s *Service) hashToken(token string) string {
	return tokenhash.HashToken(string(s.key), token)
}

func (s *Service) recordLoginLog(ctx context.Context, log models.LoginLog) {
	if strings.TrimSpace(log.Email) == "" {
		log.Email = "unknown"
	}
	if log.CreatedAt.IsZero() {
		log.CreatedAt = s.now()
	}
	_, _ = s.store.CreateLoginLog(ctx, log)
}

func loginLogForUser(user models.User, status string, reason string, req LoginRequest, at time.Time) models.LoginLog {
	log := models.LoginLog{
		UserID:        user.ID,
		Email:         user.Email,
		Username:      user.Username,
		DisplayName:   user.DisplayName,
		Status:        status,
		FailureReason: reason,
		IP:            req.IP,
		UserAgent:     req.UserAgent,
		CreatedAt:     at,
	}
	if status == "succeeded" {
		when := at.UTC()
		log.LastLoginAt = &when
	}
	return log
}

func emailOrUnknown(email string) string {
	if strings.TrimSpace(email) == "" {
		return "unknown"
	}
	return email
}
