package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"llmservicemonitor/internal/config"
	"llmservicemonitor/internal/schedule/queue"
	"llmservicemonitor/internal/store"
)

func main() {
	logger := newLogger("info")
	cfg, err := loadConfig()
	if err != nil {
		logger.Error("load config", "error", err)
		os.Exit(1)
	}
	logger = newLogger(cfg.Logging.Level)

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

	taskQueue, err := queue.NewClient(cfg)
	if err != nil {
		logger.Error("connect redis queue", "error", err)
		os.Exit(1)
	}
	defer taskQueue.Close()
	if _, err := taskQueue.EnqueueHTTPCheck(ctx); err != nil {
		logger.Error("enqueue startup http check", "error", err)
	}
	if _, err := taskQueue.EnqueueAuthCheck(ctx); err != nil {
		logger.Error("enqueue startup auth check", "error", err)
	}
	if _, err := taskQueue.EnqueueModelSnapshot(ctx); err != nil {
		logger.Error("enqueue startup model snapshot", "error", err)
	}
	if cfg.Retention.History.Duration > 0 {
		if _, err := taskQueue.EnqueueHistoryRetention(ctx); err != nil {
			logger.Error("enqueue startup history retention", "error", err)
		}
	}

	provider := queue.NewPeriodicConfigProvider(cfg, db)
	manager, err := queue.NewPeriodicTaskManager(cfg, provider, logger)
	if err != nil {
		logger.Error("build periodic task manager", "error", err)
		os.Exit(1)
	}
	if err := manager.Start(); err != nil {
		logger.Error("start periodic task manager", "error", err)
		os.Exit(1)
	}
	logger.Info("scheduler started", "queue", cfg.Asynq.Queue, "sync_interval", cfg.Asynq.SchedulerSyncInterval.Duration)
	<-ctx.Done()
	manager.Shutdown()
}

func loadConfig() (config.Config, error) {
	configPath := os.Getenv("LLM_MONITOR_CONFIG")
	if configPath == "" {
		configPath = "config.yaml"
	}
	return config.Load(configPath)
}

func newLogger(level string) *slog.Logger {
	var slogLevel slog.Level
	switch level {
	case "debug":
		slogLevel = slog.LevelDebug
	case "warn":
		slogLevel = slog.LevelWarn
	case "error":
		slogLevel = slog.LevelError
	default:
		slogLevel = slog.LevelInfo
	}
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slogLevel}))
}
