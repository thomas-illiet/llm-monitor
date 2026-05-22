package config

import "time"

// Duration wraps time.Duration with YAML string parsing.
type Duration struct {
	time.Duration
	Set bool `yaml:"-"`
}

// Config is the complete runtime configuration loaded from YAML.
type Config struct {
	Server    ServerConfig     `yaml:"server"`
	Logging   LoggingConfig    `yaml:"logging"`
	Postgres  PostgresConfig   `yaml:"postgres"`
	Redis     RedisConfig      `yaml:"redis"`
	Asynq     AsynqConfig      `yaml:"asynq"`
	TLS       TLSConfig        `yaml:"tls"`
	Providers []ProviderConfig `yaml:"providers"`
	SMTP      SMTPConfig       `yaml:"smtp"`
	MCP       MCPConfig        `yaml:"mcp"`
	Schedules ScheduleConfig   `yaml:"schedules"`
	Models    ModelsConfig     `yaml:"models"`
	Tests     TestsConfig      `yaml:"tests"`
	Dashboard DashboardConfig  `yaml:"dashboard"`
	Retention RetentionConfig  `yaml:"retention"`
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

// ProviderConfig controls outbound calls to one OpenAI-compatible LLM API.
type ProviderConfig struct {
	ID                 string                  `yaml:"id"`
	Name               string                  `yaml:"name"`
	BaseURL            string                  `yaml:"base_url"`
	HTTPCheckPath      string                  `yaml:"http_check_path"`
	Endpoints          ProviderEndpointsConfig `yaml:"endpoints"`
	Timeout            Duration                `yaml:"timeout"`
	CAFile             string                  `yaml:"ca_file"`
	APIKey             string                  `yaml:"api_key"`
	InsecureSkipVerify bool                    `yaml:"insecure_skip_verify"`
	Retry              RetryConfig             `yaml:"retry"`
	Auth               AuthConfig              `yaml:"auth"`
}

// ProviderEndpointsConfig controls the OpenAI-like endpoint URLs used by probes.
type ProviderEndpointsConfig struct {
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
	ProviderID string   `yaml:"provider_id"`
	ModelID    string   `yaml:"model_id"`
	Pattern    string   `yaml:"pattern"`
	Interval   Duration `yaml:"interval"`
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
