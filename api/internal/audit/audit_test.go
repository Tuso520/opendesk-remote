package audit

import (
	"context"
	"testing"

	"github.com/opendesk-remote/opendesk-remote/api/internal/repository"
)

func TestRepositoryWriterPersistsAuditEvent(t *testing.T) {
	store := repository.NewMemory()
	writer := RepositoryWriter{Repo: store}
	userID := int64(42)

	err := writer.Write(context.Background(), Event{
		ActorUserID:  &userID,
		ActorType:    "user",
		Action:       "create_build_job",
		ResourceType: "build_job",
		ResourceID:   "job-1",
		IP:           "203.0.113.10",
		UserAgent:    "opendesk-test",
		Metadata: map[string]any{
			"platform": "windows_x64",
		},
	})
	if err != nil {
		t.Fatalf("write audit event: %v", err)
	}

	events, err := store.ListAuditEvents(context.Background(), repository.AuditLogFilter{Action: "create_build_job"})
	if err != nil {
		t.Fatalf("list audit events: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected one audit event, got %d", len(events))
	}
	event := events[0]
	if event.ActorUserID == nil || *event.ActorUserID != userID {
		t.Fatalf("expected actor user id %d, got %+v", userID, event.ActorUserID)
	}
	if event.ResourceID == nil || *event.ResourceID != "job-1" {
		t.Fatalf("expected resource id job-1, got %+v", event.ResourceID)
	}
	if event.Metadata["platform"] != "windows_x64" {
		t.Fatalf("expected metadata to round-trip, got %+v", event.Metadata)
	}
	if event.CreatedAt.IsZero() {
		t.Fatal("expected created_at to be set")
	}
}
