package api

import (
	"context"
	"math"
	"sort"
	"time"

	"llmservicemonitor/internal/store"
)

type dashboardChartConfig struct {
	ID       string
	Title    string
	Type     string
	Metric   string
	GroupBy  string
	Interval time.Duration
	Models   []string
}

var dashboardCharts = []dashboardChartConfig{
	{
		ID:      "ttft-by-model",
		Title:   "Time to first token by model",
		Type:    "line",
		Metric:  "ttft_ms",
		GroupBy: "model",
	},
	{
		ID:      "request-latency-by-model",
		Title:   "Request latency by model",
		Type:    "line",
		Metric:  "request_latency_ms",
		GroupBy: "model",
	},
	{
		ID:      "http-latency",
		Title:   "HTTP check latency",
		Type:    "bar",
		Metric:  "http_latency_ms",
		GroupBy: "check",
	},
}

var chatModelDashboardCharts = []dashboardChartConfig{
	{
		ID:      "model-request-latency",
		Title:   "Request latency",
		Type:    "bar",
		Metric:  "request_latency_ms",
		GroupBy: "capability",
	},
	{
		ID:      "model-ttft",
		Title:   "Time to first token",
		Type:    "bar",
		Metric:  "ttft_ms",
		GroupBy: "capability",
	},
	{
		ID:      "model-itl",
		Title:   "Inter-token latency",
		Type:    "bar",
		Metric:  "itl_ms",
		GroupBy: "capability",
	},
	{
		ID:      "model-tpot",
		Title:   "Time per output token",
		Type:    "bar",
		Metric:  "tpot_ms",
		GroupBy: "capability",
	},
	{
		ID:      "model-output-throughput",
		Title:   "Output throughput",
		Type:    "bar",
		Metric:  "output_tokens_per_second",
		GroupBy: "capability",
	},
	{
		ID:      "model-errors",
		Title:   "Errors",
		Type:    "stacked-bar",
		Metric:  "errors",
		GroupBy: "capability",
	},
}

var embeddingModelDashboardCharts = []dashboardChartConfig{
	{
		ID:      "model-request-latency",
		Title:   "Request latency",
		Type:    "bar",
		Metric:  "request_latency_ms",
		GroupBy: "capability",
	},
	{
		ID:      "model-input-tokens",
		Title:   "Input tokens",
		Type:    "bar",
		Metric:  "input_tokens",
		GroupBy: "capability",
	},
	{
		ID:      "model-vector-dimensions",
		Title:   "Vector dimensions",
		Type:    "bar",
		Metric:  "vector_dimensions",
		GroupBy: "capability",
	},
	{
		ID:      "model-errors",
		Title:   "Errors",
		Type:    "stacked-bar",
		Metric:  "errors",
		GroupBy: "capability",
	},
}

// modelDashboardChartConfigs returns model detail charts that match the model capability.
func modelDashboardChartConfigs(capability string) []dashboardChartConfig {
	if capability == "embedding" {
		return embeddingModelDashboardCharts
	}
	return chatModelDashboardCharts
}

// buildChart converts one static dashboard chart into labels and datasets.
func (r *Router) buildChart(ctx context.Context, cfg dashboardChartConfig, since, now time.Time, window time.Duration) ChartResponse {
	interval := cfg.Interval
	if interval == 0 {
		interval = dashboardChartInterval(window)
	}
	response := ChartResponse{ID: cfg.ID, Title: cfg.Title, Type: cfg.Type, Metric: cfg.Metric}
	samples, err := r.store.MetricSamples(ctx, cfg.Metric, cfg.GroupBy, since)
	if err != nil {
		response.Error = err.Error()
		return response
	}
	response.Labels, response.Datasets = bucketSamples(samples, cfg.Models, since, now, interval, isSummedMetric(cfg.Metric))
	return response
}

// buildModelChart converts one static model chart into labels and datasets.
func (r *Router) buildModelChart(ctx context.Context, cfg dashboardChartConfig, modelID string, since, now time.Time, window time.Duration) ChartResponse {
	interval := cfg.Interval
	if interval == 0 {
		interval = dashboardChartInterval(window)
	}
	response := ChartResponse{ID: cfg.ID, Title: cfg.Title, Type: cfg.Type, Metric: cfg.Metric}
	samples, err := r.store.MetricSamplesForModel(ctx, cfg.Metric, cfg.GroupBy, since, modelID)
	if err != nil {
		response.Error = err.Error()
		return response
	}
	response.Labels, response.Datasets = bucketSamples(samples, nil, since, now, interval, isSummedMetric(cfg.Metric))
	return response
}

// dashboardChartInterval keeps static dashboard charts readable across KPI ranges.
func dashboardChartInterval(window time.Duration) time.Duration {
	switch {
	case window <= 24*time.Hour:
		return 30 * time.Minute
	case window <= 7*24*time.Hour:
		return 6 * time.Hour
	case window <= 30*24*time.Hour:
		return 24 * time.Hour
	default:
		return 7 * 24 * time.Hour
	}
}

// buildModelStatusHistory creates the status-count chart from snapshots and current state.
func (r *Router) buildModelStatusHistory(ctx context.Context, models []store.ModelState, since, now time.Time, window time.Duration) ChartResponse {
	response := ChartResponse{
		ID:     "model-status-history",
		Title:  "Models by status",
		Type:   "stacked-bar",
		Metric: "model_status_count",
	}
	samples, err := r.store.ModelStatusSamples(ctx, since)
	if err != nil {
		response.Error = err.Error()
		return response
	}
	samples = append(samples, modelStatusSamplesFromStates(models, now)...)
	response.Labels, response.Datasets = bucketSamples(samples, nil, since, now, modelStatusInterval(window), false)
	return response
}

// modelStatusSamplesFromStates converts current inventory state into chart samples.
func modelStatusSamplesFromStates(models []store.ModelState, at time.Time) []store.MetricSample {
	var active, inactive, skipped float64
	for _, model := range models {
		if model.Excluded || model.Capability == "skip" {
			skipped++
		}
		if model.Status == store.ModelStatusInactive || model.Status == "missing" {
			inactive++
		}
		if model.Status == store.ModelStatusActive {
			active++
		}
	}
	return []store.MetricSample{
		{At: at, ModelID: "model_inventory", Capability: "inventory", Group: "active", Value: active},
		{At: at, ModelID: "model_inventory", Capability: "inventory", Group: "inactive", Value: inactive},
		{At: at, ModelID: "model_inventory", Capability: "inventory", Group: "skipped", Value: skipped},
	}
}

// modelStatusInterval chooses readable chart buckets for the requested window.
func modelStatusInterval(window time.Duration) time.Duration {
	switch {
	case window <= 24*time.Hour:
		return time.Hour
	case window <= 7*24*time.Hour:
		return 6 * time.Hour
	case window <= 30*24*time.Hour:
		return 24 * time.Hour
	default:
		return 7 * 24 * time.Hour
	}
}

// sloThresholds maps configured SLO thresholds into store query parameters.
func (r *Router) sloThresholds() store.SLOThresholds {
	return store.SLOThresholds{
		TTFTP99MS:           r.cfg.Dashboard.SLO.TTFTP99MS,
		ITLP99MS:            r.cfg.Dashboard.SLO.ITLP99MS,
		RequestLatencyP99MS: r.cfg.Dashboard.SLO.RequestLatencyP99MS,
	}
}

// bucketSamples groups raw metric samples into fixed time buckets for charts.
func bucketSamples(samples []store.MetricSample, allowedModels []string, since, now time.Time, interval time.Duration, summed bool) ([]string, []ChartDataset) {
	allowed := map[string]bool{}
	for _, model := range allowedModels {
		allowed[model] = true
	}
	if interval <= 0 {
		interval = 15 * time.Minute
	}
	bucketCount := int(math.Ceil(now.Sub(since).Seconds()/interval.Seconds())) + 1
	if bucketCount < 1 {
		bucketCount = 1
	}
	labels := make([]string, bucketCount)
	window := now.Sub(since)
	for i := 0; i < bucketCount; i++ {
		labels[i] = formatBucketLabel(since.Add(time.Duration(i)*interval), window)
	}
	type aggregate struct {
		sum   float64
		count int
	}
	groups := map[string][]aggregate{}
	for _, sample := range samples {
		if len(allowed) > 0 && !allowed[sample.ModelID] {
			continue
		}
		index := int(sample.At.Sub(since) / interval)
		if index < 0 || index >= bucketCount {
			continue
		}
		group := sample.Group
		if group == "" {
			group = "all"
		}
		if _, ok := groups[group]; !ok {
			groups[group] = make([]aggregate, bucketCount)
		}
		groups[group][index].sum += sample.Value
		groups[group][index].count++
	}
	names := make([]string, 0, len(groups))
	for group := range groups {
		names = append(names, group)
	}
	sort.Strings(names)
	datasets := make([]ChartDataset, 0, len(names))
	for _, group := range names {
		data := make([]*float64, bucketCount)
		if summed {
			for i := range data {
				data[i] = chartValue(0)
			}
		}
		for i, agg := range groups[group] {
			if agg.count == 0 {
				continue
			}
			if summed {
				data[i] = chartValue(agg.sum)
			} else {
				data[i] = chartValue(agg.sum / float64(agg.count))
			}
		}
		datasets = append(datasets, ChartDataset{Label: group, Data: data})
	}
	return labels, datasets
}

func chartValue(value float64) *float64 {
	return &value
}

// formatBucketLabel keeps timeline labels readable as chart windows grow.
func formatBucketLabel(at time.Time, window time.Duration) string {
	switch {
	case window <= 24*time.Hour:
		return at.Format("15:04")
	case window <= 7*24*time.Hour:
		return at.Format("Mon 15:04")
	case window <= 365*24*time.Hour:
		return at.Format("Jan 02")
	default:
		return at.Format("Jan 2006")
	}
}

// isSummedMetric reports whether chart buckets should sum instead of average.
func isSummedMetric(metric string) bool {
	return metric == "input_tokens" || metric == "output_tokens" || metric == "errors"
}
