package llm

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// TokenProvider supplies bearer tokens for authenticated target requests.
type TokenProvider interface {
	Token(ctx context.Context) (string, time.Time, error)
}

// ProviderInfo is the non-secret runtime identity of one configured provider.
type ProviderInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// ProviderClients dispatches OpenAI-compatible calls to provider-specific clients.
type ProviderClients struct {
	order     []ProviderInfo
	providers map[string]ProviderInfo
	byID      map[string]*Client
}

// Providers returns provider metadata in config order.
func (c *ProviderClients) Providers() []ProviderInfo {
	if c == nil {
		return nil
	}
	return append([]ProviderInfo(nil), c.order...)
}

// ProviderIDs returns configured provider IDs in config order.
func (c *ProviderClients) ProviderIDs() []string {
	if c == nil {
		return nil
	}
	ids := make([]string, 0, len(c.order))
	for _, provider := range c.order {
		ids = append(ids, provider.ID)
	}
	return ids
}

func (c *ProviderClients) client(providerID string) (*Client, error) {
	if c == nil {
		return nil, fmt.Errorf("llm providers are not configured")
	}
	client := c.byID[providerID]
	if client == nil {
		return nil, fmt.Errorf("llm provider %q is not configured", providerID)
	}
	return client, nil
}

// ListModels fetches the current model inventory for one provider.
func (c *ProviderClients) ListModels(ctx context.Context, providerID string) ([]ProviderModel, error) {
	client, err := c.client(providerID)
	if err != nil {
		return nil, err
	}
	return client.ListModels(ctx)
}

// HealthCheck probes one provider target path.
func (c *ProviderClients) HealthCheck(ctx context.Context, providerID string) HTTPCheckResult {
	client, err := c.client(providerID)
	if err != nil {
		return HTTPCheckResult{CheckedAt: time.Now().UTC(), Error: err.Error()}
	}
	return client.HealthCheck(ctx)
}

// RunChat sends a chat completion probe to one provider.
func (c *ProviderClients) RunChat(ctx context.Context, providerID string, run ChatRequest) RunResult {
	client, err := c.client(providerID)
	if err != nil {
		return RunResult{StartedAt: time.Now().UTC(), Error: err.Error()}
	}
	return client.RunChat(ctx, run)
}

// RunChatStream sends a streaming chat probe to one provider.
func (c *ProviderClients) RunChatStream(ctx context.Context, providerID string, run ChatRequest) RunResult {
	client, err := c.client(providerID)
	if err != nil {
		return RunResult{StartedAt: time.Now().UTC(), Error: err.Error()}
	}
	return client.RunChatStream(ctx, run)
}

// RunEmbedding sends one embedding probe to one provider.
func (c *ProviderClients) RunEmbedding(ctx context.Context, providerID, model, input string) RunResult {
	client, err := c.client(providerID)
	if err != nil {
		return RunResult{StartedAt: time.Now().UTC(), Error: err.Error()}
	}
	return client.RunEmbedding(ctx, model, input)
}

// Client calls an OpenAI-compatible LLM service and normalizes probe metrics.
type Client struct {
	baseURL            *url.URL
	httpCheckEndpoint  string
	modelsEndpoint     string
	chatEndpoint       string
	embeddingsEndpoint string
	httpClient         *http.Client
	tokenProvider      TokenProvider
	logger             *slog.Logger
}

// ProviderModel is one model object returned by the configured inventory endpoint.
type ProviderModel struct {
	ID       string         `json:"id"`
	Metadata map[string]any `json:"provider_metadata,omitempty"`
}

// ChatRequest describes one chat probe sent to the completions endpoint.
type ChatRequest struct {
	Model       string
	PromptID    string
	Prompt      string
	MaxTokens   int
	Temperature float64
}

// RunResult captures common metrics from chat, streaming chat, and embedding probes.
type RunResult struct {
	StartedAt             time.Time
	OK                    bool
	StatusCode            int
	Latency               time.Duration
	TTFT                  *time.Duration
	ITL                   *time.Duration
	TPOT                  *time.Duration
	RequestLatency        *time.Duration
	InputTokens           *int
	OutputTokens          *int
	TotalTokens           *int
	OutputTokensPerSecond *float64
	VectorDimensions      *int
	Error                 string
}

// HTTPCheckResult captures target reachability for the lightweight health check.
type HTTPCheckResult struct {
	CheckedAt  time.Time
	OK         bool
	StatusCode int
	Latency    time.Duration
	Error      string
}

// FailureSummary returns a compact reason suitable for lifecycle events.
func (r HTTPCheckResult) FailureSummary() string {
	if r.OK {
		return "ok"
	}
	status := "no HTTP status"
	if r.StatusCode > 0 {
		status = fmt.Sprintf("HTTP %d", r.StatusCode)
	}
	err := strings.TrimSpace(r.Error)
	if err == "" {
		return status
	}
	if len(err) > 240 {
		err = err[:240] + "..."
	}
	return fmt.Sprintf("%s (%s)", status, err)
}
