package config

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"llmservicemonitor/internal/wildcard"
)

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
	providerIDs := validateProviders(c.Providers, &problems)
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
	validateModelRunOverrides(c.Schedules.ModelRunOverrides, providerIDs, &problems)
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

var providerIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._~-]*$`)

func validateProviders(providers []ProviderConfig, problems *[]string) map[string]struct{} {
	seen := map[string]struct{}{}
	if len(providers) == 0 {
		*problems = append(*problems, "providers is required")
		return seen
	}
	for i, provider := range providers {
		prefix := fmt.Sprintf("providers[%d]", i)
		id := strings.TrimSpace(provider.ID)
		if id == "" {
			*problems = append(*problems, prefix+".id is required")
		} else if !providerIDPattern.MatchString(id) {
			*problems = append(*problems, prefix+".id must be a URL-safe slug")
		} else if _, exists := seen[id]; exists {
			*problems = append(*problems, prefix+".id must be unique")
		} else {
			seen[id] = struct{}{}
		}
		if provider.BaseURL == "" {
			*problems = append(*problems, prefix+".base_url is required")
		} else if !isAbsoluteHTTPURL(provider.BaseURL) {
			*problems = append(*problems, prefix+".base_url must be an absolute http or https URL")
		}
		validateProviderEndpoint(prefix+".http_check_path", provider.HTTPCheckPath, problems)
		validateProviderEndpoint(prefix+".endpoints.models", provider.Endpoints.Models, problems)
		validateProviderEndpoint(prefix+".endpoints.chat", provider.Endpoints.Chat, problems)
		validateProviderEndpoint(prefix+".endpoints.embeddings", provider.Endpoints.Embeddings, problems)
		validateProviderRetry(prefix+".retry", provider.Retry, problems)
		validateProviderAuth(prefix+".auth", provider.Auth, problems)
	}
	return seen
}

func validateProviderEndpoint(name, value string, problems *[]string) {
	if !isHTTPPathOrURL(value) {
		*problems = append(*problems, name+" must start with / or be an absolute http or https URL")
	}
}

func validateProviderRetry(prefix string, retry RetryConfig, problems *[]string) {
	if retry.MaxRetries != nil && *retry.MaxRetries < 0 {
		*problems = append(*problems, prefix+".max_retries must be greater than or equal to 0")
	}
	if !retry.EnabledValue() {
		return
	}
	if retry.WaitMin.Duration <= 0 {
		*problems = append(*problems, prefix+".wait_min must be greater than 0 when retry is enabled")
	}
	if retry.WaitMax.Duration <= 0 {
		*problems = append(*problems, prefix+".wait_max must be greater than 0 when retry is enabled")
	}
	if retry.WaitMax.Duration < retry.WaitMin.Duration {
		*problems = append(*problems, prefix+".wait_max must be greater than or equal to "+prefix+".wait_min")
	}
}

func validateProviderAuth(prefix string, auth AuthConfig, problems *[]string) {
	if auth.Enabled && auth.TokenURL == "" {
		*problems = append(*problems, prefix+".token_url is required when enabled=true")
	}
	if auth.ClientAuthMethod != "client_secret_basic" && auth.ClientAuthMethod != "client_secret_post" {
		*problems = append(*problems, prefix+".client_auth_method must be client_secret_basic or client_secret_post")
	}
}

func validateModelRunOverrides(overrides []ModelRunScheduleOverride, providerIDs map[string]struct{}, problems *[]string) {
	for i, override := range overrides {
		modelID := strings.TrimSpace(override.ModelID)
		pattern := strings.TrimSpace(override.Pattern)
		providerID := strings.TrimSpace(override.ProviderID)
		if providerID != "" {
			if _, ok := providerIDs[providerID]; !ok {
				*problems = append(*problems, fmt.Sprintf("schedules.model_run_overrides[%d].provider_id must reference a configured provider", i))
			}
		}
		switch {
		case modelID == "" && pattern == "":
			*problems = append(*problems, fmt.Sprintf("schedules.model_run_overrides[%d] requires model_id or pattern", i))
		case modelID != "" && pattern != "":
			*problems = append(*problems, fmt.Sprintf("schedules.model_run_overrides[%d] must set only one of model_id or pattern", i))
		}
		if override.Interval.Duration <= 0 {
			*problems = append(*problems, fmt.Sprintf("schedules.model_run_overrides[%d].interval must be greater than 0", i))
		}
		if pattern != "" {
			if _, err := wildcard.Compile(pattern); err != nil {
				*problems = append(*problems, fmt.Sprintf("schedules.model_run_overrides[%d].pattern is invalid", i))
			}
		}
	}
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
