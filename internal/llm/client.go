package llm

import (
	"io"
	"log/slog"
	"net/url"
	"strings"

	"llmservicemonitor/internal/config"
)

// NewClient creates an OpenAI-compatible API client for the configured target.
func NewClient(cfg config.TargetConfig, tokenProvider TokenProvider, logger *slog.Logger) (*Client, error) {
	baseURL, err := url.Parse(cfg.BaseURL)
	if err != nil {
		return nil, err
	}
	httpClient, err := targetHTTPClient(cfg)
	if err != nil {
		return nil, err
	}
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	modelsEndpoint := defaultEndpoint(cfg.Endpoints.Models, "/v1/models")
	chatEndpoint := defaultEndpoint(cfg.Endpoints.Chat, "/v1/chat/completions")
	embeddingsEndpoint := defaultEndpoint(cfg.Endpoints.Embeddings, "/v1/embeddings")
	httpCheckEndpoint := strings.TrimSpace(cfg.HTTPCheckPath)
	if httpCheckEndpoint == "" {
		httpCheckEndpoint = modelsEndpoint
	}
	return &Client{
		baseURL:            baseURL,
		httpCheckEndpoint:  httpCheckEndpoint,
		modelsEndpoint:     modelsEndpoint,
		chatEndpoint:       chatEndpoint,
		embeddingsEndpoint: embeddingsEndpoint,
		httpClient:         httpClient,
		tokenProvider:      tokenProvider,
		logger:             logger,
	}, nil
}

func defaultEndpoint(configured, fallback string) string {
	configured = strings.TrimSpace(configured)
	if configured == "" {
		return fallback
	}
	return configured
}
