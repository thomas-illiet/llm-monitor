package llm

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
)

// parseChatStream consumes server-sent chat chunks and derives streaming latency metrics.
func parseChatStream(body io.Reader, start time.Time) (RunResult, error) {
	var result RunResult
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 2<<20)
	var firstTokenAt time.Time
	var previousTokenAt time.Time
	var gapSum time.Duration
	var gapCount int
	var chunkTokens int
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			break
		}
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
				Text string `json:"text"`
			} `json:"choices"`
			Usage *struct {
				PromptTokens     *int `json:"prompt_tokens"`
				CompletionTokens *int `json:"completion_tokens"`
				TotalTokens      *int `json:"total_tokens"`
			} `json:"usage"`
			Error any `json:"error"`
		}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			return result, fmt.Errorf("parse stream chunk: %w", err)
		}
		if chunk.Error != nil {
			encoded, _ := json.Marshal(chunk.Error)
			return result, fmt.Errorf("stream error: %s", trimBody(encoded))
		}
		if chunk.Usage != nil {
			result.InputTokens = chunk.Usage.PromptTokens
			result.OutputTokens = chunk.Usage.CompletionTokens
			result.TotalTokens = chunk.Usage.TotalTokens
		}
		for _, choice := range chunk.Choices {
			content := choice.Delta.Content
			if content == "" {
				content = choice.Text
			}
			if content == "" {
				continue
			}
			now := time.Now()
			chunkTokens++
			if firstTokenAt.IsZero() {
				firstTokenAt = now
				ttft := firstTokenAt.Sub(start)
				result.TTFT = &ttft
			} else {
				gapSum += now.Sub(previousTokenAt)
				gapCount++
			}
			previousTokenAt = now
		}
	}
	if err := scanner.Err(); err != nil {
		return result, err
	}
	if result.OutputTokens == nil && chunkTokens > 0 {
		result.OutputTokens = intPtr(chunkTokens)
	}
	if result.TotalTokens == nil && result.InputTokens != nil && result.OutputTokens != nil {
		result.TotalTokens = intPtr(*result.InputTokens + *result.OutputTokens)
	}
	if gapCount > 0 {
		itl := gapSum / time.Duration(gapCount)
		result.ITL = &itl
	}
	if result.TTFT != nil && result.OutputTokens != nil && *result.OutputTokens > 0 {
		elapsedAfterFirstToken := time.Since(start) - *result.TTFT
		if elapsedAfterFirstToken > 0 {
			tpot := elapsedAfterFirstToken / time.Duration(*result.OutputTokens)
			result.TPOT = &tpot
		}
	}
	return result, nil
}

// fillThroughput derives output tokens per second when token counts are available.
func fillThroughput(result *RunResult) {
	if result.OutputTokens == nil || result.Latency <= 0 {
		return
	}
	rate := float64(*result.OutputTokens) / result.Latency.Seconds()
	result.OutputTokensPerSecond = &rate
}

// intPtr returns a pointer for inline integer metric values.
func intPtr(value int) *int {
	return &value
}
