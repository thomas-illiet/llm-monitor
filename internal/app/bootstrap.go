package app

import (
	"log/slog"
	"os"

	"llmservicemonitor/internal/config"
)

const defaultConfigPath = "config.yaml"

// ConfigPath returns the configured monitor config path or the local default.
func ConfigPath() string {
	if path := os.Getenv("LLM_MONITOR_CONFIG"); path != "" {
		return path
	}
	return defaultConfigPath
}

// LoadConfig reads the monitor config from the standard environment-selected path.
func LoadConfig() (config.Config, error) {
	return config.Load(ConfigPath())
}

// NewLogger creates the process logger for the configured log level.
func NewLogger(level string) *slog.Logger {
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
