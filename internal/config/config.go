package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Duration wraps time.Duration with YAML string parsing.
type Duration struct {
	time.Duration
}

// UnmarshalYAML parses human-readable duration values such as "30s", "24h", or "90d".
func (d *Duration) UnmarshalYAML(value *yaml.Node) error {
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
	Postgres  PostgresConfig  `yaml:"postgres"`
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

// PostgresConfig controls persistence connectivity.
type PostgresConfig struct {
	DSN string `yaml:"dsn"`
}

// TargetConfig controls outbound calls to the OpenAI-compatible LLM API.
type TargetConfig struct {
	Name          string   `yaml:"name"`
	BaseURL       string   `yaml:"base_url"`
	HTTPCheckPath string   `yaml:"http_check_path"`
	Timeout       Duration `yaml:"timeout"`
	CAFile        string   `yaml:"ca_file"`
	APIKey        string   `yaml:"api_key"`
	APIKeyFile    string   `yaml:"api_key_file"`
}

// AuthConfig controls optional OAuth2 client credentials authentication.
type AuthConfig struct {
	Enabled      bool       `yaml:"enabled"`
	TokenURL     string     `yaml:"token_url"`
	ClientID     string     `yaml:"client_id"`
	ClientSecret string     `yaml:"client_secret"`
	SecretFile   string     `yaml:"client_secret_file"`
	Scopes       []string   `yaml:"scopes"`
	Audience     string     `yaml:"audience"`
	Timeout      Duration   `yaml:"timeout"`
	RefreshSkew  Duration   `yaml:"refresh_skew"`
	MTLS         MTLSConfig `yaml:"mtls"`
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
	PasswordFile       string   `yaml:"password_file"`
	From               string   `yaml:"from"`
	To                 []string `yaml:"to"`
	StartTLS           bool     `yaml:"starttls"`
	InsecureSkipVerify bool     `yaml:"insecure_skip_verify"`
}

// MCPConfig controls the optional Streamable HTTP MCP endpoint.
type MCPConfig struct {
	Enabled         bool     `yaml:"enabled"`
	Path            string   `yaml:"path"`
	BearerToken     string   `yaml:"bearer_token"`
	BearerTokenFile string   `yaml:"bearer_token_file"`
	AllowedOrigins  []string `yaml:"allowed_origins"`
}

// ScheduleConfig controls recurring monitor intervals.
type ScheduleConfig struct {
	HTTPCheck     Duration `yaml:"http_check"`
	AuthCheck     Duration `yaml:"auth_check"`
	ModelSnapshot Duration `yaml:"model_snapshot"`
	ModelRuns     Duration `yaml:"model_runs"`
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
	if c.Target.Name == "" {
		c.Target.Name = "default"
	}
	if c.Target.HTTPCheckPath == "" {
		c.Target.HTTPCheckPath = "/v1/models"
	}
	if c.Target.Timeout.Duration == 0 {
		c.Target.Timeout.Duration = 30 * time.Second
	}
	if c.Auth.Timeout.Duration == 0 {
		c.Auth.Timeout.Duration = 10 * time.Second
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
}

// Validate checks that required config fields and enum-like values are usable.
func (c Config) Validate() error {
	var problems []string
	if c.Postgres.DSN == "" {
		problems = append(problems, "postgres.dsn is required")
	}
	if c.Target.BaseURL == "" {
		problems = append(problems, "target.base_url is required")
	}
	if c.Auth.Enabled && c.Auth.TokenURL == "" {
		problems = append(problems, "auth.token_url is required when auth.enabled=true")
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
		if c.MCP.BearerToken == "" && c.MCP.BearerTokenFile == "" {
			problems = append(problems, "mcp.bearer_token or mcp.bearer_token_file is required when mcp.enabled=true")
		}
	}
	for _, origin := range c.MCP.AllowedOrigins {
		if strings.TrimSpace(origin) == "" {
			problems = append(problems, "mcp.allowed_origins cannot contain empty values")
			break
		}
	}
	if c.Retention.History.Duration < 0 {
		problems = append(problems, "retention.history must be greater than or equal to 0")
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

// ReadSecret returns an inline secret or reads one from a mounted secret file.
func ReadSecret(value, file string) (string, error) {
	if value != "" {
		return value, nil
	}
	if file == "" {
		return "", nil
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}
