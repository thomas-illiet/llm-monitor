package queue

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/hibiken/asynq"

	"llmservicemonitor/internal/config"
)

const taskTimeout = 60 * time.Second

// RedisOpt converts app config into the Asynq Redis connection settings.
func RedisOpt(cfg config.Config) asynq.RedisClientOpt {
	return asynq.RedisClientOpt{
		Addr:     cfg.Redis.Addr,
		Username: cfg.Redis.Username,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	}
}

func taskOptions(cfg config.Config) []asynq.Option {
	return []asynq.Option{
		asynq.Queue(cfg.Asynq.Queue),
		asynq.MaxRetry(0),
		asynq.Timeout(taskTimeout),
	}
}

func manualTaskOptions(cfg config.Config) []asynq.Option {
	options := taskOptions(cfg)
	options = append(options,
		asynq.Deadline(time.Now().UTC().Add(taskTimeout)),
		asynq.Retention(cfg.Asynq.ManualTaskRetention.Duration),
	)
	return options
}

func asynqLogLevel(level string) asynq.LogLevel {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		return asynq.DebugLevel
	case "warn":
		return asynq.WarnLevel
	case "error":
		return asynq.ErrorLevel
	default:
		return asynq.InfoLevel
	}
}

type slogAdapter struct {
	logger *slog.Logger
}

func newSlogAdapter(logger *slog.Logger) asynq.Logger {
	if logger == nil {
		logger = slog.Default()
	}
	return slogAdapter{logger: logger}
}

func (l slogAdapter) Debug(args ...interface{}) {
	l.logger.Debug(joinLogArgs(args...))
}

func (l slogAdapter) Info(args ...interface{}) {
	l.logger.Info(joinLogArgs(args...))
}

func (l slogAdapter) Warn(args ...interface{}) {
	l.logger.Warn(joinLogArgs(args...))
}

func (l slogAdapter) Error(args ...interface{}) {
	l.logger.Error(joinLogArgs(args...))
}

func (l slogAdapter) Fatal(args ...interface{}) {
	l.logger.Error(joinLogArgs(args...))
	os.Exit(1)
}

func joinLogArgs(args ...interface{}) string {
	parts := make([]string, 0, len(args))
	for _, arg := range args {
		parts = append(parts, fmt.Sprint(arg))
	}
	return strings.Join(parts, " ")
}
