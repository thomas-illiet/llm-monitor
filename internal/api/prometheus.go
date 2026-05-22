package api

import (
	"context"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"time"

	"llmservicemonitor/internal/config"
	"llmservicemonitor/internal/store"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const metricsCollectTimeout = 5 * time.Second

type prometheusCollector struct {
	store     DashboardStore
	logger    *slog.Logger
	providers []config.ProviderConfig

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
	registry.MustRegister(newPrometheusCollector(r.store, r.logger, r.cfg.Providers))
	return promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
}

func newPrometheusCollector(db DashboardStore, logger *slog.Logger, providers []config.ProviderConfig) *prometheusCollector {
	return &prometheusCollector{
		store:     db,
		logger:    logger,
		providers: providers,

		httpUp:                 prometheus.NewDesc("llm_monitor_http_up", "Whether the latest provider HTTP check succeeded.", []string{"provider"}, nil),
		httpLatencySeconds:     prometheus.NewDesc("llm_monitor_http_latency_seconds", "Latency of the latest provider HTTP check in seconds.", []string{"provider"}, nil),
		httpStatusCode:         prometheus.NewDesc("llm_monitor_http_status_code", "Status code returned by the latest provider HTTP check.", []string{"provider"}, nil),
		httpLastCheckTimestamp: prometheus.NewDesc("llm_monitor_http_last_check_timestamp_seconds", "Unix timestamp of the latest provider HTTP check.", []string{"provider"}, nil),
		authUp:                 prometheus.NewDesc("llm_monitor_auth_up", "Whether the latest provider auth check succeeded.", []string{"provider"}, nil),
		authLatencySeconds:     prometheus.NewDesc("llm_monitor_auth_latency_seconds", "Latency of the latest provider auth check in seconds.", []string{"provider"}, nil),
		authStatusCode:         prometheus.NewDesc("llm_monitor_auth_status_code", "Status code returned by the latest provider auth check.", []string{"provider"}, nil),
		authLastCheckTimestamp: prometheus.NewDesc("llm_monitor_auth_last_check_timestamp_seconds", "Unix timestamp of the latest provider auth check.", []string{"provider"}, nil),

		modelsTotal:                prometheus.NewDesc("llm_monitor_models_total", "Current number of models grouped by provider and inventory status.", []string{"provider", "status"}, nil),
		modelsSkippedTotal:         prometheus.NewDesc("llm_monitor_models_skipped_total", "Current number of models excluded from scheduled probes by provider.", []string{"provider"}, nil),
		modelInfo:                  prometheus.NewDesc("llm_monitor_model_info", "Current model inventory metadata.", []string{"provider", "model", "capability", "status", "excluded"}, nil),
		modelAvailable:             prometheus.NewDesc("llm_monitor_model_available", "Whether the model is currently available for scheduled probes.", []string{"provider", "model", "capability"}, nil),
		modelLastSeenTimestamp:     prometheus.NewDesc("llm_monitor_model_last_seen_timestamp_seconds", "Unix timestamp when the model was last observed.", []string{"provider", "model", "capability"}, nil),
		modelMissingSinceTimestamp: prometheus.NewDesc("llm_monitor_model_missing_since_timestamp_seconds", "Unix timestamp when the model became inactive. Metric name is kept for compatibility.", []string{"provider", "model", "capability"}, nil),

		probeSuccess:               prometheus.NewDesc("llm_monitor_model_probe_success", "Whether the latest model probe succeeded.", []string{"provider", "model", "capability"}, nil),
		probeLatencySeconds:        prometheus.NewDesc("llm_monitor_model_probe_latency_seconds", "Request latency of the latest model probe in seconds.", []string{"provider", "model", "capability"}, nil),
		probeStatusCode:            prometheus.NewDesc("llm_monitor_model_probe_status_code", "Status code returned by the latest model probe.", []string{"provider", "model", "capability"}, nil),
		probeLastRunTimestamp:      prometheus.NewDesc("llm_monitor_model_probe_last_run_timestamp_seconds", "Unix timestamp of the latest model probe.", []string{"provider", "model", "capability"}, nil),
		probeTTFTSeconds:           prometheus.NewDesc("llm_monitor_model_probe_ttft_seconds", "Time to first token for the latest chat probe in seconds.", []string{"provider", "model", "capability"}, nil),
		probeITLSeconds:            prometheus.NewDesc("llm_monitor_model_probe_itl_seconds", "Inter-token latency for the latest chat probe.", []string{"provider", "model", "capability"}, nil),
		probeTPOTSeconds:           prometheus.NewDesc("llm_monitor_model_probe_tpot_seconds", "Time per output token for the latest chat probe in seconds.", []string{"provider", "model", "capability"}, nil),
		probeInputTokens:           prometheus.NewDesc("llm_monitor_model_probe_input_tokens", "Input tokens reported by the latest model probe.", []string{"provider", "model", "capability"}, nil),
		probeOutputTokens:          prometheus.NewDesc("llm_monitor_model_probe_output_tokens", "Output tokens reported by the latest chat probe.", []string{"provider", "model", "capability"}, nil),
		probeTotalTokens:           prometheus.NewDesc("llm_monitor_model_probe_total_tokens", "Total tokens reported by the latest model probe.", []string{"provider", "model", "capability"}, nil),
		probeOutputTokensPerSecond: prometheus.NewDesc("llm_monitor_model_probe_output_tokens_per_second", "Output token throughput reported by the latest chat probe.", []string{"provider", "model", "capability"}, nil),
		probeVectorDimensions:      prometheus.NewDesc("llm_monitor_model_probe_vector_dimensions", "Vector dimensions reported by the latest embedding probe.", []string{"provider", "model", "capability"}, nil),
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
	latestRuns, err := c.store.LatestRunsByModel(ctx)
	if err != nil {
		c.collectInvalid(ch, err)
		return
	}

	for _, provider := range c.providers {
		httpCheck, err := c.store.LatestHTTPCheck(ctx, provider.ID)
		if err != nil {
			c.collectInvalid(ch, err)
			return
		}
		authCheck, err := c.store.LatestAuthCheck(ctx, provider.ID)
		if err != nil {
			c.collectInvalid(ch, err)
			return
		}
		c.collectCheck(ch, provider.ID, httpCheck, c.httpUp, c.httpLatencySeconds, c.httpStatusCode, c.httpLastCheckTimestamp)
		c.collectCheck(ch, provider.ID, authCheck, c.authUp, c.authLatencySeconds, c.authStatusCode, c.authLastCheckTimestamp)
	}
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

func (c *prometheusCollector) collectCheck(ch chan<- prometheus.Metric, providerID string, check *store.CheckRecord, upDesc, latencyDesc, statusDesc, timestampDesc *prometheus.Desc) {
	if check == nil {
		emitGauge(ch, upDesc, 0, providerID)
		return
	}
	emitGauge(ch, upDesc, boolGauge(check.OK), providerID)
	emitGauge(ch, latencyDesc, millisecondsToSeconds(check.LatencyMS), providerID)
	emitGauge(ch, statusDesc, float64(check.StatusCode), providerID)
	emitGauge(ch, timestampDesc, timestampSeconds(check.At), providerID)
}

func (c *prometheusCollector) collectModels(ch chan<- prometheus.Metric, models []store.ModelState) {
	statusCounts := map[string]map[string]float64{}
	skippedCounts := map[string]float64{}
	for _, model := range models {
		providerID := normalizedLabel(model.ProviderID, "unknown")
		status := normalizedLabel(model.Status, "unknown")
		if statusCounts[providerID] == nil {
			statusCounts[providerID] = map[string]float64{}
		}
		statusCounts[providerID][status]++
		if modelSkipped(model) {
			skippedCounts[providerID]++
		}
	}
	providers := make([]string, 0, len(statusCounts))
	for providerID := range statusCounts {
		providers = append(providers, providerID)
	}
	sort.Strings(providers)
	for _, providerID := range providers {
		statuses := make([]string, 0, len(statusCounts[providerID]))
		for status := range statusCounts[providerID] {
			statuses = append(statuses, status)
		}
		sort.Strings(statuses)
		for _, status := range statuses {
			emitGauge(ch, c.modelsTotal, statusCounts[providerID][status], providerID, status)
		}
		emitGauge(ch, c.modelsSkippedTotal, skippedCounts[providerID], providerID)
	}

	for _, model := range models {
		providerID := normalizedLabel(model.ProviderID, "unknown")
		modelID := normalizedLabel(model.ModelID, "unknown")
		capability := normalizedLabel(model.Capability, "unknown")
		status := normalizedLabel(model.Status, "unknown")
		excluded := strconv.FormatBool(model.Excluded)
		emitGauge(ch, c.modelInfo, 1, providerID, modelID, capability, status, excluded)
		emitGauge(ch, c.modelAvailable, boolGauge(modelAvailable(model)), providerID, modelID, capability)
		if !model.LastSeenAt.IsZero() {
			emitGauge(ch, c.modelLastSeenTimestamp, timestampSeconds(model.LastSeenAt), providerID, modelID, capability)
		}
		if model.MissingSince != nil && !model.MissingSince.IsZero() {
			emitGauge(ch, c.modelMissingSinceTimestamp, timestampSeconds(*model.MissingSince), providerID, modelID, capability)
		}
	}
}

func (c *prometheusCollector) collectRuns(ch chan<- prometheus.Metric, runs []store.LatestRun) {
	for _, run := range runs {
		providerID := normalizedLabel(run.ProviderID, "unknown")
		modelID := normalizedLabel(run.ModelID, "unknown")
		capability := normalizedLabel(run.Capability, "unknown")
		labels := []string{providerID, modelID, capability}

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
