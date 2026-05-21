package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"llmservicemonitor/internal/auth"
	"llmservicemonitor/internal/config"
	"llmservicemonitor/internal/llm"
	"llmservicemonitor/internal/notify"
	"llmservicemonitor/internal/schedule/queue"
	"llmservicemonitor/internal/schedule/tasks"
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
	taskQueue, err := queue.NewClient(cfg)
	if err != nil {
		logger.Error("connect redis queue", "error", err)
		os.Exit(1)
	}
	defer taskQueue.Close()

	taskDeps := tasks.Dependencies{
		Config:          cfg,
		Store:           db,
		Client:          llmClient,
		Auth:            tokenProvider,
		Notifier:        notifier,
		Logger:          logger,
		RecoveryTrigger: queue.NewModelRecoveryTrigger(taskQueue, db, logger),
	}
	taskRegistry, err := tasks.NewRegistry(taskDeps)
	if err != nil {
		logger.Error("build task registry", "error", err)
		os.Exit(1)
	}

	server := queue.NewServer(cfg, logger)
	mux := queue.NewServeMux(taskRegistry, logger)
	if err := server.Start(mux); err != nil {
		logger.Error("start worker", "error", err)
		os.Exit(1)
	}
	logger.Info("worker started", "queue", cfg.Asynq.Queue, "concurrency", cfg.Asynq.WorkerConcurrency)
	<-ctx.Done()
	server.Shutdown()
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
