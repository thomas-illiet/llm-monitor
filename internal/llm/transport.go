package llm

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/hashicorp/go-retryablehttp"

	"llmservicemonitor/internal/config"
)

// targetHTTPClient creates the outbound client used by all LLM API requests.
func targetHTTPClient(cfg config.TargetConfig) (*http.Client, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	var tlsConfig *tls.Config
	if cfg.CAFile != "" {
		ca, err := os.ReadFile(cfg.CAFile)
		if err != nil {
			return nil, fmt.Errorf("read target ca: %w", err)
		}
		pool, err := x509.SystemCertPool()
		if err != nil {
			pool = x509.NewCertPool()
		}
		if pool == nil {
			pool = x509.NewCertPool()
		}
		if !pool.AppendCertsFromPEM(ca) {
			return nil, errors.New("failed to parse target ca")
		}
		tlsConfig = &tls.Config{
			MinVersion: tls.VersionTLS12,
			RootCAs:    pool,
		}
	}
	if cfg.InsecureSkipVerify {
		if tlsConfig == nil {
			tlsConfig = &tls.Config{MinVersion: tls.VersionTLS12}
		}
		tlsConfig.InsecureSkipVerify = true
	}
	if tlsConfig != nil {
		transport.TLSClientConfig = tlsConfig
	}
	if !cfg.Retry.EnabledValue() {
		return &http.Client{Timeout: cfg.Timeout.Duration, Transport: transport}, nil
	}
	retryClient := retryablehttp.NewClient()
	retryClient.HTTPClient = &http.Client{Transport: transport}
	retryClient.Logger = nil
	retryClient.RetryMax = cfg.Retry.MaxRetriesValue()
	retryClient.RetryWaitMin = cfg.Retry.WaitMinValue()
	retryClient.RetryWaitMax = cfg.Retry.WaitMaxValue()
	retryClient.ErrorHandler = func(resp *http.Response, err error, _ int) (*http.Response, error) {
		if resp != nil {
			return resp, nil
		}
		return nil, err
	}
	return &http.Client{
		Timeout:   cfg.Timeout.Duration,
		Transport: &retryablehttp.RoundTripper{Client: retryClient},
	}, nil
}

// postJSON executes a JSON POST and normalizes response timing and errors.
func (c *Client) postJSON(ctx context.Context, start time.Time, endpoint string, payload any) (RunResult, []byte) {
	model := payloadModel(payload)
	endpointLabel := safeEndpointLabel(endpoint)
	c.logger.Debug("llm post request started", "endpoint", endpointLabel, "model", model)
	body, err := json.Marshal(payload)
	if err != nil {
		c.logger.Warn("llm post request encode failed", "endpoint", endpointLabel, "model", model, "error", err)
		return RunResult{StartedAt: start.UTC(), Latency: time.Since(start), Error: err.Error()}, nil
	}
	req, err := c.newRequest(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		c.logger.Warn("llm post request build failed", "endpoint", endpointLabel, "model", model, "error", err)
		return RunResult{StartedAt: start.UTC(), Latency: time.Since(start), Error: err.Error()}, nil
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.logger.Warn("llm post request failed", "endpoint", endpointLabel, "model", model, "latency_ms", millisSince(start), "error", err)
		return RunResult{StartedAt: start.UTC(), Latency: time.Since(start), Error: err.Error()}, nil
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		c.logger.Warn("llm post response read failed", "endpoint", endpointLabel, "model", model, "status", resp.StatusCode, "latency_ms", millisSince(start), "error", err)
		return RunResult{StartedAt: start.UTC(), StatusCode: resp.StatusCode, Latency: time.Since(start), Error: err.Error()}, nil
	}
	result := RunResult{
		StartedAt:  start.UTC(),
		OK:         resp.StatusCode >= 200 && resp.StatusCode < 300,
		StatusCode: resp.StatusCode,
		Latency:    time.Since(start),
	}
	if !result.OK {
		result.Error = fmt.Sprintf("llm returned %d: %s", resp.StatusCode, trimBody(respBody))
		c.logger.Warn("llm post request returned error", "endpoint", endpointLabel, "model", model, "status", resp.StatusCode, "latency_ms", millisSince(start), "error", result.Error)
		return result, respBody
	}
	c.logger.Debug("llm post request completed", "endpoint", endpointLabel, "model", model, "status", resp.StatusCode, "latency_ms", millisSince(start))
	return result, respBody
}

// newRequest builds an authenticated request against the target base URL.
func (c *Client) newRequest(ctx context.Context, method, endpoint string, body io.Reader) (*http.Request, error) {
	target, err := c.resolveEndpoint(endpoint)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, method, target, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if c.tokenProvider != nil {
		token, _, err := c.tokenProvider.Token(ctx)
		if err != nil {
			return nil, err
		}
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
	}
	return req, nil
}

func (c *Client) resolveEndpoint(endpoint string) (string, error) {
	endpoint = strings.TrimSpace(endpoint)
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return "", err
	}
	if parsed.IsAbs() {
		if (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
			return "", fmt.Errorf("endpoint must use http or https: %s", safeEndpointLabel(endpoint))
		}
		return parsed.String(), nil
	}
	if !strings.HasPrefix(endpoint, "/") {
		return "", fmt.Errorf("endpoint must start with / or be an absolute URL: %s", endpoint)
	}
	target := *c.baseURL
	target.Path = strings.TrimRight(c.baseURL.Path, "/") + "/" + strings.TrimLeft(parsed.Path, "/")
	target.RawQuery = parsed.RawQuery
	target.ForceQuery = parsed.ForceQuery
	target.Fragment = parsed.Fragment
	return target.String(), nil
}

func payloadModel(payload any) string {
	values, ok := payload.(map[string]any)
	if !ok {
		return ""
	}
	model, _ := values["model"].(string)
	return model
}

func safeEndpointLabel(endpoint string) string {
	parsed, err := url.Parse(strings.TrimSpace(endpoint))
	if err != nil {
		return endpoint
	}
	if parsed.IsAbs() {
		parsed.RawQuery = ""
		parsed.ForceQuery = false
		parsed.Fragment = ""
		return parsed.String()
	}
	return parsed.Path
}

func millisSince(start time.Time) float64 {
	return float64(time.Since(start).Microseconds()) / 1000
}

// trimBody keeps upstream error payloads compact when stored or returned.
func trimBody(body []byte) string {
	text := strings.TrimSpace(string(body))
	if len(text) > 512 {
		return text[:512]
	}
	return text
}
