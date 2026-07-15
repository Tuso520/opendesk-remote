package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/opendesk-remote/opendesk-remote/api/internal/repository"
)

func TestLoginIssuesSignedSession(t *testing.T) {
	hash, err := HashPassword("correct-password")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	store := repository.NewMemoryWithInitialAdmin("admin@example.com", hash)
	service := NewService(store, Config{SigningKey: []byte("test-session-signing-key"), TokenTTL: time.Minute})

	resp, err := service.Login(context.Background(), LoginRequest{Email: "ADMIN@example.com", Password: "correct-password", IP: "127.0.0.1", UserAgent: "test-agent"})
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}
	if resp.AccessToken == "" {
		t.Fatal("expected access token")
	}
	claims, err := service.ParseToken(resp.AccessToken)
	if err != nil {
		t.Fatalf("parse token: %v", err)
	}
	if claims.UserID != resp.User.ID || claims.Email != "admin@example.com" {
		t.Fatalf("unexpected claims: %+v", claims)
	}
	stored, err := store.FindSessionByTokenHash(context.Background(), service.hashToken(resp.AccessToken))
	if err != nil {
		t.Fatalf("expected stored session hash: %v", err)
	}
	if stored.TokenHash == resp.AccessToken || stored.TokenHash == "" {
		t.Fatalf("session must store only token hash, got %q", stored.TokenHash)
	}
	if stored.IP != "127.0.0.1" || stored.UserAgent != "test-agent" {
		t.Fatalf("unexpected session request metadata: %+v", stored)
	}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	request.AddCookie(&http.Cookie{Name: SessionCookieName, Value: resp.AccessToken})
	session, err := service.AuthenticateRequest(request)
	if err != nil {
		t.Fatalf("authenticate stored session: %v", err)
	}
	if session.ID != stored.ID || session.User.ID != resp.User.ID {
		t.Fatalf("unexpected authenticated session: %+v", session)
	}
}

func TestLoginRejectsWrongPassword(t *testing.T) {
	hash, err := HashPassword("correct-password")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	store := repository.NewMemoryWithInitialAdmin("admin@example.com", hash)
	service := NewService(store, Config{SigningKey: []byte("test-session-signing-key"), TokenTTL: time.Minute})

	_, err = service.Login(context.Background(), LoginRequest{Email: "admin@example.com", Password: "wrong-password"})
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected invalid credentials, got %v", err)
	}
}

func TestExpiredTokenIsRejected(t *testing.T) {
	hash, err := HashPassword("correct-password")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	store := repository.NewMemoryWithInitialAdmin("admin@example.com", hash)
	service := NewService(store, Config{SigningKey: []byte("test-session-signing-key"), TokenTTL: time.Minute})
	base := time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return base }
	user, err := store.FindUserByEmail(context.Background(), "admin@example.com")
	if err != nil {
		t.Fatalf("find user: %v", err)
	}
	token, _, err := service.IssueToken(user)
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}

	service.now = func() time.Time { return base.Add(2 * time.Minute) }
	if _, err := service.ParseToken(token); !errors.Is(err, ErrExpiredToken) {
		t.Fatalf("expected expired token, got %v", err)
	}
}

func TestLogoutRevokesStoredSession(t *testing.T) {
	hash, err := HashPassword("correct-password")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	store := repository.NewMemoryWithInitialAdmin("admin@example.com", hash)
	service := NewService(store, Config{SigningKey: []byte("test-session-signing-key"), TokenTTL: time.Minute})

	resp, err := service.Login(context.Background(), LoginRequest{Email: "admin@example.com", Password: "correct-password"})
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	request.AddCookie(&http.Cookie{Name: SessionCookieName, Value: resp.AccessToken})
	if _, err := service.AuthenticateRequest(request); err != nil {
		t.Fatalf("authenticate before logout: %v", err)
	}

	if err := service.Logout(context.Background(), resp.AccessToken); err != nil {
		t.Fatalf("logout failed: %v", err)
	}
	if _, err := service.AuthenticateRequest(request); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("expected revoked session to be rejected, got %v", err)
	}
}
