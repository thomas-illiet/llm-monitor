package api

import (
	"context"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"time"

	"llmservicemonitor/internal/store"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const metricsCollectTimeout = 5 * time.Second

type prometheusCollector struct {
	store  DashboardStore
	logger *slog.Logger

	httpUp                     *prometheus.Desc
	httpLatencySeconds         *prometheus.Desc
	httpStatusCode             *prometheus.Desc
	httpLastCheckTimestamp     *prometheus.Desc
	authUp                     *prometheus.Desc
	authLatencySeconds         *prometheus.Desc
	authStatusCode             *prometheus.Desc
	authLastCheckTimestamp     *prometheus.Desc
	modelsTotal                *prometheus.Desc
	modelsSkippedTotal         *prometheus.Desc
	modelInfo                  *prometheus.Desc
	modelAvailable             *prometheus.Desc
	modelLastSeenTimestamp     *prometheus.Desc
	modelMissingSinceTimestamp *prometheus.Desc
	probeSuccess               *prometheus.Desc
	probeLatencySeconds        *prometheus.Desc
	probeStatusCode            *prometheus.Desc
	probeLastRunTimestamp      *prometheus.Desc
	probeTTFTSeconds           *prometheus.Desc
	probeITLSeconds            *prometheus.Desc
	probeTPOTSeconds           *prometheus.Desc
	probeInputTokens           *prometheus.Desc
	probeOutputTokens          *prometheus.Desc
	probeTotalTokens           *prometheus.Desc
	probeOutputTokensPerSecond *prometheus.Desc
	probeVectorDimensions      *prometheus.Desc
}

// metricsHandler returns a dedicated Prometheus registry for application and runtime metrics.
func (r *Router) metricsHandler() http.Handler {
	registry := prometheus.NewRegistry()
	registry.MustRegister(collectors.NewGoCollector())
	registry.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	registry.MustRegister(newPrometheusCollector(r.store, r.logger))
	return promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
}

func newPrometheusCollector(db DashboardStore, logger *slog.Logger) *prometheusCollector {
	return &prometheusCollector{
		store:  db,
		logger: logger,

		httpUp:                 prometheus.NewDesc("llm_monitor_http_up", "Whether the latest target HTTP check succeeded.", nil, nil),
		httpLatencySeconds:     prometheus.NewDesc("llm_monitor_http_latency_seconds", "Latency of the latest target HTTP check in seconds.", nil, nil),
		httpStatusCode:         prometheus.NewDesc("llm_monitor_http_status_code", "Status code returned by the latest target HTTP check.", nil, nil),
		httpLastCheckTimestamp: prometheus.NewDesc("llm_monitor_http_last_check_timestamp_seconds", "Unix timestamp of the latest target HTTP check.", nil, nil),
		authUp:                 prometheus.NewDesc("llm_monitor_auth_up", "Whether the latest auth check succeeded.", nil, nil),
		authLatencySeconds:     prometheus.NewDesc("llm_monitor_auth_latency_seconds", "Latency of the latest auth check in seconds.", nil, nil),
		authStatusCode:         prometheus.NewDesc("llm_monitor_auth_status_code", "Status code returned by the latest auth check.", nil, nil),
		authLastCheckTimestamp: prometheus.NewDesc("llm_monitor_auth_last_check_timestamp_seconds", "Unix timestamp of the latest auth check.", nil, nil),

		modelsTotal:                prometheus.NewDesc("llm_monitor_models_total", "Current number of models grouped by inventory status.", []string{"status"}, nil),
		modelsSkippedTotal:         prometheus.NewDesc("llm_monitor_models_skipped_total", "Current number of models excluded from scheduled probes.", nil, nil),
		modelInfo:                  prometheus.NewDesc("llm_monitor_model_info", "Current model inventory metadata.", []string{"model", "capability", "status", "excluded"}, nil),
		modelAvailable:             prometheus.NewDesc("llm_monitor_model_available", "Whether the model is currently available for scheduled probes.", []string{"model", "capability"}, nil),
		modelLastSeenTimestamp:     prometheus.NewDesc("llm_monitor_model_last_seen_timestamp_seconds", "Unix timestamp when the model was last observed.", []string{"model", "capability"}, nil),
		modelMissingSinceTimestamp: prometheus.NewDesc("llm_monitor_model_missing_since_timestamp_seconds", "Unix timestamp when the model became inactive. Metric name is kept for compatibility.", []string{"model", "capability"}, nil),

		probeSuccess:               prometheus.NewDesc("llm_monitor_model_probe_success", "Whether the latest model probe succeeded.", []string{"model", "capability"}, nil),
		probeLatencySeconds:        prometheus.NewDesc("llm_monitor_model_probe_latency_seconds", "Request latency of the latest model probe in seconds.", []string{"model", "capability"}, nil),
		probeStatusCode:            prometheus.NewDesc("llm_monitor_model_probe_status_code", "Status code returned by the latest model probe.", []string{"model", "capability"}, nil),
		probeLastRunTimestamp:      prometheus.NewDesc("llm_monitor_model_probe_last_run_timestamp_seconds", "Unix timestamp of the latest model probe.", []string{"model", "capability"}, nil),
		probeTTFTSeconds:           prometheus.NewDesc("llm_monitor_model_probe_ttft_seconds", "Time to first token for the latest chat probe in seconds.", []string{"model", "capability"}, nil),
		probeITLSeconds:            prometheus.NewDesc("llm_monitor_model_probe_itl_seconds", "Inter-token latency for the latest chat probe in seconds.", []string{"model", "capability"}, nil),
		probeTPOTSeconds:           prometheus.NewDesc("llm_monitor_model_probe_tpot_seconds", "Time per output token for the latest chat probe in seconds.", []string{"model", "capability"}, nil),
		probeInputTokens:           prometheus.NewDesc("llm_monitor_model_probe_input_tokens", "Input tokens reported by the latest model probe.", []string{"model", "capability"}, nil),
		probeOutputTokens:          prometheus.NewDesc("llm_monitor_model_probe_output_tokens", "Output tokens reported by the latest chat probe.", []string{"model", "capability"}, nil),
		probeTotalTokens:           prometheus.NewDesc("llm_monitor_model_probe_total_tokens", "Total tokens reported by the latest model probe.", []string{"model", "capability"}, nil),
		probeOutputTokensPerSecond: prometheus.NewDesc("llm_monitor_model_probe_output_tokens_per_second", "Output token throughput reported by the latest chat probe.", []string{"model", "capability"}, nil),
		probeVectorDimensions:      prometheus.NewDesc("llm_monitor_model_probe_vector_dimensions", "Vector dimensions reported by the latest embedding probe.", []string{"model", "capability"}, nil),
	}
}

// Describe implements prometheus.Collector.
func (c *prometheusCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, desc := range c.descriptors() {
		ch <- desc
	}
}

// Collect implements prometheus.Collector by reading the latest persisted monitor state.
func (c *prometheusCollector) Collect(ch chan<- prometheus.Metric) {
	ctx, cancel := context.WithTimeout(context.Background(), metricsCollectTimeout)
	defer cancel()

	models, err := c.store.ListModelStates(ctx)
	if err != nil {
		c.collectInvalid(ch, err)
		return
	}
	httpCheck, err := c.store.LatestHTTPCheck(ctx)
	if err != nil {
		c.collectInvalid(ch, err)
		return
	}
	authCheck, err := c.store.LatestAuthCheck(ctx)
	if err != nil {
		c.collectInvalid(ch, err)
		return
	}
	latestRuns, err := c.store.LatestRunsByModel(ctx)
	if err != nil {
		c.collectInvalid(ch, err)
		return
	}

	c.collectCheck(ch, httpCheck, c.httpUp, c.httpLatencySeconds, c.httpStatusCode, c.httpLastCheckTimestamp)
	c.collectCheck(ch, authCheck, c.authUp, c.authLatencySeconds, c.authStatusCode, c.authLastCheckTimestamp)
	c.collectModels(ch, models)
	c.collectRuns(ch, latestRuns)
}

func (c *prometheusCollector) descriptors() []*prometheus.Desc {
	return []*prometheus.Desc{
		c.httpUp,
		c.httpLatencySeconds,
		c.httpStatusCode,
		c.httpLastCheckTimestamp,
		c.authUp,
		c.authLatencySeconds,
		c.authStatusCode,
		c.authLastCheckTimestamp,
		c.modelsTotal,
		c.modelsSkippedTotal,
		c.modelInfo,
		c.modelAvailable,
		c.modelLastSeenTimestamp,
		c.modelMissingSinceTimestamp,
		c.probeSuccess,
		c.probeLatencySeconds,
		c.probeStatusCode,
		c.probeLastRunTimestamp,
		c.probeTTFTSeconds,
		c.probeITLSeconds,
		c.probeTPOTSeconds,
		c.probeInputTokens,
		c.probeOutputTokens,
		c.probeTotalTokens,
		c.probeOutputTokensPerSecond,
		c.probeVectorDimensions,
	}
}

func (c *prometheusCollector) collectInvalid(ch chan<- prometheus.Metric, err error) {
	if c.logger != nil {
		c.logger.Error("collect prometheus metrics", "error", err)
	}
	ch <- prometheus.NewInvalidMetric(c.httpUp, err)
}

func (c *prometheusCollector) collectCheck(ch chan<- prometheus.Metric, check *store.CheckRecord, upDesc, latencyDesc, statusDesc, timestampDesc *prometheus.Desc) {
	if check == nil {
		emitGauge(ch, upDesc, 0)
		return
	}
	emitGauge(ch, upDesc, boolGauge(check.OK))
	emitGauge(ch, latencyDesc, millisecondsToSeconds(check.LatencyMS))
	emitGauge(ch, statusDesc, float64(check.StatusCode))
	emitGauge(ch, timestampDesc, timestampSeconds(check.At))
}

func (c *prometheusCollector) collectModels(ch chan<- prometheus.Metric, models []store.ModelState) {
	statusCounts := map[string]float64{}
	var skipped float64
	for _, model := range models {
		status := normalizedLabel(model.Status, "unknown")
		statusCounts[status]++
		if modelSkipped(model) {
			skipped++
		}
	}
	statuses := make([]string, 0, len(statusCounts))
	for status := range statusCounts {
		statuses = append(statuses, status)
	}
	sort.Strings(statuses)
	for _, status := range statuses {
		emitGauge(ch, c.modelsTotal, statusCounts[status], status)
	}
	emitGauge(ch, c.modelsSkippedTotal, skipped)

	for _, model := range models {
		modelID := normalizedLabel(model.ModelID, "unknown")
		capability := normalizedLabel(model.Capability, "unknown")
		status := normalizedLabel(model.Status, "unknown")
		excluded := strconv.FormatBool(model.Excluded)
		emitGauge(ch, c.modelInfo, 1, modelID, capability, status, excluded)
		emitGauge(ch, c.modelAvailable, boolGauge(modelAvailable(model)), modelID, capability)
		if !model.LastSeenAt.IsZero() {
			emitGauge(ch, c.modelLastSeenTimestamp, timestampSeconds(model.LastSeenAt), modelID, capability)
		}
		if model.MissingSince != nil && !model.MissingSince.IsZero() {
			emitGauge(ch, c.modelMissingSinceTimestamp, timestampSeconds(*model.MissingSince), modelID, capability)
		}
	}
}

func (c *prometheusCollector) collectRuns(ch chan<- prometheus.Metric, runs []store.LatestRun) {
	for _, run := range runs {
		modelID := normalizedLabel(run.ModelID, "unknown")
		capability := normalizedLabel(run.Capability, "unknown")
		labels := []string{modelID, capability}

		emitGauge(ch, c.probeSuccess, boolGauge(run.OK), labels...)
		emitGauge(ch, c.probeLatencySeconds, millisecondsToSeconds(run.LatencyMS), labels...)
		emitGauge(ch, c.probeStatusCode, float64(run.StatusCode), labels...)
		if !run.StartedAt.IsZero() {
			emitGauge(ch, c.probeLastRunTimestamp, timestampSeconds(run.StartedAt), labels...)
		}
		emitOptionalMilliseconds(ch, c.probeTTFTSeconds, run.TTFTMS, labels...)
		emitOptionalMilliseconds(ch, c.probeITLSeconds, run.ITLMS, labels...)
		emitOptionalMilliseconds(ch, c.probeTPOTSeconds, run.TPOTMS, labels...)
		emitOptionalInt(ch, c.probeInputTokens, run.InputTokens, labels...)
		emitOptionalInt(ch, c.probeOutputTokens, run.OutputTokens, labels...)
		emitOptionalInt(ch, c.probeTotalTokens, run.TotalTokens, labels...)
		emitOptionalFloat(ch, c.probeOutputTokensPerSecond, run.OutputTokensPerSecond, labels...)
		emitOptionalInt(ch, c.probeVectorDimensions, run.VectorDimensions, labels...)
	}
}

func modelSkipped(model store.ModelState) bool {
	return model.Excluded || model.Capability == "skip"
}

func modelAvailable(model store.ModelState) bool {
	return model.Status == store.ModelStatusActive && !modelSkipped(model)
}

func boolGauge(value bool) float64 {
	if value {
		return 1
	}
	return 0
}

func timestampSeconds(value time.Time) float64 {
	return float64(value.UnixNano()) / float64(time.Second)
}

func millisecondsToSeconds(value float64) float64 {
	return value / 1000
}

func normalizedLabel(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func emitGauge(ch chan<- prometheus.Metric, desc *prometheus.Desc, value float64, labels ...string) {
	ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, value, labels...)
}

func emitOptionalMilliseconds(ch chan<- prometheus.Metric, desc *prometheus.Desc, value *float64, labels ...string) {
	if value == nil {
		return
	}
	emitGauge(ch, desc, millisecondsToSeconds(*value), labels...)
}

func emitOptionalFloat(ch chan<- prometheus.Metric, desc *prometheus.Desc, value *float64, labels ...string) {
	if value == nil {
		return
	}
	emitGauge(ch, desc, *value, labels...)
}

func emitOptionalInt(ch chan<- prometheus.Metric, desc *prometheus.Desc, value *int, labels ...string) {
	if value == nil {
		return
	}
	emitGauge(ch, desc, float64(*value), labels...)
}
