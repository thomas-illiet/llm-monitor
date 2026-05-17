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
	result, body := c.postJSON(ctx, start, "/v1/chat/completions", payload)
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
	body, err := json.Marshal(payload)
	if err != nil {
		return RunResult{StartedAt: start.UTC(), Latency: time.Since(start), Error: err.Error()}
	}
	req, err := c.newRequest(ctx, http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return RunResult{StartedAt: start.UTC(), Latency: time.Since(start), Error: err.Error()}
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return RunResult{StartedAt: start.UTC(), Latency: time.Since(start), Error: err.Error()}
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
		return RunResult{
			StartedAt:  start.UTC(),
			StatusCode: resp.StatusCode,
			Latency:    time.Since(start),
			Error:      fmt.Sprintf("llm returned %d: %s", resp.StatusCode, trimBody(respBody)),
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
		return result
	}
	result.OK = true
	fillThroughput(&result)
	return result
}
