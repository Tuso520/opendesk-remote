package relaygrant

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/opendesk-remote/opendesk-remote/api/internal/models"
)

var (
	ErrInvalidToken = errors.New("invalid relay grant")
	ErrExpired      = errors.New("relay grant expired")
	ErrRevoked      = errors.New("relay grant revoked")
)

type Status string

const (
	StatusIssued  Status = "issued"
	StatusUsed    Status = "used"
	StatusExpired Status = "expired"
	StatusRevoked Status = "revoked"
)

type IssueRequest struct {
	UserID             *int64   `json:"user_id,omitempty"`
	ControllerDeviceID *int64   `json:"controller_device_id,omitempty"`
	TargetDeviceID     *int64   `json:"target_device_id,omitempty"`
	TargetRustDeskID   string   `json:"target_rustdesk_id,omitempty"`
	RelayID            *int64   `json:"relay_id,omitempty"`
	AllowedRelays      []string `json:"allowed_relays"`
	TTLSeconds         int      `json:"ttl_seconds,omitempty"`
	Authenticated      bool     `json:"-"`
}

type IssueResponse struct {
	GrantID   string    `json:"grant_id"`
	Token     string    `json:"grant_token"`
	ExpiresAt time.Time `json:"expires_at"`
}

type ValidateRequest struct {
	Token            string `json:"grant_token"`
	Relay            string `json:"relay,omitempty"`
	TargetDeviceID   *int64 `json:"target_device_id,omitempty"`
	TargetRustDeskID string `json:"target_rustdesk_id,omitempty"`
}

type ValidateResponse struct {
	Valid     bool      `json:"valid"`
	GrantID   string    `json:"grant_id,omitempty"`
	Status    Status    `json:"status,omitempty"`
	ExpiresAt time.Time `json:"expires_at,omitempty"`
	Reason    string    `json:"reason,omitempty"`
}

type RevokeRequest struct {
	GrantID string `json:"grant_id"`
}

type Grant struct {
	GrantID            string     `json:"grant_id"`
	UserID             *int64     `json:"user_id,omitempty"`
	ControllerDeviceID *int64     `json:"controller_device_id,omitempty"`
	TargetDeviceID     *int64     `json:"target_device_id,omitempty"`
	TargetRustDeskID   string     `json:"target_rustdesk_id,omitempty"`
	RelayID            *int64     `json:"relay_id,omitempty"`
	AllowedRelays      []string   `json:"allowed_relays"`
	ExpiresAt          time.Time  `json:"expires_at"`
	Nonce              string     `json:"nonce"`
	Status             Status     `json:"status"`
	CreatedAt          time.Time  `json:"created_at"`
	UsedAt             *time.Time `json:"used_at,omitempty"`
}

type tokenPayload struct {
	GrantID            string   `json:"grant_id"`
	UserID             *int64   `json:"user_id,omitempty"`
	ControllerDeviceID *int64   `json:"controller_device_id,omitempty"`
	TargetDeviceID     *int64   `json:"target_device_id,omitempty"`
	TargetRustDeskID   string   `json:"target_rustdesk_id,omitempty"`
	RelayID            *int64   `json:"relay_id,omitempty"`
	AllowedRelays      []string `json:"allowed_relays"`
	ExpiresAt          int64    `json:"expires_at"`
	Nonce              string   `json:"nonce"`
}

type Service struct {
	key        []byte
	defaultTTL time.Duration
	mu         sync.Mutex
	grants     map[string]*Grant
	store      GrantStore
}

type GrantStore interface {
	CreateRelayGrant(ctx context.Context, grant models.RelayGrant) (models.RelayGrant, error)
	FindRelayGrantByGrantID(ctx context.Context, grantID string) (models.RelayGrant, error)
	UpdateRelayGrantStatus(ctx context.Context, grantID string, status string, usedAt *time.Time) (models.RelayGrant, error)
}

func NewService(key []byte, defaultTTL time.Duration) *Service {
	if defaultTTL == 0 {
		defaultTTL = 2 * time.Minute
	}
	return &Service{key: key, defaultTTL: defaultTTL, grants: map[string]*Grant{}}
}

func (s *Service) WithStore(store GrantStore) *Service {
	s.store = store
	return s
}

func (s *Service) Issue(req IssueRequest) (IssueResponse, error) {
	return s.IssueWithContext(context.Background(), req)
}

func (s *Service) IssueWithContext(ctx context.Context, req IssueRequest) (IssueResponse, error) {
	if req.UserID == nil && req.ControllerDeviceID == nil && !req.Authenticated {
		return IssueResponse{}, errors.New("relay grant requires user or managed device identity")
	}
	req.TargetRustDeskID = strings.TrimSpace(req.TargetRustDeskID)
	if req.TargetDeviceID == nil && req.TargetRustDeskID == "" {
		return IssueResponse{}, errors.New("target_device_id or target_rustdesk_id is required")
	}
	if len(req.AllowedRelays) == 0 {
		return IssueResponse{}, errors.New("allowed_relays is required")
	}
	ttl := s.defaultTTL
	if req.TTLSeconds > 0 {
		ttl = time.Duration(req.TTLSeconds) * time.Second
	}
	if ttl < time.Minute || ttl > 5*time.Minute {
		return IssueResponse{}, errors.New("ttl must be between 60 seconds and 5 minutes")
	}
	now := time.Now().UTC()
	grant := &Grant{
		GrantID:            randomID("rg"),
		UserID:             req.UserID,
		ControllerDeviceID: req.ControllerDeviceID,
		TargetDeviceID:     req.TargetDeviceID,
		TargetRustDeskID:   req.TargetRustDeskID,
		RelayID:            req.RelayID,
		AllowedRelays:      append([]string(nil), req.AllowedRelays...),
		ExpiresAt:          now.Add(ttl),
		Nonce:              randomID("nonce"),
		Status:             StatusIssued,
		CreatedAt:          now,
	}
	payload := tokenPayload{
		GrantID:            grant.GrantID,
		UserID:             grant.UserID,
		ControllerDeviceID: grant.ControllerDeviceID,
		TargetDeviceID:     grant.TargetDeviceID,
		TargetRustDeskID:   grant.TargetRustDeskID,
		RelayID:            grant.RelayID,
		AllowedRelays:      grant.AllowedRelays,
		ExpiresAt:          grant.ExpiresAt.Unix(),
		Nonce:              grant.Nonce,
	}
	token, err := s.sign(payload)
	if err != nil {
		return IssueResponse{}, err
	}
	if s.store != nil {
		created, err := s.store.CreateRelayGrant(ctx, modelFromGrant(grant))
		if err != nil {
			return IssueResponse{}, err
		}
		grant = grantFromModel(created)
	} else {
		s.mu.Lock()
		s.grants[grant.GrantID] = grant
		s.mu.Unlock()
	}
	return IssueResponse{GrantID: grant.GrantID, Token: token, ExpiresAt: grant.ExpiresAt}, nil
}

func (s *Service) Validate(req ValidateRequest) (ValidateResponse, error) {
	return s.ValidateWithContext(context.Background(), req)
}

func (s *Service) ValidateWithContext(ctx context.Context, req ValidateRequest) (ValidateResponse, error) {
	payload, err := s.parse(req.Token)
	if err != nil {
		return ValidateResponse{Valid: false, Reason: "invalid_relay_grant"}, err
	}
	grant, ok := s.findGrant(ctx, payload.GrantID)
	if !ok {
		return ValidateResponse{Valid: false, GrantID: payload.GrantID, Reason: "unknown_relay_grant"}, ErrInvalidToken
	}
	if grant.Nonce != payload.Nonce {
		return ValidateResponse{Valid: false, GrantID: grant.GrantID, Status: grant.Status, Reason: "invalid_relay_grant"}, ErrInvalidToken
	}
	if grant.Status == StatusRevoked {
		return ValidateResponse{Valid: false, GrantID: grant.GrantID, Status: grant.Status, Reason: "revoked_relay_grant"}, ErrRevoked
	}
	if grant.Status == StatusUsed {
		return ValidateResponse{Valid: false, GrantID: grant.GrantID, Status: grant.Status, Reason: "replayed_relay_grant"}, ErrInvalidToken
	}
	if grant.Status == StatusExpired {
		return ValidateResponse{Valid: false, GrantID: grant.GrantID, Status: grant.Status, Reason: "expired_relay_grant"}, ErrExpired
	}
	if time.Now().UTC().After(grant.ExpiresAt) {
		grant = s.updateGrantStatus(ctx, grant, StatusExpired, nil)
		return ValidateResponse{Valid: false, GrantID: grant.GrantID, Status: grant.Status, Reason: "expired_relay_grant"}, ErrExpired
	}
	if req.TargetDeviceID != nil && grant.TargetDeviceID != nil && *req.TargetDeviceID != *grant.TargetDeviceID {
		return ValidateResponse{Valid: false, GrantID: grant.GrantID, Status: grant.Status, Reason: "target_device_mismatch"}, ErrInvalidToken
	}
	if req.TargetRustDeskID != "" && grant.TargetRustDeskID != "" && req.TargetRustDeskID != grant.TargetRustDeskID {
		return ValidateResponse{Valid: false, GrantID: grant.GrantID, Status: grant.Status, Reason: "target_device_mismatch"}, ErrInvalidToken
	}
	if req.Relay != "" && !contains(grant.AllowedRelays, req.Relay) {
		return ValidateResponse{Valid: false, GrantID: grant.GrantID, Status: grant.Status, Reason: "relay_not_allowed"}, ErrInvalidToken
	}
	now := time.Now().UTC()
	grant = s.updateGrantStatus(ctx, grant, StatusUsed, &now)
	return ValidateResponse{Valid: true, GrantID: grant.GrantID, Status: grant.Status, ExpiresAt: grant.ExpiresAt}, nil
}

func (s *Service) Revoke(grantID string) bool {
	return s.RevokeWithContext(context.Background(), grantID)
}

func (s *Service) RevokeWithContext(ctx context.Context, grantID string) bool {
	grantID = strings.TrimSpace(grantID)
	if grantID == "" {
		return false
	}
	if s.store != nil {
		_, err := s.store.UpdateRelayGrantStatus(ctx, grantID, string(StatusRevoked), nil)
		return err == nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	grant, ok := s.grants[grantID]
	if !ok {
		return false
	}
	grant.Status = StatusRevoked
	return true
}

func (s *Service) sign(payload tokenPayload) (string, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	encoded := base64.RawURLEncoding.EncodeToString(raw)
	mac := hmac.New(sha256.New, s.key)
	mac.Write([]byte(encoded))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return encoded + "." + sig, nil
}

func (s *Service) parse(token string) (tokenPayload, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return tokenPayload{}, ErrInvalidToken
	}
	mac := hmac.New(sha256.New, s.key)
	mac.Write([]byte(parts[0]))
	expected := mac.Sum(nil)
	got, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || !hmac.Equal(expected, got) {
		return tokenPayload{}, ErrInvalidToken
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return tokenPayload{}, ErrInvalidToken
	}
	var payload tokenPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return tokenPayload{}, ErrInvalidToken
	}
	if strings.TrimSpace(payload.GrantID) == "" || strings.TrimSpace(payload.Nonce) == "" || payload.ExpiresAt <= 0 {
		return tokenPayload{}, ErrInvalidToken
	}
	return payload, nil
}

func (s *Service) findGrant(ctx context.Context, grantID string) (*Grant, bool) {
	if s.store != nil {
		grant, err := s.store.FindRelayGrantByGrantID(ctx, grantID)
		if err != nil {
			return nil, false
		}
		return grantFromModel(grant), true
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	grant, ok := s.grants[grantID]
	if !ok {
		return nil, false
	}
	return cloneGrant(grant), true
}

func (s *Service) updateGrantStatus(ctx context.Context, grant *Grant, status Status, usedAt *time.Time) *Grant {
	if s.store != nil {
		updated, err := s.store.UpdateRelayGrantStatus(ctx, grant.GrantID, string(status), usedAt)
		if err == nil {
			return grantFromModel(updated)
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if stored, ok := s.grants[grant.GrantID]; ok {
		stored.Status = status
		if usedAt != nil {
			when := usedAt.UTC()
			stored.UsedAt = &when
		}
		return cloneGrant(stored)
	}
	grant.Status = status
	if usedAt != nil {
		when := usedAt.UTC()
		grant.UsedAt = &when
	}
	return grant
}

func modelFromGrant(grant *Grant) models.RelayGrant {
	if grant == nil {
		return models.RelayGrant{}
	}
	return models.RelayGrant{
		GrantID:            grant.GrantID,
		UserID:             cloneInt64Pointer(grant.UserID),
		ControllerDeviceID: cloneInt64Pointer(grant.ControllerDeviceID),
		TargetDeviceID:     cloneInt64Pointer(grant.TargetDeviceID),
		TargetRustDeskID:   grant.TargetRustDeskID,
		RelayID:            cloneInt64Pointer(grant.RelayID),
		AllowedRelays:      append([]string(nil), grant.AllowedRelays...),
		ExpiresAt:          grant.ExpiresAt,
		Nonce:              grant.Nonce,
		Status:             string(grant.Status),
		CreatedAt:          grant.CreatedAt,
		UsedAt:             cloneTimePointer(grant.UsedAt),
	}
}

func grantFromModel(model models.RelayGrant) *Grant {
	return &Grant{
		GrantID:            model.GrantID,
		UserID:             cloneInt64Pointer(model.UserID),
		ControllerDeviceID: cloneInt64Pointer(model.ControllerDeviceID),
		TargetDeviceID:     cloneInt64Pointer(model.TargetDeviceID),
		TargetRustDeskID:   model.TargetRustDeskID,
		RelayID:            cloneInt64Pointer(model.RelayID),
		AllowedRelays:      append([]string(nil), model.AllowedRelays...),
		ExpiresAt:          model.ExpiresAt,
		Nonce:              model.Nonce,
		Status:             Status(model.Status),
		CreatedAt:          model.CreatedAt,
		UsedAt:             cloneTimePointer(model.UsedAt),
	}
}

func cloneGrant(grant *Grant) *Grant {
	return grantFromModel(modelFromGrant(grant))
}

func cloneInt64Pointer(value *int64) *int64 {
	if value == nil {
		return nil
	}
	out := *value
	return &out
}

func cloneTimePointer(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	out := value.UTC()
	return &out
}

func randomID(prefix string) string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return prefix + "_" + base64.RawURLEncoding.EncodeToString(b[:])
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
