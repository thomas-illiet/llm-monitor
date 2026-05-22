package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"llmservicemonitor/internal/app"
	"llmservicemonitor/internal/schedule/queue"
	"llmservicemonitor/internal/store"
)

func main() {
	logger := app.NewLogger("info")
	cfg, err := app.LoadConfig()
	if err != nil {
		logger.Error("load config", "error", err)
		os.Exit(1)
	}
	logger = app.NewLogger(cfg.Logging.Level)

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
