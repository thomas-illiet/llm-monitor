package llm

import (
	"context"
	"encoding/json"
	"time"
)

// RunEmbedding sends one embedding probe and extracts token and vector metrics.
func (c *Client) RunEmbedding(ctx context.Context, model, input string) RunResult {
	start := time.Now()
	payload := map[string]any{
		"model": model,
		"input": input,
	}
	result, body := c.postJSON(ctx, start, "/v1/embeddings", payload)
	if !result.OK {
		return result
	}
	var decoded struct {
		Data []struct {
			Embedding []float64 `json:"embedding"`
		} `json:"data"`
		Usage struct {
			PromptTokens *int `json:"prompt_tokens"`
			TotalTokens  *int `json:"total_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(body, &decoded); err != nil {
		result.OK = false
		result.Error = err.Error()
		return result
	}
	result.InputTokens = decoded.Usage.PromptTokens
	result.TotalTokens = decoded.Usage.TotalTokens
	if len(decoded.Data) > 0 {
		dimensions := len(decoded.Data[0].Embedding)
		result.VectorDimensions = &dimensions
	}
	return result
}
