package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// RunChat sends one chat completion probe and extracts token usage metrics.
func (c *Client) RunChat(ctx context.Context, run ChatRequest) RunResult {
	start := time.Now()
	payload := map[string]any{
		"model": run.Model,
		"messages": []map[string]string{
			{"role": "user", "content": run.Prompt},
		},
		"max_tokens":  run.MaxTokens,
		"temperature": run.Temperature,
	}
	result, body := c.postJSON(ctx, start, c.chatEndpoint, payload)
	if !result.OK {
		return result
	}
	var decoded struct {
		Usage struct {
			PromptTokens     *int `json:"prompt_tokens"`
			CompletionTokens *int `json:"completion_tokens"`
			TotalTokens      *int `json:"total_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(body, &decoded); err != nil {
		result.OK = false
		result.Error = err.Error()
		c.logger.Warn("llm chat response decode failed", "endpoint", safeEndpointLabel(c.chatEndpoint), "model", run.Model, "status", result.StatusCode, "latency_ms", millisSince(start), "error", err)
		return result
	}
	result.InputTokens = decoded.Usage.PromptTokens
	result.OutputTokens = decoded.Usage.CompletionTokens
	result.TotalTokens = decoded.Usage.TotalTokens
	return result
}

// RunChatStream sends a streaming chat probe and extracts GuideLLM-style metrics.
func (c *Client) RunChatStream(ctx context.Context, run ChatRequest) RunResult {
	start := time.Now()
	payload := map[string]any{
		"model": run.Model,
		"messages": []map[string]string{
			{"role": "user", "content": run.Prompt},
		},
		"max_tokens":     run.MaxTokens,
		"temperature":    run.Temperature,
		"stream":         true,
		"stream_options": map[string]any{"include_usage": true},
	}
	endpointLabel := safeEndpointLabel(c.chatEndpoint)
	c.logger.Debug("llm chat stream request started", "endpoint", endpointLabel, "model", run.Model)
	body, err := json.Marshal(payload)
	if err != nil {
		c.logger.Warn("llm chat stream request encode failed", "endpoint", endpointLabel, "model", run.Model, "error", err)
		return RunResult{StartedAt: start.UTC(), Latency: time.Since(start), Error: err.Error()}
	}
	req, err := c.newRequest(ctx, http.MethodPost, c.chatEndpoint, bytes.NewReader(body))
	if err != nil {
		c.logger.Warn("llm chat stream request build failed", "endpoint", endpointLabel, "model", run.Model, "error", err)
		return RunResult{StartedAt: start.UTC(), Latency: time.Since(start), Error: err.Error()}
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.logger.Warn("llm chat stream request failed", "endpoint", endpointLabel, "model", run.Model, "latency_ms", millisSince(start), "error", err)
		return RunResult{StartedAt: start.UTC(), Latency: time.Since(start), Error: err.Error()}
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
		errMessage := fmt.Sprintf("llm returned %d: %s", resp.StatusCode, trimBody(respBody))
		c.logger.Warn("llm chat stream request returned error", "endpoint", endpointLabel, "model", run.Model, "status", resp.StatusCode, "latency_ms", millisSince(start), "error", errMessage)
		return RunResult{
			StartedAt:  start.UTC(),
			StatusCode: resp.StatusCode,
			Latency:    time.Since(start),
			Error:      errMessage,
		}
	}
	result, err := parseChatStream(resp.Body, start)
	result.StartedAt = start.UTC()
	result.StatusCode = resp.StatusCode
	result.Latency = time.Since(start)
	requestLatency := result.Latency
	result.RequestLatency = &requestLatency
	if err != nil {
		result.OK = false
		result.Error = err.Error()
		c.logger.Warn("llm chat stream response parse failed", "endpoint", endpointLabel, "model", run.Model, "status", resp.StatusCode, "latency_ms", millisSince(start), "error", err)
		return result
	}
	result.OK = true
	fillThroughput(&result)
	c.logger.Debug("llm chat stream request completed", "endpoint", endpointLabel, "model", run.Model, "status", resp.StatusCode, "latency_ms", millisSince(start))
	return result
}
