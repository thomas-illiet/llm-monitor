package llm

import (
	"io"
	"log/slog"
	"net/url"
	"strings"

	"llmservicemonitor/internal/config"
)

// NewProviderClients creates OpenAI-compatible API clients for configured providers.
func NewProviderClients(providerCfgs []config.ProviderConfig, tokenFor func(string) TokenProvider, logger *slog.Logger) (*ProviderClients, error) {
	clients := &ProviderClients{
		order:     make([]ProviderInfo, 0, len(providerCfgs)),
		byID:      make(map[string]*Client, len(providerCfgs)),
		providers: make(map[string]ProviderInfo, len(providerCfgs)),
	}
	for _, providerCfg := range providerCfgs {
		client, err := NewClient(providerCfg, tokenFor(providerCfg.ID), logger)
		if err != nil {
			return nil, err
		}
		info := ProviderInfo{ID: providerCfg.ID, Name: providerCfg.Name}
		clients.order = append(clients.order, info)
		clients.providers[providerCfg.ID] = info
		clients.byID[providerCfg.ID] = client
	}
	return clients, nil
}

// NewClient creates an OpenAI-compatible API client for the configured provider.
func NewClient(cfg config.ProviderConfig, tokenProvider TokenProvider, logger *slog.Logger) (*Client, error) {
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
