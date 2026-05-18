package main

import (
	"context"
	"embed"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"llmservicemonitor/internal/api"
	"llmservicemonitor/internal/auth"
	"llmservicemonitor/internal/config"
	"llmservicemonitor/internal/llm"
	"llmservicemonitor/internal/notify"
	"llmservicemonitor/internal/schedule/runner"
	"llmservicemonitor/internal/schedule/tasks"
	"llmservicemonitor/internal/store"
)

//go:embed all:static
var staticFiles embed.FS

// main wires configuration, storage, monitoring, and the HTTP server.
func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	configPath := os.Getenv("LLM_MONITOR_CONFIG")
	if configPath == "" {
		configPath = "config.yaml"
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		logger.Error("load config", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := store.Open(ctx, cfg.Postgres.DSN)
	if err != nil {
		logger.Error("connect postgres", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := db.Migrate(ctx); err != nil {
		logger.Error("run migrations", "error", err)
		os.Exit(1)
	}

	tokenProvider, err := auth.NewProvider(cfg.Auth, cfg.Target, logger)
	if err != nil {
		logger.Error("build auth provider", "error", err)
		os.Exit(1)
	}

	llmClient, err := llm.NewClient(cfg.Target, tokenProvider, logger)
	if err != nil {
		logger.Error("build llm client", "error", err)
		os.Exit(1)
	}

	notifier, err := notify.NewSMTPNotifier(cfg.SMTP, logger)
	if err != nil {
		logger.Error("build smtp notifier", "error", err)
		os.Exit(1)
	}

	modelPlanStore := tasks.NewMemoryModelPlanStore()
	taskDeps := tasks.Dependencies{
		Config:         cfg,
		Store:          db,
		Client:         llmClient,
		Auth:           tokenProvider,
		Notifier:       notifier,
		Logger:         logger,
		ModelPlanStore: modelPlanStore,
	}
	taskRegistry, err := tasks.NewRegistry(taskDeps)
	if err != nil {
		logger.Error("build task registry", "error", err)
		os.Exit(1)
	}
	scheduler := runner.NewLocalScheduler(taskRegistry, logger, tasks.LocalScheduleGroups(taskDeps)...)
	scheduler.Start(ctx)

	staticRoot, err := fs.Sub(staticFiles, "static")
	if err != nil {
		logger.Error("load embedded frontend", "error", err)
		os.Exit(1)
	}

	handler, err := api.NewRouter(cfg, db, staticRoot, logger)
	if err != nil {
		logger.Error("build http router", "error", err)
		os.Exit(1)
	}

	server := &http.Server{
		Addr:              cfg.Server.Address,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		logger.Info("server listening", "address", cfg.Server.Address)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("http server stopped", "error", err)
			stop()
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown server", "error", err)
	}
}
