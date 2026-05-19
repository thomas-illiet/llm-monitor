package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestLoadAppliesDefaults verifies that omitted optional fields get defaults.
func TestLoadAppliesDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	data := []byte(`
postgres:
  dsn: postgres://user:pass@localhost:5432/monitor
target:
  base_url: https://llm.example.test
  ca_file: /run/certs/llm-api-ca.crt
dashboard:
  default_window: 24h
`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Server.Address != ":8080" {
		t.Fatalf("unexpected address %q", cfg.Server.Address)
	}
	if cfg.Target.CAFile != "/run/certs/llm-api-ca.crt" {
		t.Fatalf("unexpected target ca file %q", cfg.Target.CAFile)
	}
	if !cfg.Target.Retry.EnabledValue() || cfg.Target.Retry.MaxRetriesValue() != 2 {
		t.Fatalf("unexpected target retry defaults: %#v", cfg.Target.Retry)
	}
	if cfg.Target.Retry.WaitMinValue() != 500*time.Millisecond || cfg.Target.Retry.WaitMaxValue() != 5*time.Second {
		t.Fatalf("unexpected target retry waits: %#v", cfg.Target.Retry)
	}
	if cfg.Schedules.HTTPCheck.Duration != 30*time.Second {
		t.Fatalf("unexpected http schedule %s", cfg.Schedules.HTTPCheck.Duration)
	}
	if cfg.Models.MaxConcurrency != 4 {
		t.Fatalf("unexpected max concurrency %d", cfg.Models.MaxConcurrency)
	}
	if cfg.MCP.Enabled {
		t.Fatal("mcp should be disabled by default")
	}
	if cfg.MCP.Path != "/mcp" {
		t.Fatalf("unexpected mcp path %q", cfg.MCP.Path)
	}
	if cfg.Dashboard.DefaultWindow.Duration != 24*time.Hour {
		t.Fatalf("unexpected dashboard window %s", cfg.Dashboard.DefaultWindow.Duration)
	}
	if cfg.Dashboard.SiteName != "LLM Service Monitor" {
		t.Fatalf("unexpected site name %q", cfg.Dashboard.SiteName)
	}
	if cfg.Dashboard.SiteURL != "" {
		t.Fatalf("unexpected site url %q", cfg.Dashboard.SiteURL)
	}
	if cfg.Retention.History.Duration != 90*24*time.Hour {
		t.Fatalf("unexpected retention history %s", cfg.Retention.History.Duration)
	}
	if cfg.Dashboard.SLO.TTFTP99MS != 200 || cfg.Dashboard.SLO.ITLP99MS != 50 || cfg.Dashboard.SLO.RequestLatencyP99MS != 3000 {
		t.Fatalf("unexpected dashboard slo defaults: %#v", cfg.Dashboard.SLO)
	}
}

// TestLoadParsesTargetRetry verifies retry configuration supports explicit opt-out.
func TestLoadParsesTargetRetry(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	data := []byte(`
postgres:
  dsn: postgres://user:pass@localhost:5432/monitor
target:
  base_url: https://llm.example.test
  retry:
    enabled: true
    max_retries: 4
    wait_min: 250ms
    wait_max: 2s
`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Target.Retry.EnabledValue() || cfg.Target.Retry.MaxRetriesValue() != 4 {
		t.Fatalf("unexpected retry config: %#v", cfg.Target.Retry)
	}
	if cfg.Target.Retry.WaitMinValue() != 250*time.Millisecond || cfg.Target.Retry.WaitMaxValue() != 2*time.Second {
		t.Fatalf("unexpected retry waits: %#v", cfg.Target.Retry)
	}

	path = filepath.Join(dir, "disabled.yaml")
	data = []byte(`
postgres:
  dsn: postgres://user:pass@localhost:5432/monitor
target:
  base_url: https://llm.example.test
  retry:
    max_retries: 0
`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err = Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Target.Retry.EnabledValue() {
		t.Fatalf("retry should be disabled when max_retries is 0: %#v", cfg.Target.Retry)
	}
}

// TestValidateRejectsInvalidTargetRetry verifies retry values are bounded.
func TestValidateRejectsInvalidTargetRetry(t *testing.T) {
	negativeRetries := -1
	cfg := Config{
		Postgres: PostgresConfig{DSN: "postgres://user:pass@localhost:5432/monitor"},
		Target: TargetConfig{
			BaseURL: "https://llm.example.test",
			Retry:   RetryConfig{MaxRetries: &negativeRetries},
		},
	}
	cfg.ApplyDefaults()
	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "target.retry.max_retries") {
		t.Fatalf("Validate() error = %v, want retry max requirement", err)
	}

	retries := 2
	cfg.Target.Retry = RetryConfig{
		MaxRetries: &retries,
		WaitMin:    Duration{Duration: 5 * time.Second, Set: true},
		WaitMax:    Duration{Duration: time.Second, Set: true},
	}
	err = cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "target.retry.wait_max") {
		t.Fatalf("Validate() error = %v, want retry wait requirement", err)
	}
}

// TestValidateRejectsInvalidTargetBaseURL verifies target URLs must be absolute web URLs.
func TestValidateRejectsInvalidTargetBaseURL(t *testing.T) {
	for _, baseURL := range []string{"/relative", "ftp://llm.example.test", "://bad"} {
		t.Run(baseURL, func(t *testing.T) {
			cfg := Config{
				Postgres: PostgresConfig{DSN: "postgres://user:pass@localhost:5432/monitor"},
				Target:   TargetConfig{BaseURL: baseURL},
			}
			cfg.ApplyDefaults()
			err := cfg.Validate()
			if err == nil || !strings.Contains(err.Error(), "target.base_url must be an absolute http or https URL") {
				t.Fatalf("Validate() error = %v, want target base_url requirement", err)
			}
		})
	}
}

// TestLoadParsesDashboardBranding verifies dashboard branding is loaded and trimmed.
func TestLoadParsesDashboardBranding(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	data := []byte(`
postgres:
  dsn: postgres://user:pass@localhost:5432/monitor
target:
  base_url: https://llm.example.test
dashboard:
  site_name: " Platform Monitor "
  site_url: " https://monitor.example.test "
`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Dashboard.SiteName != "Platform Monitor" {
		t.Fatalf("site name = %q, want Platform Monitor", cfg.Dashboard.SiteName)
	}
	if cfg.Dashboard.SiteURL != "https://monitor.example.test" {
		t.Fatalf("site url = %q, want https://monitor.example.test", cfg.Dashboard.SiteURL)
	}
}

// TestValidateRejectsInvalidDashboardSiteURL verifies dashboard links must be absolute web URLs.
func TestValidateRejectsInvalidDashboardSiteURL(t *testing.T) {
	cfg := Config{
		Postgres:  PostgresConfig{DSN: "postgres://user:pass@localhost:5432/monitor"},
		Target:    TargetConfig{BaseURL: "https://llm.example.test"},
		Dashboard: DashboardConfig{SiteURL: "/dashboard"},
	}
	cfg.ApplyDefaults()
	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "dashboard.site_url must be an absolute http or https URL") {
		t.Fatalf("Validate() error = %v, want dashboard site_url requirement", err)
	}
}

// TestLoadParsesRetentionHistoryDays verifies the retention window accepts day units.
func TestLoadParsesRetentionHistoryDays(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	data := []byte(`
postgres:
  dsn: postgres://user:pass@localhost:5432/monitor
target:
  base_url: https://llm.example.test
retention:
  history: 90d
`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Retention.History.Duration != 90*24*time.Hour {
		t.Fatalf("retention history = %s, want 2160h", cfg.Retention.History.Duration)
	}
}

// TestLoadExplicitZeroRetentionDisablesPruning verifies 0s is distinct from an omitted retention window.
func TestLoadExplicitZeroRetentionDisablesPruning(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	data := []byte(`
postgres:
  dsn: postgres://user:pass@localhost:5432/monitor
target:
  base_url: https://llm.example.test
retention:
  history: 0s
`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Retention.History.Duration != 0 {
		t.Fatalf("retention history = %s, want disabled 0s", cfg.Retention.History.Duration)
	}
}

// TestValidateRejectsNegativeRetentionHistory verifies retention windows cannot be negative.
func TestValidateRejectsNegativeRetentionHistory(t *testing.T) {
	cfg := Config{
		Postgres:  PostgresConfig{DSN: "postgres://user:pass@localhost:5432/monitor"},
		Target:    TargetConfig{BaseURL: "https://llm.example.test"},
		Retention: RetentionConfig{History: Duration{Duration: -time.Hour}},
	}
	cfg.ApplyDefaults()
	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "retention.history must be greater than or equal to 0") {
		t.Fatalf("Validate() error = %v, want retention history requirement", err)
	}
}

// TestValidateMCPRequiresBearerToken verifies enabled MCP endpoints require a dedicated secret.
func TestValidateMCPRequiresBearerToken(t *testing.T) {
	cfg := Config{
		Postgres: PostgresConfig{DSN: "postgres://user:pass@localhost:5432/monitor"},
		Target:   TargetConfig{BaseURL: "https://llm.example.test"},
		MCP:      MCPConfig{Enabled: true},
	}
	cfg.ApplyDefaults()
	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "mcp.bearer_token or mcp.bearer_token_file is required") {
		t.Fatalf("Validate() error = %v, want mcp bearer token requirement", err)
	}

	cfg.MCP.BearerTokenFile = "/run/secrets/mcp-token"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() with bearer token file error = %v", err)
	}
}

// TestLoadIgnoresDeprecatedMCPAllowedOrigins verifies stale configs keep loading after the option was removed.
func TestLoadIgnoresDeprecatedMCPAllowedOrigins(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	data := []byte(`
postgres:
  dsn: postgres://user:pass@localhost:5432/monitor
target:
  base_url: https://llm.example.test
mcp:
  enabled: true
  bearer_token: test-token
  allowed_origins:
    - ""
`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !cfg.MCP.Enabled || cfg.MCP.BearerToken != "test-token" {
		t.Fatalf("unexpected mcp config: %#v", cfg.MCP)
	}
}

// TestExampleConfigsLoad verifies shipped config examples stay valid.
func TestExampleConfigsLoad(t *testing.T) {
	for _, path := range []string{"../../config.example.yaml", "../../examples/config.compose.yaml"} {
		if _, err := Load(path); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
	}
}
