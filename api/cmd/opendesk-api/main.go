package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/opendesk-remote/opendesk-remote/api/internal/app"
	"github.com/opendesk-remote/opendesk-remote/api/internal/config"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{}))
	cfg, err := config.Load()
	if err != nil {
		logger.Error("configuration failed", "error", err)
		os.Exit(1)
	}
	startCtx, startCancel := context.WithTimeout(context.Background(), 10*time.Second)
	store, cleanupStore, err := app.NewStore(startCtx, cfg, logger)
	startCancel()
	if err != nil {
		logger.Error("store initialization failed", "error", err)
		os.Exit(1)
	}
	defer cleanupStore()

	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           app.NewRouterWithStore(cfg, logger, store),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		logger.Info("opendesk-api listening", "addr", cfg.HTTPAddr, "env", cfg.Env)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("http server stopped", "error", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
		os.Exit(1)
	}
	logger.Info("opendesk-api stopped")
}
