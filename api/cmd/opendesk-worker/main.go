package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/opendesk-remote/opendesk-remote/api/internal/app"
	"github.com/opendesk-remote/opendesk-remote/api/internal/buildworker"
	"github.com/opendesk-remote/opendesk-remote/api/internal/config"
)

func main() {
	if len(os.Args) < 2 || os.Args[1] != "run-once" {
		fmt.Fprintln(os.Stderr, "usage: opendesk-worker run-once")
		os.Exit(2)
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{}))
	cfg, err := config.Load()
	if err != nil {
		logger.Error("configuration failed", "error", err)
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), cfg.BuilderTimeout+30*time.Second)
	defer cancel()
	store, cleanup, err := app.NewStore(ctx, cfg, logger)
	if err != nil {
		logger.Error("store initialization failed", "error", err)
		os.Exit(1)
	}
	defer cleanup()
	worker := buildworker.New(store, buildworker.ConfigFromAPI(cfg), buildworker.CLIExecutor{})
	result, err := worker.RunOnce(ctx)
	raw, marshalErr := json.MarshalIndent(result, "", "  ")
	if marshalErr != nil {
		logger.Error("marshal result failed", "error", marshalErr)
		os.Exit(1)
	}
	fmt.Println(string(raw))
	if errors.Is(err, buildworker.ErrNoQueuedJobs) {
		return
	}
	if err != nil {
		logger.Error("build worker failed", "error", err)
		os.Exit(1)
	}
}
