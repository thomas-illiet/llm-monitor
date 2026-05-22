package auth

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"llmservicemonitor/internal/config"
)

// Provider supplies access tokens and reports token endpoint health.
type Provider interface {
	Token(ctx context.Context) (string, time.Time, error)
	Check(ctx context.Context) CheckResult
}

// CheckResult captures one auth provider health check.
type CheckResult struct {
	CheckedAt  time.Time
	OK         bool
	StatusCode int
	Latency    time.Duration
	ExpiresAt  *time.Time
	Error      string
}

// staticProvider returns a configured static bearer token.
type staticProvider struct {
	token string
}

// Token returns the configured static token when OAuth monitoring is disabled.
func (p staticProvider) Token(context.Context) (string, time.Time, error) {
	return p.token, time.Time{}, nil
}

// Check marks static auth as healthy because no token endpoint is configured.
func (p staticProvider) Check(context.Context) CheckResult {
	return CheckResult{CheckedAt: time.Now().UTC(), OK: true}
}

// clientCredentialsProvider caches OAuth2 client_credentials tokens.
type clientCredentialsProvider struct {
	cfg          config.AuthConfig
	clientSecret string
	httpClient   *http.Client
	logger       *slog.Logger

	mu        sync.Mutex
	token     string
	expiresAt time.Time
}

// Providers stores one auth provider per configured LLM provider.
type Providers struct {
	order []string
	byID  map[string]Provider
}

// NewProviders builds auth providers for every configured LLM provider.
func NewProviders(providerCfgs []config.ProviderConfig, logger *slog.Logger) (*Providers, error) {
	providers := &Providers{
		order: make([]string, 0, len(providerCfgs)),
		byID:  make(map[string]Provider, len(providerCfgs)),
	}
	for _, providerCfg := range providerCfgs {
		provider, err := NewProvider(providerCfg.Auth, providerCfg, logger)
		if err != nil {
			return nil, fmt.Errorf("build auth provider %s: %w", providerCfg.ID, err)
		}
		providers.order = append(providers.order, providerCfg.ID)
		providers.byID[providerCfg.ID] = provider
	}
	return providers, nil
}

// ProviderIDs returns the configured provider IDs in config order.
func (p *Providers) ProviderIDs() []string {
	if p == nil {
		return nil
	}
	return append([]string(nil), p.order...)
}

// ForProvider returns the token provider for one configured provider ID.
func (p *Providers) ForProvider(providerID string) Provider {
	if p == nil {
		return nil
	}
	return p.byID[providerID]
}

// Check runs an auth health check for one provider.
func (p *Providers) Check(ctx context.Context, providerID string) CheckResult {
	provider := p.ForProvider(providerID)
	if provider == nil {
		return CheckResult{CheckedAt: time.Now().UTC(), Error: "auth provider is not configured"}
	}
	return provider.Check(ctx)
}

// NewProvider builds either a static token provider or an OAuth2 mTLS provider.
func NewProvider(authCfg config.AuthConfig, providerCfg config.ProviderConfig, logger *slog.Logger) (Provider, error) {
	if authCfg.ClientAuthMethod == "" {
		authCfg.ClientAuthMethod = "client_secret_basic"
	}
	if !authCfg.Enabled {
		return staticProvider{token: strings.TrimSpace(providerCfg.APIKey)}, nil
	}

	httpClient, err := mtlsHTTPClient(authCfg)
	if err != nil {
		return nil, err
	}
	return &clientCredentialsProvider{
		cfg:          authCfg,
		clientSecret: strings.TrimSpace(authCfg.ClientSecret),
		httpClient:   httpClient,
		logger:       logger,
	}, nil
}

// Token returns a cached access token or refreshes it before it expires.
func (p *clientCredentialsProvider) Token(ctx context.Context) (string, time.Time, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.token != "" && time.Until(p.expiresAt) > p.cfg.RefreshSkew.Duration {
		return p.token, p.expiresAt, nil
	}
	token, expiresAt, _, err := p.fetch(ctx)
	if err != nil {
		return "", time.Time{}, err
	}
	p.token = token
	p.expiresAt = expiresAt
	return token, expiresAt, nil
}

// Check forces a token endpoint call and captures latency and OAuth/TLS errors.
func (p *clientCredentialsProvider) Check(ctx context.Context) CheckResult {
	start := time.Now()
	p.mu.Lock()
	defer p.mu.Unlock()
	token, expiresAt, status, err := p.fetch(ctx)
	result := CheckResult{
		CheckedAt:  start.UTC(),
		StatusCode: status,
		Latency:    time.Since(start),
	}
	if err != nil {
		result.Error = err.Error()
		return result
	}
	p.token = token
	p.expiresAt = expiresAt
	result.OK = true
	result.ExpiresAt = &expiresAt
	return result
}

// fetch performs one OAuth2 client_credentials token request.
func (p *clientCredentialsProvider) fetch(ctx context.Context) (string, time.Time, int, error) {
	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	if p.cfg.ClientAuthMethod == "client_secret_post" {
		if p.cfg.ClientID != "" {
			form.Set("client_id", p.cfg.ClientID)
		}
		if p.clientSecret != "" {
			form.Set("client_secret", p.clientSecret)
		}
	}
	if len(p.cfg.Scopes) > 0 {
		form.Set("scope", strings.Join(p.cfg.Scopes, " "))
	}
	if p.cfg.Audience != "" {
		form.Set("audience", p.cfg.Audience)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.cfg.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", time.Time{}, 0, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	if p.cfg.ClientAuthMethod == "client_secret_basic" {
		req.SetBasicAuth(p.cfg.ClientID, p.clientSecret)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", time.Time{}, 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", time.Time{}, resp.StatusCode, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", time.Time{}, resp.StatusCode, fmt.Errorf("token endpoint returned %d: %s", resp.StatusCode, trimBody(body))
	}

	var decoded struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int64  `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &decoded); err != nil {
		return "", time.Time{}, resp.StatusCode, err
	}
	if decoded.AccessToken == "" {
		return "", time.Time{}, resp.StatusCode, errors.New("token response missing access_token")
	}
	if decoded.ExpiresIn <= 0 {
		decoded.ExpiresIn = 300
	}
	return decoded.AccessToken, time.Now().UTC().Add(time.Duration(decoded.ExpiresIn) * time.Second), resp.StatusCode, nil
}

// mtlsHTTPClient creates an HTTP client with optional client cert and CA trust.
func mtlsHTTPClient(cfg config.AuthConfig) (*http.Client, error) {
	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12, InsecureSkipVerify: cfg.MTLS.InsecureSkipVerify}
	if cfg.MTLS.CertFile != "" || cfg.MTLS.KeyFile != "" {
		cert, err := tls.LoadX509KeyPair(cfg.MTLS.CertFile, cfg.MTLS.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("load mtls cert/key: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}
	if cfg.MTLS.CAFile != "" {
		ca, err := os.ReadFile(cfg.MTLS.CAFile)
		if err != nil {
			return nil, fmt.Errorf("read mtls ca: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(ca) {
			return nil, errors.New("failed to parse mtls ca")
		}
		tlsConfig.RootCAs = pool
	}
	return &http.Client{
		Timeout: cfg.Timeout.Duration,
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
		},
	}, nil
}

// trimBody keeps remote error bodies small enough for logs and API responses.
func trimBody(body []byte) string {
	text := strings.TrimSpace(string(body))
	if len(text) > 512 {
		return text[:512]
	}
	return text
}
