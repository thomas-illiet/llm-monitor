package main

import (
	"context"
	"embed"
	"errors"
	"io/fs"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"llmservicemonitor/internal/api"
	"llmservicemonitor/internal/app"
	"llmservicemonitor/internal/schedule/queue"
	"llmservicemonitor/internal/store"
)

//go:embed all:static
var staticFiles embed.FS

// main wires configuration, storage, monitoring, and the HTTP server.
func main() {
	logger := app.NewLogger("info")

	cfg, err := app.LoadConfig()
	if err != nil {
		logger.Error("load config", "error", err)
		os.Exit(1)
	}
	logger = app.NewLogger(cfg.Logging.Level)
	logger.Info("config loaded",
		"log_level", cfg.Logging.Level,
		"target", cfg.Target.Name,
		"target_base_url", cfg.Target.BaseURL,
		"models_endpoint", cfg.Target.Endpoints.Models,
		"chat_endpoint", cfg.Target.Endpoints.Chat,
		"embeddings_endpoint", cfg.Target.Endpoints.Embeddings,
		"http_check_endpoint", cfg.Target.HTTPCheckPath,
	)

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

	staticRoot, err := fs.Sub(staticFiles, "static")
	if err != nil {
		logger.Error("load embedded frontend", "error", err)
		os.Exit(1)
	}

	handler, err := api.NewRouter(cfg, db, staticRoot, logger, taskQueue)
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
