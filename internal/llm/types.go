package llm

import (
	"context"
	"log/slog"
	"net/http"
	"net/url"
	"time"
)

// TokenProvider supplies bearer tokens for authenticated target requests.
type TokenProvider interface {
	Token(ctx context.Context) (string, time.Time, error)
}

// Client calls an OpenAI-compatible LLM service and normalizes probe metrics.
type Client struct {
	baseURL       *url.URL
	httpCheckPath string
	httpClient    *http.Client
	tokenProvider TokenProvider
	logger        *slog.Logger
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
