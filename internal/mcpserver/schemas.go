package mcpserver

func emptyInputSchema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
	}
}

func kpisInputSchema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"properties": map[string]any{
			"range": map[string]any{
				"type":        "string",
				"description": "Go duration string such as 24h, 168h, or 720h. Defaults to the dashboard window.",
			},
		},
	}
}

func modelsInputSchema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"properties": map[string]any{
			"status": map[string]any{
				"type":        "string",
				"description": "Optional model status filter, for example active or inactive.",
			},
			"capability": map[string]any{
				"type":        "string",
				"description": "Optional capability filter, for example chat, embedding, skip, or unknown.",
			},
			"limit": map[string]any{
				"type":        "integer",
				"minimum":     1,
				"maximum":     maxLimit,
				"description": "Maximum number of matching models to return.",
			},
		},
	}
}

func modelPerformanceInputSchema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"properties": map[string]any{
			"range": map[string]any{
				"type":        "string",
				"description": "Go duration string such as 24h, 168h, or 720h. Defaults to the dashboard window.",
			},
			"sort": map[string]any{
				"type":        "string",
				"enum":        []string{"error_count", "success_rate", "avg_latency_ms", "p95_latency_ms", "p99_latency_ms", "model_id"},
				"description": "Sort key. Defaults to error_count.",
			},
			"limit": map[string]any{
				"type":        "integer",
				"minimum":     1,
				"maximum":     maxLimit,
				"description": "Maximum number of model rows to return.",
			},
		},
	}
}
