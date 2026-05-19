package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ListModels fetches the current model identifiers from /v1/models.
func (c *Client) ListModels(ctx context.Context) ([]string, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/v1/models", nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("list models returned %d: %s", resp.StatusCode, trimBody(body))
	}
	var decoded struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &decoded); err != nil {
		return nil, err
	}
	models := make([]string, 0, len(decoded.Data))
	for _, model := range decoded.Data {
		if model.ID != "" {
			models = append(models, model.ID)
		}
	}
	return models, nil
}

// HealthCheck probes the configured target path and records HTTP reachability.
func (c *Client) HealthCheck(ctx context.Context) HTTPCheckResult {
	start := time.Now()
	req, err := c.newRequest(ctx, http.MethodGet, c.httpCheckPath, nil)
	if err != nil {
		return HTTPCheckResult{CheckedAt: start.UTC(), Latency: time.Since(start), Error: err.Error()}
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
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
	}
	return result
}
