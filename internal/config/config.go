package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Duration wraps time.Duration with YAML string parsing.
type Duration struct {
	time.Duration
	Set bool `yaml:"-"`
}

// UnmarshalYAML parses human-readable duration values such as "30s", "24h", or "90d".
func (d *Duration) UnmarshalYAML(value *yaml.Node) error {
	d.Set = true
	if value.Kind == 0 || value.Value == "" {
		return nil
	}
	parsed, err := parseDuration(value.Value)
	if err != nil {
		return fmt.Errorf("parse duration %q: %w", value.Value, err)
	}
	d.Duration = parsed
	return nil
}

// parseDuration extends Go duration strings with a day unit for config files.
func parseDuration(raw string) (time.Duration, error) {
	raw = strings.TrimSpace(raw)
	parsed, err := time.ParseDuration(raw)
	if err == nil || !strings.Contains(raw, "d") {
		return parsed, err
	}

	sign := ""
	if strings.HasPrefix(raw, "+") || strings.HasPrefix(raw, "-") {
		sign = raw[:1]
		raw = raw[1:]
	}

	var converted strings.Builder
	converted.WriteString(sign)
	for i := 0; i < len(raw); {
		start := i
		for i < len(raw) && ((raw[i] >= '0' && raw[i] <= '9') || raw[i] == '.') {
			i++
		}
		if start == i {
			return 0, err
		}
		number := raw[start:i]
		unitStart := i
		for i < len(raw) && !((raw[i] >= '0' && raw[i] <= '9') || raw[i] == '.') {
			i++
		}
		unit := raw[unitStart:i]
		if unit == "d" {
			days, parseErr := strconv.ParseFloat(number, 64)
			if parseErr != nil {
				return 0, parseErr
			}
			converted.WriteString(strconv.FormatFloat(days*24, 'f', -1, 64))
			converted.WriteString("h")
			continue
		}
		converted.WriteString(number)
		converted.WriteString(unit)
	}
	return time.ParseDuration(converted.String())
}

// MarshalYAML writes durations back using Go's standard duration string format.
func (d Duration) MarshalYAML() (any, error) {
	return d.Duration.String(), nil
}

// Config is the complete runtime configuration loaded from YAML.
type Config struct {
	Server    ServerConfig    `yaml:"server"`
	Logging   LoggingConfig   `yaml:"logging"`
	Postgres  PostgresConfig  `yaml:"postgres"`
	Redis     RedisConfig     `yaml:"redis"`
	Asynq     AsynqConfig     `yaml:"asynq"`
	TLS       TLSConfig       `yaml:"tls"`
	Target    TargetConfig    `yaml:"target"`
	Auth      AuthConfig      `yaml:"auth"`
	SMTP      SMTPConfig      `yaml:"smtp"`
	MCP       MCPConfig       `yaml:"mcp"`
	Schedules ScheduleConfig  `yaml:"schedules"`
	Models    ModelsConfig    `yaml:"models"`
	Tests     TestsConfig     `yaml:"tests"`
	Dashboard DashboardConfig `yaml:"dashboard"`
	Retention RetentionConfig `yaml:"retention"`
}

// ServerConfig controls the HTTP listener.
type ServerConfig struct {
	Address string `yaml:"address"`
}

// LoggingConfig controls application log verbosity.
type LoggingConfig struct {
	Level string `yaml:"level"`
}

// PostgresConfig controls persistence connectivity.
type PostgresConfig struct {
	DSN string `yaml:"dsn"`
}

// RedisConfig controls the Redis instance used by Asynq queues.
type RedisConfig struct {
	Addr     string `yaml:"addr"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
}

// AsynqConfig controls queue, worker, scheduler, and manual task retention settings.
type AsynqConfig struct {
	Queue                 string   `yaml:"queue"`
	WorkerConcurrency     int      `yaml:"worker_concurrency"`
	SchedulerSyncInterval Duration `yaml:"scheduler_sync_interval"`
	ManualTaskRetention   Duration `yaml:"manual_task_retention"`
}

// TLSConfig controls shared outbound HTTP TLS behavior.
type TLSConfig struct {
	InsecureSkipVerify bool `yaml:"insecure_skip_verify"`
}

// TargetConfig controls outbound calls to the OpenAI-compatible LLM API.
type TargetConfig struct {
	Name               string                `yaml:"name"`
	BaseURL            string                `yaml:"base_url"`
	HTTPCheckPath      string                `yaml:"http_check_path"`
	Endpoints          TargetEndpointsConfig `yaml:"endpoints"`
	Timeout            Duration              `yaml:"timeout"`
	CAFile             string                `yaml:"ca_file"`
	APIKey             string                `yaml:"api_key"`
	InsecureSkipVerify bool                  `yaml:"insecure_skip_verify"`
	Retry              RetryConfig           `yaml:"retry"`
}

// TargetEndpointsConfig controls the OpenAI-like endpoint URLs used by probes.
type TargetEndpointsConfig struct {
	Models     string `yaml:"models"`
	Chat       string `yaml:"chat"`
	Embeddings string `yaml:"embeddings"`
}

// RetryConfig controls retry behavior for outbound LLM API HTTP calls.
type RetryConfig struct {
	Enabled    *bool    `yaml:"enabled"`
	MaxRetries *int     `yaml:"max_retries"`
	WaitMin    Duration `yaml:"wait_min"`
	WaitMax    Duration `yaml:"wait_max"`
}

// AuthConfig controls optional OAuth2 client credentials authentication.
type AuthConfig struct {
	Enabled          bool       `yaml:"enabled"`
	TokenURL         string     `yaml:"token_url"`
	ClientID         string     `yaml:"client_id"`
	ClientSecret     string     `yaml:"client_secret"`
	ClientAuthMethod string     `yaml:"client_auth_method"`
	Scopes           []string   `yaml:"scopes"`
	Audience         string     `yaml:"audience"`
	Timeout          Duration   `yaml:"timeout"`
	RefreshSkew      Duration   `yaml:"refresh_skew"`
	MTLS             MTLSConfig `yaml:"mtls"`
}

// MTLSConfig controls optional mutual TLS for OAuth token requests.
type MTLSConfig struct {
	CertFile           string `yaml:"cert_file"`
	KeyFile            string `yaml:"key_file"`
	CAFile             string `yaml:"ca_file"`
	InsecureSkipVerify bool   `yaml:"insecure_skip_verify"`
}

// SMTPConfig controls email delivery for model lifecycle alerts.
type SMTPConfig struct {
	Enabled            bool     `yaml:"enabled"`
	Host               string   `yaml:"host"`
	Port               int      `yaml:"port"`
	Username           string   `yaml:"username"`
	Password           string   `yaml:"password"`
	From               string   `yaml:"from"`
	To                 []string `yaml:"to"`
	StartTLS           bool     `yaml:"starttls"`
	InsecureSkipVerify bool     `yaml:"insecure_skip_verify"`
}

// MCPConfig controls the optional Streamable HTTP MCP endpoint.
type MCPConfig struct {
	Enabled     bool   `yaml:"enabled"`
	Path        string `yaml:"path"`
	BearerToken string `yaml:"bearer_token"`
}

// ScheduleConfig controls recurring monitor intervals.
type ScheduleConfig struct {
	HTTPCheck         Duration                   `yaml:"http_check"`
	AuthCheck         Duration                   `yaml:"auth_check"`
	ModelSnapshot     Duration                   `yaml:"model_snapshot"`
	ModelRuns         Duration                   `yaml:"model_runs"`
	ModelRunOverrides []ModelRunScheduleOverride `yaml:"model_run_overrides"`
}

// ModelRunScheduleOverride controls a model-specific probe interval.
type ModelRunScheduleOverride struct {
	ModelID  string   `yaml:"model_id"`
	Pattern  string   `yaml:"pattern"`
	Interval Duration `yaml:"interval"`
}

// ModelsConfig controls inventory alerting and probe concurrency.
type ModelsConfig struct {
	AbsenceAlertAfter Duration `yaml:"absence_alert_after"`
	MaxConcurrency    int      `yaml:"max_concurrency"`
}

// TestsConfig controls scheduled chat and embedding probes.
type TestsConfig struct {
	ChatPrompts      []ChatPromptConfig     `yaml:"chat_prompts"`
	EmbeddingFixture EmbeddingFixtureConfig `yaml:"embedding_fixture"`
}

// ChatPromptConfig describes one chat prompt used by scheduled probes.
type ChatPromptConfig struct {
	ID          string  `yaml:"id"`
	Prompt      string  `yaml:"prompt"`
	MaxTokens   int     `yaml:"max_tokens"`
	Temperature float64 `yaml:"temperature"`
}

// EmbeddingFixtureConfig describes the text fixture used by embedding probes.
type EmbeddingFixtureConfig struct {
	Path     string `yaml:"path"`
	MaxBytes int    `yaml:"max_bytes"`
}

// DashboardConfig controls KPI windows and SLOs.
type DashboardConfig struct {
	DefaultWindow Duration  `yaml:"default_window"`
	SiteName      string    `yaml:"site_name"`
	SiteURL       string    `yaml:"site_url"`
	SLO           SLOConfig `yaml:"slo"`
}

// SLOConfig defines thresholds used to classify degraded model behavior.
type SLOConfig struct {
	TTFTP99MS           float64 `yaml:"ttft_p99_ms"`
	ITLP99MS            float64 `yaml:"itl_p99_ms"`
	RequestLatencyP99MS float64 `yaml:"request_latency_p99_ms"`
}

// RetentionConfig controls optional pruning of persisted history.
type RetentionConfig struct {
	History Duration `yaml:"history"`
}

// Load reads a YAML config file, applies defaults, and validates the result.
func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}
	cfg.ApplyDefaults()
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// ApplyDefaults fills optional settings with conservative production defaults.
func (c *Config) ApplyDefaults() {
	if c.Server.Address == "" {
		c.Server.Address = ":8080"
	}
	c.Logging.Level = strings.ToLower(strings.TrimSpace(c.Logging.Level))
	if c.Logging.Level == "" {
		c.Logging.Level = "info"
	}
	c.Redis.Addr = strings.TrimSpace(c.Redis.Addr)
	if c.Redis.Addr == "" {
		c.Redis.Addr = "localhost:6379"
	}
	c.Redis.Username = strings.TrimSpace(c.Redis.Username)
	c.Asynq.Queue = strings.TrimSpace(c.Asynq.Queue)
	if c.Asynq.Queue == "" {
		c.Asynq.Queue = "default"
	}
	if c.Asynq.WorkerConcurrency <= 0 {
		c.Asynq.WorkerConcurrency = 10
	}
	if c.Asynq.SchedulerSyncInterval.Duration == 0 {
		c.Asynq.SchedulerSyncInterval.Duration = 30 * time.Second
	}
	if c.Asynq.ManualTaskRetention.Duration == 0 {
		c.Asynq.ManualTaskRetention.Duration = 10 * time.Minute
	}
	if c.Target.Name == "" {
		c.Target.Name = "default"
	}
	c.Target.BaseURL = strings.TrimSpace(c.Target.BaseURL)
	c.Target.Endpoints.Models = strings.TrimSpace(c.Target.Endpoints.Models)
	if c.Target.Endpoints.Models == "" {
		c.Target.Endpoints.Models = "/v1/models"
	}
	c.Target.Endpoints.Chat = strings.TrimSpace(c.Target.Endpoints.Chat)
	if c.Target.Endpoints.Chat == "" {
		c.Target.Endpoints.Chat = "/v1/chat/completions"
	}
	c.Target.Endpoints.Embeddings = strings.TrimSpace(c.Target.Endpoints.Embeddings)
	if c.Target.Endpoints.Embeddings == "" {
		c.Target.Endpoints.Embeddings = "/v1/embeddings"
	}
	c.Target.HTTPCheckPath = strings.TrimSpace(c.Target.HTTPCheckPath)
	if c.Target.HTTPCheckPath == "" {
		c.Target.HTTPCheckPath = c.Target.Endpoints.Models
	}
	if c.Target.Timeout.Duration == 0 {
		c.Target.Timeout.Duration = 30 * time.Second
	}
	if !c.Target.Retry.WaitMin.Set && c.Target.Retry.WaitMin.Duration == 0 {
		c.Target.Retry.WaitMin.Duration = 500 * time.Millisecond
	}
	if !c.Target.Retry.WaitMax.Set && c.Target.Retry.WaitMax.Duration == 0 {
		c.Target.Retry.WaitMax.Duration = 5 * time.Second
	}
	if c.TLS.InsecureSkipVerify {
		c.Target.InsecureSkipVerify = true
		c.Auth.MTLS.InsecureSkipVerify = true
	}
	if c.Auth.Timeout.Duration == 0 {
		c.Auth.Timeout.Duration = 10 * time.Second
	}
	if c.Auth.ClientAuthMethod == "" {
		c.Auth.ClientAuthMethod = "client_secret_basic"
	}
	if c.Auth.RefreshSkew.Duration == 0 {
		c.Auth.RefreshSkew.Duration = 30 * time.Second
	}
	if c.SMTP.Port == 0 {
		c.SMTP.Port = 587
	}
	if c.MCP.Path == "" {
		c.MCP.Path = "/mcp"
	}
	if c.Schedules.HTTPCheck.Duration == 0 {
		c.Schedules.HTTPCheck.Duration = 30 * time.Second
	}
	if c.Schedules.AuthCheck.Duration == 0 {
		c.Schedules.AuthCheck.Duration = time.Minute
	}
	if c.Schedules.ModelSnapshot.Duration == 0 {
		c.Schedules.ModelSnapshot.Duration = 5 * time.Minute
	}
	if c.Schedules.ModelRuns.Duration == 0 {
		c.Schedules.ModelRuns.Duration = 15 * time.Minute
	}
	if c.Models.AbsenceAlertAfter.Duration == 0 {
		c.Models.AbsenceAlertAfter.Duration = 24 * time.Hour
	}
	if c.Models.MaxConcurrency <= 0 {
		c.Models.MaxConcurrency = 4
	}
	c.Dashboard.SiteName = strings.TrimSpace(c.Dashboard.SiteName)
	if c.Dashboard.SiteName == "" {
		c.Dashboard.SiteName = "LLM Service Monitor"
	}
	c.Dashboard.SiteURL = strings.TrimSpace(c.Dashboard.SiteURL)
	if c.Dashboard.DefaultWindow.Duration == 0 {
		c.Dashboard.DefaultWindow.Duration = 24 * time.Hour
	}
	if c.Dashboard.SLO.TTFTP99MS == 0 {
		c.Dashboard.SLO.TTFTP99MS = 200
	}
	if c.Dashboard.SLO.ITLP99MS == 0 {
		c.Dashboard.SLO.ITLP99MS = 50
	}
	if c.Dashboard.SLO.RequestLatencyP99MS == 0 {
		c.Dashboard.SLO.RequestLatencyP99MS = 3000
	}
	for i := range c.Tests.ChatPrompts {
		if c.Tests.ChatPrompts[i].MaxTokens == 0 {
			c.Tests.ChatPrompts[i].MaxTokens = 128
		}
	}
	if c.Tests.EmbeddingFixture.MaxBytes == 0 {
		c.Tests.EmbeddingFixture.MaxBytes = 4096
	}
	if !c.Retention.History.Set && c.Retention.History.Duration == 0 {
		c.Retention.History.Duration = 90 * 24 * time.Hour
	}
}

// Validate checks that required config fields and enum-like values are usable.
func (c Config) Validate() error {
	var problems []string
	if c.Postgres.DSN == "" {
		problems = append(problems, "postgres.dsn is required")
	}
	if strings.TrimSpace(c.Redis.Addr) == "" {
		problems = append(problems, "redis.addr is required")
	}
	if c.Redis.DB < 0 {
		problems = append(problems, "redis.db must be greater than or equal to 0")
	}
	if strings.TrimSpace(c.Asynq.Queue) == "" {
		problems = append(problems, "asynq.queue is required")
	}
	if c.Asynq.WorkerConcurrency <= 0 {
		problems = append(problems, "asynq.worker_concurrency must be greater than 0")
	}
	if c.Asynq.SchedulerSyncInterval.Duration <= 0 {
		problems = append(problems, "asynq.scheduler_sync_interval must be greater than 0")
	}
	if c.Asynq.ManualTaskRetention.Duration <= 0 {
		problems = append(problems, "asynq.manual_task_retention must be greater than 0")
	}
	if c.Schedules.HTTPCheck.Duration <= 0 {
		problems = append(problems, "schedules.http_check must be greater than 0")
	}
	if c.Schedules.AuthCheck.Duration <= 0 {
		problems = append(problems, "schedules.auth_check must be greater than 0")
	}
	if c.Schedules.ModelSnapshot.Duration <= 0 {
		problems = append(problems, "schedules.model_snapshot must be greater than 0")
	}
	if c.Schedules.ModelRuns.Duration <= 0 {
		problems = append(problems, "schedules.model_runs must be greater than 0")
	}
	if !isLogLevel(c.Logging.Level) {
		problems = append(problems, "logging.level must be debug, info, warn, or error")
	}
	if c.Target.BaseURL == "" {
		problems = append(problems, "target.base_url is required")
	} else if !isAbsoluteHTTPURL(c.Target.BaseURL) {
		problems = append(problems, "target.base_url must be an absolute http or https URL")
	}
	if !isHTTPPathOrURL(c.Target.HTTPCheckPath) {
		problems = append(problems, "target.http_check_path must start with / or be an absolute http or https URL")
	}
	if !isHTTPPathOrURL(c.Target.Endpoints.Models) {
		problems = append(problems, "target.endpoints.models must start with / or be an absolute http or https URL")
	}
	if !isHTTPPathOrURL(c.Target.Endpoints.Chat) {
		problems = append(problems, "target.endpoints.chat must start with / or be an absolute http or https URL")
	}
	if !isHTTPPathOrURL(c.Target.Endpoints.Embeddings) {
		problems = append(problems, "target.endpoints.embeddings must start with / or be an absolute http or https URL")
	}
	if c.Target.Retry.MaxRetries != nil && *c.Target.Retry.MaxRetries < 0 {
		problems = append(problems, "target.retry.max_retries must be greater than or equal to 0")
	}
	if c.Target.Retry.EnabledValue() {
		if c.Target.Retry.WaitMin.Duration <= 0 {
			problems = append(problems, "target.retry.wait_min must be greater than 0 when retry is enabled")
		}
		if c.Target.Retry.WaitMax.Duration <= 0 {
			problems = append(problems, "target.retry.wait_max must be greater than 0 when retry is enabled")
		}
		if c.Target.Retry.WaitMax.Duration < c.Target.Retry.WaitMin.Duration {
			problems = append(problems, "target.retry.wait_max must be greater than or equal to target.retry.wait_min")
		}
	}
	if c.Auth.Enabled && c.Auth.TokenURL == "" {
		problems = append(problems, "auth.token_url is required when auth.enabled=true")
	}
	if c.Auth.ClientAuthMethod != "client_secret_basic" && c.Auth.ClientAuthMethod != "client_secret_post" {
		problems = append(problems, "auth.client_auth_method must be client_secret_basic or client_secret_post")
	}
	if c.SMTP.Enabled {
		if c.SMTP.Host == "" {
			problems = append(problems, "smtp.host is required when smtp.enabled=true")
		}
		if c.SMTP.From == "" {
			problems = append(problems, "smtp.from is required when smtp.enabled=true")
		}
		if len(c.SMTP.To) == 0 {
			problems = append(problems, "smtp.to is required when smtp.enabled=true")
		}
	}
	if c.MCP.Enabled {
		if c.MCP.Path == "" || !strings.HasPrefix(c.MCP.Path, "/") || c.MCP.Path == "/" {
			problems = append(problems, "mcp.path must start with / and cannot be /")
		}
		if c.MCP.BearerToken == "" {
			problems = append(problems, "mcp.bearer_token is required when mcp.enabled=true")
		}
	}
	if c.Retention.History.Duration < 0 {
		problems = append(problems, "retention.history must be greater than or equal to 0")
	}
	for i, override := range c.Schedules.ModelRunOverrides {
		modelID := strings.TrimSpace(override.ModelID)
		pattern := strings.TrimSpace(override.Pattern)
		switch {
		case modelID == "" && pattern == "":
			problems = append(problems, fmt.Sprintf("schedules.model_run_overrides[%d] requires model_id or pattern", i))
		case modelID != "" && pattern != "":
			problems = append(problems, fmt.Sprintf("schedules.model_run_overrides[%d] must set only one of model_id or pattern", i))
		}
		if override.Interval.Duration <= 0 {
			problems = append(problems, fmt.Sprintf("schedules.model_run_overrides[%d].interval must be greater than 0", i))
		}
		if pattern != "" {
			if _, err := regexp.Compile(wildcardPatternRegexp(pattern)); err != nil {
				problems = append(problems, fmt.Sprintf("schedules.model_run_overrides[%d].pattern is invalid", i))
			}
		}
	}
	if c.Dashboard.SiteURL != "" {
		parsed, err := url.Parse(c.Dashboard.SiteURL)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
			problems = append(problems, "dashboard.site_url must be an absolute http or https URL")
		}
	}
	if len(problems) > 0 {
		return errors.New(strings.Join(problems, "; "))
	}
	return nil
}

// EnabledValue reports whether retry behavior should be used.
func (r RetryConfig) EnabledValue() bool {
	if r.Enabled != nil && !*r.Enabled {
		return false
	}
	return r.MaxRetriesValue() > 0
}

// MaxRetriesValue returns the configured retry count, defaulting to a light profile.
func (r RetryConfig) MaxRetriesValue() int {
	if r.MaxRetries == nil {
		return 2
	}
	return *r.MaxRetries
}

// WaitMinValue returns the lower retry backoff bound.
func (r RetryConfig) WaitMinValue() time.Duration {
	if r.WaitMin.Duration == 0 {
		return 500 * time.Millisecond
	}
	return r.WaitMin.Duration
}

// WaitMaxValue returns the upper retry backoff bound.
func (r RetryConfig) WaitMaxValue() time.Duration {
	if r.WaitMax.Duration == 0 {
		return 5 * time.Second
	}
	return r.WaitMax.Duration
}

// isLogLevel reports whether a configured level is accepted by the app logger.
func isLogLevel(level string) bool {
	switch level {
	case "debug", "info", "warn", "error":
		return true
	default:
		return false
	}
}

// isHTTPPathOrURL reports whether an endpoint is a rooted path or absolute web URL.
func isHTTPPathOrURL(raw string) bool {
	return strings.HasPrefix(raw, "/") || isAbsoluteHTTPURL(raw)
}

// isAbsoluteHTTPURL reports whether a config URL can be used for outbound web requests.
func isAbsoluteHTTPURL(raw string) bool {
	parsed, err := url.Parse(raw)
	return err == nil && parsed.Scheme != "" && parsed.Host != "" && (parsed.Scheme == "http" || parsed.Scheme == "https")
}

func wildcardPatternRegexp(pattern string) string {
	var builder strings.Builder
	builder.WriteString("^")
	for _, r := range pattern {
		switch r {
		case '*':
			builder.WriteString(".*")
		case '?':
			builder.WriteString(".")
		default:
			builder.WriteString(regexp.QuoteMeta(string(r)))
		}
	}
	builder.WriteString("$")
	return builder.String()
}
