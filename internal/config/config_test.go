package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoadAppliesProviderDefaults(t *testing.T) {
	cfg := loadYAML(t, `
postgres:
  dsn: postgres://user:pass@localhost:5432/monitor
providers:
  - id: production
    base_url: https://llm.example.test
    ca_file: /run/certs/llm-api-ca.crt
dashboard:
  default_window: 24h
`)
	provider := cfg.Providers[0]
	if provider.ID != "production" {
		t.Fatalf("provider id = %q, want production", provider.ID)
	}
	if provider.CAFile != "/run/certs/llm-api-ca.crt" {
		t.Fatalf("provider ca file = %q", provider.CAFile)
	}
	if provider.Endpoints.Models != "/v1/models" || provider.Endpoints.Chat != "/v1/chat/completions" || provider.Endpoints.Embeddings != "/v1/embeddings" {
		t.Fatalf("unexpected provider endpoints: %#v", provider.Endpoints)
	}
	if provider.HTTPCheckPath != provider.Endpoints.Models {
		t.Fatalf("http check path = %q, want models endpoint %q", provider.HTTPCheckPath, provider.Endpoints.Models)
	}
	if !provider.Retry.EnabledValue() || provider.Retry.MaxRetriesValue() != 2 {
		t.Fatalf("unexpected provider retry defaults: %#v", provider.Retry)
	}
	if provider.Retry.WaitMinValue() != 500*time.Millisecond || provider.Retry.WaitMaxValue() != 5*time.Second {
		t.Fatalf("unexpected provider retry waits: %#v", provider.Retry)
	}
	if provider.Auth.ClientAuthMethod != "client_secret_basic" {
		t.Fatalf("unexpected auth client method %q", provider.Auth.ClientAuthMethod)
	}
	if provider.Auth.Timeout.Duration != 10*time.Second || provider.Auth.RefreshSkew.Duration != 30*time.Second {
		t.Fatalf("unexpected auth duration defaults: %#v", provider.Auth)
	}
	if cfg.Server.Address != ":8080" || cfg.Logging.Level != "info" || cfg.Redis.Addr != "localhost:6379" {
		t.Fatalf("unexpected global defaults: server=%#v logging=%#v redis=%#v", cfg.Server, cfg.Logging, cfg.Redis)
	}
	if cfg.Asynq.Queue != "default" || cfg.Asynq.WorkerConcurrency != 10 {
		t.Fatalf("unexpected asynq defaults: %#v", cfg.Asynq)
	}
	if cfg.Dashboard.DefaultWindow.Duration != 24*time.Hour || cfg.Dashboard.SiteName != "LLM Service Monitor" {
		t.Fatalf("unexpected dashboard defaults: %#v", cfg.Dashboard)
	}
	if cfg.Retention.History.Duration != 90*24*time.Hour {
		t.Fatalf("unexpected retention history %s", cfg.Retention.History.Duration)
	}
}

func TestValidateRequiresProviders(t *testing.T) {
	cfg := Config{Postgres: PostgresConfig{DSN: "postgres://user:pass@localhost:5432/monitor"}}
	cfg.ApplyDefaults()
	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "providers is required") {
		t.Fatalf("Validate() error = %v, want providers requirement", err)
	}
}

func TestValidateRejectsDuplicateProviderIDs(t *testing.T) {
	cfg := baseConfig()
	cfg.Providers = append(cfg.Providers, ProviderConfig{ID: "production", BaseURL: "https://other.example.test"})
	cfg.ApplyDefaults()
	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "providers[1].id must be unique") {
		t.Fatalf("Validate() error = %v, want duplicate provider id requirement", err)
	}
}

func TestValidateRejectsInvalidProviderID(t *testing.T) {
	cfg := baseConfig()
	cfg.Providers[0].ID = "bad/id"
	cfg.ApplyDefaults()
	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "providers[0].id") {
		t.Fatalf("Validate() error = %v, want provider id slug requirement", err)
	}
}

func TestLoadParsesProviderEndpoints(t *testing.T) {
	cfg := loadYAML(t, `
postgres:
  dsn: postgres://user:pass@localhost:5432/monitor
providers:
  - id: production
    base_url: https://llm.example.test/api
    endpoints:
      models: "/custom/models"
      chat: " https://chat.example.test/probe "
      embeddings: "/custom/embeddings"
`)
	provider := cfg.Providers[0]
	if provider.Endpoints.Models != "/custom/models" {
		t.Fatalf("models endpoint = %q, want /custom/models", provider.Endpoints.Models)
	}
	if provider.Endpoints.Chat != "https://chat.example.test/probe" {
		t.Fatalf("chat endpoint = %q, want absolute chat URL", provider.Endpoints.Chat)
	}
	if provider.Endpoints.Embeddings != "/custom/embeddings" {
		t.Fatalf("embeddings endpoint = %q, want /custom/embeddings", provider.Endpoints.Embeddings)
	}
	if provider.HTTPCheckPath != "/custom/models" {
		t.Fatalf("http check path = %q, want /custom/models", provider.HTTPCheckPath)
	}
}

func TestLoadKeepsExplicitHTTPCheckPath(t *testing.T) {
	cfg := loadYAML(t, `
postgres:
  dsn: postgres://user:pass@localhost:5432/monitor
providers:
  - id: production
    base_url: https://llm.example.test
    http_check_path: "/health"
    endpoints:
      models: "/custom/models"
`)
	if cfg.Providers[0].HTTPCheckPath != "/health" {
		t.Fatalf("http check path = %q, want /health", cfg.Providers[0].HTTPCheckPath)
	}
}

func TestValidateRejectsInvalidProviderEndpoints(t *testing.T) {
	tests := []struct {
		name      string
		mutate    func(*ProviderConfig)
		wantError string
	}{
		{
			name: "models",
			mutate: func(provider *ProviderConfig) {
				provider.Endpoints.Models = "v1/models"
				provider.HTTPCheckPath = "/v1/models"
			},
			wantError: "providers[0].endpoints.models",
		},
		{
			name: "chat",
			mutate: func(provider *ProviderConfig) {
				provider.Endpoints.Chat = "ftp://llm.example.test/chat"
			},
			wantError: "providers[0].endpoints.chat",
		},
		{
			name: "embeddings",
			mutate: func(provider *ProviderConfig) {
				provider.Endpoints.Embeddings = "embeddings"
			},
			wantError: "providers[0].endpoints.embeddings",
		},
		{
			name: "http_check_path",
			mutate: func(provider *ProviderConfig) {
				provider.HTTPCheckPath = "health"
			},
			wantError: "providers[0].http_check_path",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := baseConfig()
			cfg.ApplyDefaults()
			tt.mutate(&cfg.Providers[0])
			err := cfg.Validate()
			if err == nil || !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("Validate() error = %v, want %s requirement", err, tt.wantError)
			}
		})
	}
}

func TestLoadParsesProviderRetry(t *testing.T) {
	cfg := loadYAML(t, `
postgres:
  dsn: postgres://user:pass@localhost:5432/monitor
providers:
  - id: production
    base_url: https://llm.example.test
    retry:
      enabled: true
      max_retries: 4
      wait_min: 250ms
      wait_max: 2s
`)
	if !cfg.Providers[0].Retry.EnabledValue() || cfg.Providers[0].Retry.MaxRetriesValue() != 4 {
		t.Fatalf("unexpected retry config: %#v", cfg.Providers[0].Retry)
	}
	if cfg.Providers[0].Retry.WaitMinValue() != 250*time.Millisecond || cfg.Providers[0].Retry.WaitMaxValue() != 2*time.Second {
		t.Fatalf("unexpected retry waits: %#v", cfg.Providers[0].Retry)
	}

	cfg = loadYAML(t, `
postgres:
  dsn: postgres://user:pass@localhost:5432/monitor
providers:
  - id: production
    base_url: https://llm.example.test
    retry:
      max_retries: 0
`)
	if cfg.Providers[0].Retry.EnabledValue() {
		t.Fatalf("retry should be disabled when max_retries is 0: %#v", cfg.Providers[0].Retry)
	}
}

func TestValidateRejectsInvalidProviderRetry(t *testing.T) {
	negativeRetries := -1
	cfg := baseConfig()
	cfg.Providers[0].Retry = RetryConfig{MaxRetries: &negativeRetries}
	cfg.ApplyDefaults()
	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "providers[0].retry.max_retries") {
		t.Fatalf("Validate() error = %v, want retry max requirement", err)
	}

	retries := 2
	cfg.Providers[0].Retry = RetryConfig{
		MaxRetries: &retries,
		WaitMin:    Duration{Duration: 5 * time.Second, Set: true},
		WaitMax:    Duration{Duration: time.Second, Set: true},
	}
	err = cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "providers[0].retry.wait_max") {
		t.Fatalf("Validate() error = %v, want retry wait requirement", err)
	}
}

func TestValidateRejectsInvalidProviderBaseURL(t *testing.T) {
	for _, baseURL := range []string{"/relative", "ftp://llm.example.test", "://bad"} {
		t.Run(baseURL, func(t *testing.T) {
			cfg := baseConfig()
			cfg.Providers[0].BaseURL = baseURL
			cfg.ApplyDefaults()
			err := cfg.Validate()
			if err == nil || !strings.Contains(err.Error(), "providers[0].base_url must be an absolute http or https URL") {
				t.Fatalf("Validate() error = %v, want provider base_url requirement", err)
			}
		})
	}
}

func TestLoadParsesModelRunScheduleOverrides(t *testing.T) {
	cfg := loadYAML(t, `
postgres:
  dsn: postgres://user:pass@localhost:5432/monitor
providers:
  - id: production
    base_url: https://llm.example.test
schedules:
  model_run_overrides:
    - provider_id: "production"
      model_id: "chat-a"
      interval: "5m"
    - pattern: "embedding-*"
      interval: "30m"
`)
	if len(cfg.Schedules.ModelRunOverrides) != 2 {
		t.Fatalf("overrides len = %d, want 2", len(cfg.Schedules.ModelRunOverrides))
	}
	if cfg.Schedules.ModelRunOverrides[0].ProviderID != "production" {
		t.Fatalf("provider override = %q, want production", cfg.Schedules.ModelRunOverrides[0].ProviderID)
	}
	if cfg.Schedules.ModelRunOverrides[0].Interval.Duration != 5*time.Minute {
		t.Fatalf("exact override interval = %s, want 5m", cfg.Schedules.ModelRunOverrides[0].Interval.Duration)
	}
}

func TestValidateRejectsInvalidModelRunScheduleOverrides(t *testing.T) {
	tests := []struct {
		name     string
		override ModelRunScheduleOverride
		want     string
	}{
		{name: "missing selector", override: ModelRunScheduleOverride{Interval: Duration{Duration: time.Minute}}, want: "requires model_id or pattern"},
		{name: "two selectors", override: ModelRunScheduleOverride{ModelID: "a", Pattern: "*", Interval: Duration{Duration: time.Minute}}, want: "must set only one"},
		{name: "missing interval", override: ModelRunScheduleOverride{ModelID: "a"}, want: "interval must be greater than 0"},
		{name: "unknown provider", override: ModelRunScheduleOverride{ProviderID: "missing", ModelID: "a", Interval: Duration{Duration: time.Minute}}, want: "provider_id must reference a configured provider"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := baseConfig()
			cfg.Schedules.ModelRunOverrides = []ModelRunScheduleOverride{tt.override}
			cfg.ApplyDefaults()
			err := cfg.Validate()
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Validate() error = %v, want %s", err, tt.want)
			}
		})
	}
}

func TestLoadAppliesGlobalTLSInsecureSkipVerify(t *testing.T) {
	cfg := loadYAML(t, `
postgres:
  dsn: postgres://user:pass@localhost:5432/monitor
tls:
  insecure_skip_verify: true
providers:
  - id: production
    base_url: https://llm.example.test
`)
	if !cfg.TLS.InsecureSkipVerify {
		t.Fatal("tls.insecure_skip_verify = false, want true")
	}
	if !cfg.Providers[0].InsecureSkipVerify {
		t.Fatal("provider insecure skip verify = false, want true")
	}
	if !cfg.Providers[0].Auth.MTLS.InsecureSkipVerify {
		t.Fatal("auth mtls insecure skip verify = false, want true")
	}
}

func TestValidateRejectsUnknownAuthClientMethod(t *testing.T) {
	cfg := baseConfig()
	cfg.Providers[0].Auth = AuthConfig{ClientAuthMethod: "unsupported"}
	cfg.ApplyDefaults()
	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "providers[0].auth.client_auth_method must be client_secret_basic or client_secret_post") {
		t.Fatalf("Validate() error = %v, want auth client method requirement", err)
	}
}

func TestLoadParsesLogging(t *testing.T) {
	cfg := loadYAML(t, `
logging:
  level: " DEBUG "
postgres:
  dsn: postgres://user:pass@localhost:5432/monitor
providers:
  - id: production
    base_url: https://llm.example.test
`)
	if cfg.Logging.Level != "debug" {
		t.Fatalf("log level = %q, want debug", cfg.Logging.Level)
	}
}

func TestValidateRejectsInvalidLoggingLevel(t *testing.T) {
	cfg := baseConfig()
	cfg.Logging = LoggingConfig{Level: "trace"}
	cfg.ApplyDefaults()
	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "logging.level must be debug, info, warn, or error") {
		t.Fatalf("Validate() error = %v, want logging level requirement", err)
	}
}

func TestLoadParsesDashboardBranding(t *testing.T) {
	cfg := loadYAML(t, `
postgres:
  dsn: postgres://user:pass@localhost:5432/monitor
providers:
  - id: production
    base_url: https://llm.example.test
dashboard:
  site_name: " Platform Monitor "
  site_url: " https://monitor.example.test "
`)
	if cfg.Dashboard.SiteName != "Platform Monitor" {
		t.Fatalf("site name = %q, want Platform Monitor", cfg.Dashboard.SiteName)
	}
	if cfg.Dashboard.SiteURL != "https://monitor.example.test" {
		t.Fatalf("site url = %q, want https://monitor.example.test", cfg.Dashboard.SiteURL)
	}
}

func TestValidateRejectsInvalidDashboardSiteURL(t *testing.T) {
	cfg := baseConfig()
	cfg.Dashboard = DashboardConfig{SiteURL: "/dashboard"}
	cfg.ApplyDefaults()
	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "dashboard.site_url must be an absolute http or https URL") {
		t.Fatalf("Validate() error = %v, want dashboard site_url requirement", err)
	}
}

func TestLoadParsesRetentionHistoryDays(t *testing.T) {
	cfg := loadYAML(t, `
postgres:
  dsn: postgres://user:pass@localhost:5432/monitor
providers:
  - id: production
    base_url: https://llm.example.test
retention:
  history: 90d
`)
	if cfg.Retention.History.Duration != 90*24*time.Hour {
		t.Fatalf("retention history = %s, want 2160h", cfg.Retention.History.Duration)
	}
}

func TestLoadExplicitZeroRetentionDisablesPruning(t *testing.T) {
	cfg := loadYAML(t, `
postgres:
  dsn: postgres://user:pass@localhost:5432/monitor
providers:
  - id: production
    base_url: https://llm.example.test
retention:
  history: 0s
`)
	if cfg.Retention.History.Duration != 0 {
		t.Fatalf("retention history = %s, want disabled 0s", cfg.Retention.History.Duration)
	}
}

func TestValidateRejectsNegativeRetentionHistory(t *testing.T) {
	cfg := baseConfig()
	cfg.Retention = RetentionConfig{History: Duration{Duration: -time.Hour}}
	cfg.ApplyDefaults()
	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "retention.history must be greater than or equal to 0") {
		t.Fatalf("Validate() error = %v, want retention history requirement", err)
	}
}

func TestValidateMCPRequiresBearerToken(t *testing.T) {
	cfg := baseConfig()
	cfg.MCP = MCPConfig{Enabled: true}
	cfg.ApplyDefaults()
	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "mcp.bearer_token is required") {
		t.Fatalf("Validate() error = %v, want mcp bearer token requirement", err)
	}

	cfg.MCP.BearerToken = "mcp-token"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() with bearer token error = %v", err)
	}
}

func TestLoadIgnoresDeprecatedMCPAllowedOrigins(t *testing.T) {
	cfg := loadYAML(t, `
postgres:
  dsn: postgres://user:pass@localhost:5432/monitor
providers:
  - id: production
    base_url: https://llm.example.test
mcp:
  enabled: true
  bearer_token: test-token
  allowed_origins:
    - ""
`)
	if !cfg.MCP.Enabled || cfg.MCP.BearerToken != "test-token" {
		t.Fatalf("unexpected mcp config: %#v", cfg.MCP)
	}
}

func TestExampleConfigsLoad(t *testing.T) {
	for _, path := range []string{"../../config.example.yaml", "../../examples/config.compose.yaml"} {
		if _, err := Load(path); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
	}
}

func loadYAML(t *testing.T, data string) Config {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func baseConfig() Config {
	return Config{
		Postgres: PostgresConfig{DSN: "postgres://user:pass@localhost:5432/monitor"},
		Providers: []ProviderConfig{
			{ID: "production", BaseURL: "https://llm.example.test"},
		},
	}
}
