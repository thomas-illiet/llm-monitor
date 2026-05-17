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
	"os"
	"strings"
	"time"

	"llmservicemonitor/internal/config"
)

// targetHTTPClient creates the outbound client used by all LLM API requests.
func targetHTTPClient(cfg config.TargetConfig) (*http.Client, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
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
		transport.TLSClientConfig = &tls.Config{
			MinVersion: tls.VersionTLS12,
			RootCAs:    pool,
		}
	}
	return &http.Client{Timeout: cfg.Timeout.Duration, Transport: transport}, nil
}

// postJSON executes a JSON POST and normalizes response timing and errors.
func (c *Client) postJSON(ctx context.Context, start time.Time, endpoint string, payload any) (RunResult, []byte) {
	body, err := json.Marshal(payload)
	if err != nil {
		return RunResult{StartedAt: start.UTC(), Latency: time.Since(start), Error: err.Error()}, nil
	}
	req, err := c.newRequest(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return RunResult{StartedAt: start.UTC(), Latency: time.Since(start), Error: err.Error()}, nil
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return RunResult{StartedAt: start.UTC(), Latency: time.Since(start), Error: err.Error()}, nil
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
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
	}
	return result, respBody
}

// newRequest builds an authenticated request against the target base URL.
func (c *Client) newRequest(ctx context.Context, method, endpoint string, body io.Reader) (*http.Request, error) {
	target := *c.baseURL
	target.Path = strings.TrimRight(c.baseURL.Path, "/") + "/" + strings.TrimLeft(endpoint, "/")
	req, err := http.NewRequestWithContext(ctx, method, target.String(), body)
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

// trimBody keeps upstream error payloads compact when stored or returned.
func trimBody(body []byte) string {
	text := strings.TrimSpace(string(body))
	if len(text) > 512 {
		return text[:512]
	}
	return text
}
