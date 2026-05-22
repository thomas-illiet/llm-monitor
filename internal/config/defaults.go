package config

import (
	"strings"
	"time"
)

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
	for i := range c.Providers {
		applyProviderDefaults(&c.Providers[i], c.TLS)
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

func applyProviderDefaults(provider *ProviderConfig, tls TLSConfig) {
	provider.ID = strings.TrimSpace(provider.ID)
	provider.Name = strings.TrimSpace(provider.Name)
	if provider.Name == "" {
		provider.Name = provider.ID
	}
	provider.BaseURL = strings.TrimSpace(provider.BaseURL)
	provider.Endpoints.Models = strings.TrimSpace(provider.Endpoints.Models)
	if provider.Endpoints.Models == "" {
		provider.Endpoints.Models = "/v1/models"
	}
	provider.Endpoints.Chat = strings.TrimSpace(provider.Endpoints.Chat)
	if provider.Endpoints.Chat == "" {
		provider.Endpoints.Chat = "/v1/chat/completions"
	}
	provider.Endpoints.Embeddings = strings.TrimSpace(provider.Endpoints.Embeddings)
	if provider.Endpoints.Embeddings == "" {
		provider.Endpoints.Embeddings = "/v1/embeddings"
	}
	provider.HTTPCheckPath = strings.TrimSpace(provider.HTTPCheckPath)
	if provider.HTTPCheckPath == "" {
		provider.HTTPCheckPath = provider.Endpoints.Models
	}
	if provider.Timeout.Duration == 0 {
		provider.Timeout.Duration = 30 * time.Second
	}
	if !provider.Retry.WaitMin.Set && provider.Retry.WaitMin.Duration == 0 {
		provider.Retry.WaitMin.Duration = 500 * time.Millisecond
	}
	if !provider.Retry.WaitMax.Set && provider.Retry.WaitMax.Duration == 0 {
		provider.Retry.WaitMax.Duration = 5 * time.Second
	}
	if tls.InsecureSkipVerify {
		provider.InsecureSkipVerify = true
		provider.Auth.MTLS.InsecureSkipVerify = true
	}
	if provider.Auth.Timeout.Duration == 0 {
		provider.Auth.Timeout.Duration = 10 * time.Second
	}
	if provider.Auth.ClientAuthMethod == "" {
		provider.Auth.ClientAuthMethod = "client_secret_basic"
	}
	if provider.Auth.RefreshSkew.Duration == 0 {
		provider.Auth.RefreshSkew.Duration = 30 * time.Second
	}
}
