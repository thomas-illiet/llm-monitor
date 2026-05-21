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
