package api

import (
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"llmservicemonitor/internal/store"
)

type modelDashboardQuery struct {
	ModelID string
	Window  time.Duration
}

// parseDashboardWindow maps a range query parameter to a dashboard window.
func parseDashboardWindow(values url.Values, fallback time.Duration) time.Duration {
	if raw := values.Get("range"); raw != "" {
		if parsed, err := time.ParseDuration(raw); err == nil {
			return parsed
		}
	}
	return fallback
}

// parseLimit parses a bounded positive limit with a generous dashboard cap.
func parseLimit(raw string, fallback int) int {
	return parseBoundedPositiveInt(raw, fallback, 1000)
}

// parseModelEventsQuery maps URL query parameters to store event filters.
func parseModelEventsQuery(values url.Values) (store.ModelEventQuery, string) {
	modelID := strings.TrimSpace(values.Get("model_id"))
	if modelID == "" {
		return store.ModelEventQuery{}, "model_id is required"
	}
	return store.ModelEventQuery{
		ModelID:    modelID,
		Limit:      parseBoundedPositiveInt(values.Get("limit"), 25, 100),
		Offset:     parseOffset(values.Get("offset")),
		Statuses:   cleanQueryValues(values["status"]),
		Sources:    cleanQueryValues(values["source"]),
		EventTypes: cleanQueryValues(values["event_type"]),
	}, ""
}

// parseModelDashboardQuery maps URL query parameters to one model dashboard request.
func parseModelDashboardQuery(values url.Values, fallbackWindow time.Duration) (modelDashboardQuery, string) {
	modelID := strings.TrimSpace(values.Get("model_id"))
	if modelID == "" {
		return modelDashboardQuery{}, "model_id is required"
	}
	return modelDashboardQuery{
		ModelID: modelID,
		Window:  parseDashboardWindow(values, fallbackWindow),
	}, ""
}

// parseBoundedPositiveInt parses a positive integer and caps it when requested.
func parseBoundedPositiveInt(raw string, fallback, max int) int {
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return fallback
	}
	if max > 0 && value > max {
		return max
	}
	return value
}

// parseOffset parses pagination offsets and normalizes invalid values to zero.
func parseOffset(raw string) int {
	if raw == "" {
		return 0
	}
	offset, err := strconv.Atoi(raw)
	if err != nil || offset < 0 {
		return 0
	}
	return offset
}

// cleanQueryValues trims, deduplicates, and sorts repeated filter parameters.
func cleanQueryValues(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	cleaned := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		cleaned = append(cleaned, value)
	}
	sort.Strings(cleaned)
	return cleaned
}
