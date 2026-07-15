package relaygrant

import (
	"context"
	"testing"
	"time"

	"github.com/opendesk-remote/opendesk-remote/api/internal/repository"
)

func TestRelayGrantIssueAndValidate(t *testing.T) {
	service := NewService([]byte("test-signing-key"), time.Minute)
	userID := int64(1)
	controllerID := int64(10)
	targetID := int64(20)

	issued, err := service.Issue(IssueRequest{
		UserID:             &userID,
		ControllerDeviceID: &controllerID,
		TargetDeviceID:     &targetID,
		AllowedRelays:      []string{"relay-a"},
		TTLSeconds:         60,
	})
	if err != nil {
		t.Fatalf("issue failed: %v", err)
	}

	validated, err := service.Validate(ValidateRequest{Token: issued.Token, Relay: "relay-a", TargetDeviceID: &targetID})
	if err != nil {
		t.Fatalf("validate failed: %v", err)
	}
	if !validated.Valid || validated.GrantID != issued.GrantID {
		t.Fatalf("unexpected validation response: %+v", validated)
	}
}

func TestRelayGrantRejectsWrongRelay(t *testing.T) {
	service := NewService([]byte("test-signing-key"), time.Minute)
	userID := int64(1)
	targetID := int64(20)
	issued, err := service.Issue(IssueRequest{UserID: &userID, TargetDeviceID: &targetID, AllowedRelays: []string{"relay-a"}, TTLSeconds: 60})
	if err != nil {
		t.Fatalf("issue failed: %v", err)
	}

	resp, err := service.Validate(ValidateRequest{Token: issued.Token, Relay: "relay-b"})
	if err == nil || resp.Valid {
		t.Fatalf("expected wrong relay rejection, got resp=%+v err=%v", resp, err)
	}
}

func TestRelayGrantValidatesRustDeskTargetID(t *testing.T) {
	service := NewService([]byte("test-signing-key"), time.Minute)
	userID := int64(1)
	issued, err := service.Issue(IssueRequest{
		UserID:           &userID,
		TargetRustDeskID: "100000001",
		AllowedRelays:    []string{"hbbr-relay-a"},
		TTLSeconds:       60,
	})
	if err != nil {
		t.Fatalf("issue failed: %v", err)
	}

	validated, err := service.Validate(ValidateRequest{
		Token:            issued.Token,
		Relay:            "hbbr-relay-a",
		TargetRustDeskID: "100000001",
	})
	if err != nil {
		t.Fatalf("validate failed: %v", err)
	}
	if !validated.Valid || validated.GrantID != issued.GrantID {
		t.Fatalf("unexpected validation response: %+v", validated)
	}
}

func TestRelayGrantRejectsWrongRustDeskTargetID(t *testing.T) {
	service := NewService([]byte("test-signing-key"), time.Minute)
	userID := int64(1)
	issued, err := service.Issue(IssueRequest{
		UserID:           &userID,
		TargetRustDeskID: "100000001",
		AllowedRelays:    []string{"hbbr-relay-a"},
		TTLSeconds:       60,
	})
	if err != nil {
		t.Fatalf("issue failed: %v", err)
	}

	resp, err := service.Validate(ValidateRequest{
		Token:            issued.Token,
		Relay:            "hbbr-relay-a",
		TargetRustDeskID: "999999999",
	})
	if err == nil || resp.Reason != "target_device_mismatch" {
		t.Fatalf("expected target mismatch, got resp=%+v err=%v", resp, err)
	}
}

func TestRelayGrantRevocation(t *testing.T) {
	service := NewService([]byte("test-signing-key"), time.Minute)
	userID := int64(1)
	targetID := int64(20)
	issued, err := service.Issue(IssueRequest{UserID: &userID, TargetDeviceID: &targetID, AllowedRelays: []string{"relay-a"}, TTLSeconds: 60})
	if err != nil {
		t.Fatalf("issue failed: %v", err)
	}
	if !service.Revoke(issued.GrantID) {
		t.Fatal("expected revoke to succeed")
	}
	resp, err := service.Validate(ValidateRequest{Token: issued.Token, Relay: "relay-a"})
	if err == nil || resp.Reason != "revoked_relay_grant" {
		t.Fatalf("expected revoked grant rejection, got resp=%+v err=%v", resp, err)
	}
}

func TestRelayGrantRejectsReplay(t *testing.T) {
	service := NewService([]byte("test-signing-key"), time.Minute)
	userID := int64(1)
	targetID := int64(20)
	issued, err := service.Issue(IssueRequest{UserID: &userID, TargetDeviceID: &targetID, AllowedRelays: []string{"relay-a"}, TTLSeconds: 60})
	if err != nil {
		t.Fatalf("issue failed: %v", err)
	}
	if _, err := service.Validate(ValidateRequest{Token: issued.Token, Relay: "relay-a"}); err != nil {
		t.Fatalf("first validate failed: %v", err)
	}
	resp, err := service.Validate(ValidateRequest{Token: issued.Token, Relay: "relay-a"})
	if err == nil || resp.Reason != "replayed_relay_grant" {
		t.Fatalf("expected replay rejection, got resp=%+v err=%v", resp, err)
	}
}

func TestRelayGrantRequiresIdentity(t *testing.T) {
	service := NewService([]byte("test-signing-key"), time.Minute)
	targetID := int64(20)
	_, err := service.Issue(IssueRequest{TargetDeviceID: &targetID, AllowedRelays: []string{"relay-a"}, TTLSeconds: 60})
	if err == nil {
		t.Fatal("expected missing identity error")
	}
}

func TestRelayGrantAllowsAuthenticatedSessionIdentity(t *testing.T) {
	service := NewService([]byte("test-signing-key"), time.Minute)
	_, err := service.Issue(IssueRequest{
		Authenticated:    true,
		TargetRustDeskID: "100000001",
		AllowedRelays:    []string{"relay-a"},
		TTLSeconds:       60,
	})
	if err != nil {
		t.Fatalf("expected authenticated session identity to issue grant, got %v", err)
	}
}

func TestRelayGrantPersistsThroughRepositoryBackedService(t *testing.T) {
	store := repository.NewMemory()
	service := NewService([]byte("test-signing-key"), time.Minute).WithStore(store)
	userID := int64(1)
	targetID := int64(20)

	issued, err := service.IssueWithContext(context.Background(), IssueRequest{
		UserID:         &userID,
		TargetDeviceID: &targetID,
		AllowedRelays:  []string{"relay-a"},
		TTLSeconds:     60,
	})
	if err != nil {
		t.Fatalf("issue failed: %v", err)
	}
	stored, err := store.FindRelayGrantByGrantID(context.Background(), issued.GrantID)
	if err != nil {
		t.Fatalf("expected persisted relay grant: %v", err)
	}
	if stored.Status != string(StatusIssued) || stored.Nonce == "" || len(stored.AllowedRelays) != 1 {
		t.Fatalf("unexpected persisted grant: %+v", stored)
	}

	restartedService := NewService([]byte("test-signing-key"), time.Minute).WithStore(store)
	validated, err := restartedService.ValidateWithContext(context.Background(), ValidateRequest{Token: issued.Token, Relay: "relay-a", TargetDeviceID: &targetID})
	if err != nil {
		t.Fatalf("repository-backed validate failed: %v", err)
	}
	if !validated.Valid || validated.GrantID != issued.GrantID {
		t.Fatalf("unexpected validation response: %+v", validated)
	}
	used, err := store.FindRelayGrantByGrantID(context.Background(), issued.GrantID)
	if err != nil {
		t.Fatalf("find used grant: %v", err)
	}
	if used.Status != string(StatusUsed) || used.UsedAt == nil {
		t.Fatalf("expected used persisted grant, got %+v", used)
	}
}
