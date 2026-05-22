package mcpserver

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"llmservicemonitor/internal/config"
	"llmservicemonitor/internal/store"
)

const (
	defaultLimit = 20
	maxLimit     = 100
)

// Store describes the read-only persistence methods exposed through MCP tools.
type Store interface {
	ListModelStates(ctx context.Context) ([]store.ModelState, error)
	LatestAuthCheck(ctx context.Context) (*store.CheckRecord, error)
	LatestHTTPCheck(ctx context.Context) (*store.CheckRecord, error)
	KPISummary(ctx context.Context, since time.Time, slo store.SLOThresholds) (store.KPISummary, error)
	ModelPerformance(ctx context.Context, query store.ModelPerformanceQuery) ([]store.ModelPerformanceRow, error)
}

type server struct {
	cfg   config.Config
	store Store
}

// NewHandler builds the protected Streamable HTTP MCP handler.
func NewHandler(cfg config.Config, db Store, logger *slog.Logger) (http.Handler, error) {
	token := strings.TrimSpace(cfg.MCP.BearerToken)
	if token == "" {
		return nil, errors.New("mcp bearer token is empty")
	}

	mcpServer := newServer(cfg, db)
	handler := http.Handler(mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return mcpServer
	}, &mcp.StreamableHTTPOptions{
		JSONResponse:   true,
		Logger:         logger,
		SessionTimeout: 30 * time.Minute,
		EventStore:     nil,
		Stateless:      false,
	}))
	return bearerAuth(token, handler), nil
}
