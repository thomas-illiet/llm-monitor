package llm

import (
	"log/slog"
	"net/url"

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
	return &Client{
		baseURL:       baseURL,
		httpCheckPath: cfg.HTTPCheckPath,
		httpClient:    httpClient,
		tokenProvider: tokenProvider,
		logger:        logger,
	}, nil
}
