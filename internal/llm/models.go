package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"llmservicemonitor/internal/metadata"
)

// ListModels fetches the current model inventory from /v1/models.
func (c *Client) ListModels(ctx context.Context) ([]ProviderModel, error) {
	start := time.Now()
	endpointLabel := safeEndpointLabel(c.modelsEndpoint)
	c.logger.Debug("llm list models request started", "endpoint", endpointLabel)
	req, err := c.newRequest(ctx, http.MethodGet, c.modelsEndpoint, nil)
	if err != nil {
		c.logger.Warn("llm list models request build failed", "endpoint", endpointLabel, "error", err)
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.logger.Warn("llm list models request failed", "endpoint", endpointLabel, "latency_ms", millisSince(start), "error", err)
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		c.logger.Warn("llm list models response read failed", "endpoint", endpointLabel, "status", resp.StatusCode, "latency_ms", millisSince(start), "error", err)
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		err := fmt.Errorf("list models returned %d: %s", resp.StatusCode, trimBody(body))
		c.logger.Warn("llm list models request returned error", "endpoint", endpointLabel, "status", resp.StatusCode, "latency_ms", millisSince(start), "error", err)
		return nil, err
	}
	var decoded struct {
		Data []map[string]any `json:"data"`
	}
	if err := json.Unmarshal(body, &decoded); err != nil {
		c.logger.Warn("llm list models response decode failed", "endpoint", endpointLabel, "status", resp.StatusCode, "latency_ms", millisSince(start), "error", err)
		return nil, err
	}
	models := make([]ProviderModel, 0, len(decoded.Data))
	for _, model := range decoded.Data {
		id, ok := model["id"].(string)
		if ok && id != "" {
			models = append(models, ProviderModel{
				ID:       id,
				Metadata: metadata.RedactProviderMetadata(model),
			})
		}
	}
	c.logger.Debug("llm list models request completed", "endpoint", endpointLabel, "status", resp.StatusCode, "latency_ms", millisSince(start), "models", len(models))
	return models, nil
}

// HealthCheck probes the configured target path and records HTTP reachability.
func (c *Client) HealthCheck(ctx context.Context) HTTPCheckResult {
	start := time.Now()
	endpointLabel := safeEndpointLabel(c.httpCheckEndpoint)
	c.logger.Debug("llm health check started", "endpoint", endpointLabel)
	req, err := c.newRequest(ctx, http.MethodGet, c.httpCheckEndpoint, nil)
	if err != nil {
		c.logger.Warn("llm health check request build failed", "endpoint", endpointLabel, "error", err)
		return HTTPCheckResult{CheckedAt: start.UTC(), Latency: time.Since(start), Error: err.Error()}
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.logger.Warn("llm health check request failed", "endpoint", endpointLabel, "latency_ms", millisSince(start), "error", err)
		return HTTPCheckResult{CheckedAt: start.UTC(), Latency: time.Since(start), Error: err.Error()}
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	result := HTTPCheckResult{
		CheckedAt:  start.UTC(),
		OK:         resp.StatusCode >= 200 && resp.StatusCode < 400,
		StatusCode: resp.StatusCode,
		Latency:    time.Since(start),
	}
	if !result.OK {
		result.Error = fmt.Sprintf("llm returned %d: %s", resp.StatusCode, trimBody(body))
		c.logger.Warn("llm health check returned unhealthy status", "endpoint", endpointLabel, "status", resp.StatusCode, "latency_ms", millisSince(start), "error", result.Error)
		return result
	}
	c.logger.Debug("llm health check completed", "endpoint", endpointLabel, "status", resp.StatusCode, "latency_ms", millisSince(start))
	return result
}
