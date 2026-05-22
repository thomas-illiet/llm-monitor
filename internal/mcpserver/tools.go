package mcpserver

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"llmservicemonitor/internal/config"
)

func newServer(cfg config.Config, db Store) *mcp.Server {
	owner := &server{cfg: cfg, store: db}
	s := mcp.NewServer(&mcp.Implementation{
		Name:    "llm-service-monitor",
		Title:   "LLM Service Monitor",
		Version: "v1.0.0",
	}, nil)
	s.AddTool(&mcp.Tool{
		Name:        "llm_monitor.status",
		Title:       "LLM Monitor Status",
		Description: "Return the current monitor health, model counts, and latest auth/HTTP checks.",
		InputSchema: emptyInputSchema(),
	}, owner.handleStatus)
	s.AddTool(&mcp.Tool{
		Name:        "llm_monitor.kpis",
		Title:       "LLM Monitor KPIs",
		Description: "Return latency, reliability, token, throughput, and SLO KPIs for a recent time window.",
		InputSchema: kpisInputSchema(),
	}, owner.handleKPIs)
	s.AddTool(&mcp.Tool{
		Name:        "llm_monitor.models",
		Title:       "LLM Monitor Models",
		Description: "Return the current model inventory, optionally filtered by status and capability.",
		InputSchema: modelsInputSchema(),
	}, owner.handleModels)
	s.AddTool(&mcp.Tool{
		Name:        "llm_monitor.model_performance",
		Title:       "LLM Monitor Model Performance",
		Description: "Return recent run performance aggregated by model.",
		InputSchema: modelPerformanceInputSchema(),
	}, owner.handleModelPerformance)
	return s
}
