package audit

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/opendesk-remote/opendesk-remote/api/internal/models"
)

type Event struct {
	ActorUserID  *int64
	ActorType    string
	Action       string
	ResourceType string
	ResourceID   string
	IP           string
	UserAgent    string
	Metadata     map[string]any
	CreatedAt    time.Time
}

type Writer interface {
	Write(context.Context, Event) error
}

type Repository interface {
	CreateAuditEvent(ctx context.Context, event models.AuditEvent) (models.AuditEvent, error)
}

type RepositoryWriter struct {
	Repo Repository
}

func (w RepositoryWriter) Write(ctx context.Context, event Event) error {
	if w.Repo == nil {
		return errors.New("audit repository is required")
	}
	_, err := w.Repo.CreateAuditEvent(ctx, ToModel(event))
	return err
}

type MemoryWriter struct {
	mu     sync.Mutex
	Events []Event
}

func (w *MemoryWriter) Write(_ context.Context, event Event) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	event.CreatedAt = time.Now().UTC()
	w.Events = append(w.Events, event)
	return nil
}

func ToModel(event Event) models.AuditEvent {
	var resourceID *string
	if event.ResourceID != "" {
		value := event.ResourceID
		resourceID = &value
	}
	return models.AuditEvent{
		ActorUserID:  event.ActorUserID,
		ActorType:    event.ActorType,
		Action:       event.Action,
		ResourceType: event.ResourceType,
		ResourceID:   resourceID,
		IP:           event.IP,
		UserAgent:    event.UserAgent,
		Metadata:     cloneMetadata(event.Metadata),
		CreatedAt:    event.CreatedAt,
	}
}

func cloneMetadata(metadata map[string]any) map[string]any {
	if metadata == nil {
		return map[string]any{}
	}
	out := make(map[string]any, len(metadata))
	for key, value := range metadata {
		out[key] = value
	}
	return out
}
