package app

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"strings"

	"github.com/opendesk-remote/opendesk-remote/api/internal/audit"
	"github.com/opendesk-remote/opendesk-remote/api/internal/auth"
	"github.com/opendesk-remote/opendesk-remote/api/internal/config"
	"github.com/opendesk-remote/opendesk-remote/api/internal/database"
	"github.com/opendesk-remote/opendesk-remote/api/internal/models"
	"github.com/opendesk-remote/opendesk-remote/api/internal/repository"
)

func NewStore(ctx context.Context, cfg config.Config, logger *slog.Logger) (repository.Store, func() error, error) {
	if strings.TrimSpace(cfg.MySQLDSN) == "" {
		store := repository.NewMemoryWithoutInitialAdmin()
		if err := bootstrapInitialAdminIfConfigured(ctx, store, cfg); err != nil {
			return nil, nil, err
		}
		return store, func() error { return nil }, nil
	}
	db, err := database.OpenMySQL(cfg.MySQLDSN)
	if err != nil {
		return nil, nil, err
	}
	cleanup := db.Close
	if err := database.Ping(ctx, db); err != nil {
		_ = cleanup()
		return nil, nil, err
	}
	store := repository.NewMySQL(db)
	if err := bootstrapInitialAdminIfConfigured(ctx, store, cfg); err != nil {
		_ = cleanup()
		return nil, nil, err
	}
	logger.Info("opendesk-api using mysql repository")
	return store, cleanup, nil
}

func fallbackMemoryStore(cfg config.Config, logger *slog.Logger) repository.Store {
	store := repository.NewMemoryWithoutInitialAdmin()
	if err := bootstrapInitialAdminIfConfigured(context.Background(), store, cfg); err != nil {
		logger.Error("initial admin bootstrap failed", "error", err)
	}
	return store
}

func bootstrapInitialAdminIfConfigured(ctx context.Context, store repository.Store, cfg config.Config) error {
	email := strings.TrimSpace(cfg.InitialAdminEmail)
	password := cfg.InitialAdminPassword
	if email == "" && strings.TrimSpace(password) == "" {
		return nil
	}
	if email == "" || strings.TrimSpace(password) == "" {
		return errors.New("both initial admin email and password are required when bootstrapping initial admin")
	}
	count, err := store.CountUsers(ctx)
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	passwordHash, err := auth.HashPassword(password)
	if err != nil {
		return err
	}
	username, _, _ := strings.Cut(email, "@")
	if strings.TrimSpace(username) == "" {
		username = "admin"
	}
	created, err := store.CreateUser(ctx, models.User{
		Email:        email,
		Username:     username,
		DisplayName:  "Initial Admin",
		PasswordHash: passwordHash,
		Status:       models.UserStatusActive,
		Source:       "local",
	})
	if err != nil {
		return err
	}
	return (audit.RepositoryWriter{Repo: store}).Write(ctx, audit.Event{
		ActorType:    "system",
		Action:       "bootstrap_initial_admin",
		ResourceType: "user",
		ResourceID:   int64String(created.ID),
		Metadata: map[string]any{
			"email":    created.Email,
			"username": created.Username,
			"source":   "initial_admin_env",
		},
	})
}

func int64String(value int64) string {
	return strconv.FormatInt(value, 10)
}
