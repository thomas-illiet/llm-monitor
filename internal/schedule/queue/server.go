package queue

import (
	"context"
	"log/slog"

	"github.com/hibiken/asynq"

	"llmservicemonitor/internal/config"
)

// NewServer creates the Asynq worker server for the configured queue.
func NewServer(cfg config.Config, logger *slog.Logger) *asynq.Server {
	return asynq.NewServer(RedisOpt(cfg), asynq.Config{
		Concurrency: cfg.Asynq.WorkerConcurrency,
		Queues: map[string]int{
			cfg.Asynq.Queue: 1,
		},
		Logger:   newSlogAdapter(logger),
		LogLevel: asynqLogLevel(cfg.Logging.Level),
		ErrorHandler: asynq.ErrorHandlerFunc(func(_ context.Context, task *asynq.Task, err error) {
			if logger != nil {
				logger.Error("asynq task error", "task", task.Type(), "error", err)
			}
		}),
	})
}
