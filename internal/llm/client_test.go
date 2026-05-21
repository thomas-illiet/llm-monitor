package llm

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"llmservicemonitor/internal/config"
)

// TestClientUsesTargetCustomCA verifies LLM API calls trust target.ca_file.
func TestClientUsesTargetCustomCA(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"gpt-test"}]}`))
	}))
	defer server.Close()

	caPath := writeServerCertificate(t, server)
	client, err := NewClient(config.TargetConfig{
		BaseURL:       server.URL,
		HTTPCheckPath: "/v1/models",
		Timeout:       config.Duration{Duration: 2 * time.Second},
		CAFile:        caPath,
	}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	models, err := client.ListModels(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	assertModelIDs(t, models, []string{"gpt-test"})
}

// TestClientCanSkipTargetTLSVerification verifies the global TLS escape hatch
// can be propagated to target HTTP requests.
func TestClientCanSkipTargetTLSVerification(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"gpt-test"}]}`))
	}))
	defer server.Close()

	client, err := NewClient(config.TargetConfig{
		BaseURL:            server.URL,
		HTTPCheckPath:      "/v1/models",
		Timeout:            config.Duration{Duration: 2 * time.Second},
		InsecureSkipVerify: true,
	}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	models, err := client.ListModels(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	assertModelIDs(t, models, []string{"gpt-test"})
}

// TestClientUsesCustomTargetEndpoints verifies model, chat, embedding, and
// health calls use configurable endpoint paths.
func TestClientUsesCustomTargetEndpoints(t *testing.T) {
	var modelCalls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/vendor/models":
			atomic.AddInt32(&modelCalls, 1)
			_, _ = w.Write([]byte(`{"data":[{"id":"gpt-test"}]}`))
		case "/vendor/chat":
			if r.Method != http.MethodPost {
				t.Fatalf("chat method = %s, want POST", r.Method)
			}
			_, _ = w.Write([]byte(`{"usage":{"prompt_tokens":2,"completion_tokens":3,"total_tokens":5}}`))
		case "/vendor/embeddings":
			if r.Method != http.MethodPost {
				t.Fatalf("embedding method = %s, want POST", r.Method)
			}
			_, _ = w.Write([]byte(`{"data":[{"embedding":[0.1,0.2,0.3]}],"usage":{"prompt_tokens":7,"total_tokens":7}}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client, err := NewClient(config.TargetConfig{
		BaseURL: server.URL,
		Timeout: config.Duration{Duration: 2 * time.Second},
		Endpoints: config.TargetEndpointsConfig{
			Models:     "/vendor/models",
			Chat:       "/vendor/chat",
			Embeddings: "/vendor/embeddings",
		},
	}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	models, err := client.ListModels(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	assertModelIDs(t, models, []string{"gpt-test"})
	health := client.HealthCheck(context.Background())
	if !health.OK {
		t.Fatalf("health check failed: %#v", health)
	}
	if modelCalls != 2 {
		t.Fatalf("models endpoint calls = %d, want 2", modelCalls)
	}

	chat := client.RunChat(context.Background(), ChatRequest{
		Model:       "gpt-test",
		Prompt:      "hello",
		MaxTokens:   4,
		Temperature: 0,
	})
	if !chat.OK || chat.InputTokens == nil || *chat.InputTokens != 2 || chat.OutputTokens == nil || *chat.OutputTokens != 3 {
		t.Fatalf("unexpected chat result: %#v", chat)
	}

	embedding := client.RunEmbedding(context.Background(), "embed-test", "hello")
	if !embedding.OK || embedding.VectorDimensions == nil || *embedding.VectorDimensions != 3 {
		t.Fatalf("unexpected embedding result: %#v", embedding)
	}
}

// TestClientUsesAbsoluteEndpointURL verifies an endpoint can target a full URL
// instead of being joined to target.base_url.
func TestClientUsesAbsoluteEndpointURL(t *testing.T) {
	var absoluteCalls int32
	absoluteServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&absoluteCalls, 1)
		if r.URL.Path != "/absolute/models" {
			t.Fatalf("absolute path = %s, want /absolute/models", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"absolute-model"}]}`))
	}))
	defer absoluteServer.Close()

	baseServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("base server should not be called, got %s", r.URL.Path)
	}))
	defer baseServer.Close()

	client, err := NewClient(config.TargetConfig{
		BaseURL: baseServer.URL,
		Timeout: config.Duration{Duration: 2 * time.Second},
		Endpoints: config.TargetEndpointsConfig{
			Models: absoluteServer.URL + "/absolute/models",
		},
	}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	models, err := client.ListModels(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	assertModelIDs(t, models, []string{"absolute-model"})
	if absoluteCalls != 1 {
		t.Fatalf("absolute endpoint calls = %d, want 1", absoluteCalls)
	}
}

// TestClientRetriesTransientHTTPFailures verifies retryable target requests recover from 5xx responses.
func TestClientRetriesTransientHTTPFailures(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen := atomic.AddInt32(&calls, 1)
		if seen < 3 {
			http.Error(w, "temporary outage", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"gpt-test"}]}`))
	}))
	defer server.Close()

	retries := 2
	client, err := NewClient(config.TargetConfig{
		BaseURL:       server.URL,
		HTTPCheckPath: "/v1/models",
		Timeout:       config.Duration{Duration: 2 * time.Second},
		Retry: config.RetryConfig{
			MaxRetries: &retries,
			WaitMin:    config.Duration{Duration: time.Millisecond, Set: true},
			WaitMax:    config.Duration{Duration: time.Millisecond, Set: true},
		},
	}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	models, err := client.ListModels(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	assertModelIDs(t, models, []string{"gpt-test"})
	if calls != 3 {
		t.Fatalf("calls = %d, want 3", calls)
	}
}

func TestClientListModelsKeepsRedactedProviderMetadata(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"data": [{
				"id": "provider/model",
				"object": "model",
				"owned_by": "acme",
				"created": 1710000000,
				"api_key": "secret-value",
				"nested": {
					"access_token": "token-value",
					"safe": "visible"
				},
				"variants": [{
					"name": "fast",
					"password": "hidden"
				}]
			}]
		}`))
	}))
	defer server.Close()

	client, err := NewClient(config.TargetConfig{
		BaseURL:       server.URL,
		HTTPCheckPath: "/v1/models",
		Timeout:       config.Duration{Duration: 2 * time.Second},
	}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	models, err := client.ListModels(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	assertModelIDs(t, models, []string{"provider/model"})
	metadata := models[0].Metadata
	if metadata["owned_by"] != "acme" || metadata["id"] != "provider/model" {
		t.Fatalf("metadata = %#v, want provider fields", metadata)
	}
	if metadata["api_key"] != "[redacted]" {
		t.Fatalf("api_key = %#v, want redacted", metadata["api_key"])
	}
	nested, ok := metadata["nested"].(map[string]any)
	if !ok || nested["access_token"] != "[redacted]" || nested["safe"] != "visible" {
		t.Fatalf("nested metadata = %#v, want nested redaction", metadata["nested"])
	}
	variants, ok := metadata["variants"].([]any)
	if !ok || len(variants) != 1 {
		t.Fatalf("variants = %#v, want one variant", metadata["variants"])
	}
	variant, ok := variants[0].(map[string]any)
	if !ok || variant["password"] != "[redacted]" || variant["name"] != "fast" {
		t.Fatalf("variant = %#v, want array item redaction", variants[0])
	}
}

// TestClientRetryCanBeDisabled verifies max_retries=0 leaves transient errors un-retried.
func TestClientRetryCanBeDisabled(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		http.Error(w, "temporary outage", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	retries := 0
	client, err := NewClient(config.TargetConfig{
		BaseURL:       server.URL,
		HTTPCheckPath: "/v1/models",
		Timeout:       config.Duration{Duration: 2 * time.Second},
		Retry:         config.RetryConfig{MaxRetries: &retries},
	}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.ListModels(context.Background())
	if err == nil {
		t.Fatal("ListModels() error = nil, want upstream 503 error")
	}
	if calls != 1 {
		t.Fatalf("calls = %d, want 1", calls)
	}
}

// TestHealthCheckKeepsLastRetriedStatusCode verifies failed retries preserve upstream status details.
func TestHealthCheckKeepsLastRetriedStatusCode(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		http.Error(w, "still down", http.StatusBadGateway)
	}))
	defer server.Close()

	retries := 2
	client, err := NewClient(config.TargetConfig{
		BaseURL:       server.URL,
		HTTPCheckPath: "/v1/models",
		Timeout:       config.Duration{Duration: 2 * time.Second},
		Retry: config.RetryConfig{
			MaxRetries: &retries,
			WaitMin:    config.Duration{Duration: time.Millisecond, Set: true},
			WaitMax:    config.Duration{Duration: time.Millisecond, Set: true},
		},
	}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	result := client.HealthCheck(context.Background())

	if result.OK {
		t.Fatal("health check OK = true, want false")
	}
	if result.StatusCode != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502", result.StatusCode)
	}
	if calls != 3 {
		t.Fatalf("calls = %d, want 3", calls)
	}
}

// TestRunChatStreamMeasuresStreamingMetrics verifies SSE chat completions populate streaming metrics.
func TestRunChatStreamMeasuresStreamingMetrics(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("test response writer cannot flush")
		}
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"hel\"}}]}\n\n"))
		flusher.Flush()
		time.Sleep(5 * time.Millisecond)
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"lo\"}}]}\n\n"))
		flusher.Flush()
		_, _ = w.Write([]byte("data: {\"choices\":[],\"usage\":{\"prompt_tokens\":3,\"completion_tokens\":2,\"total_tokens\":5}}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer server.Close()

	client, err := NewClient(config.TargetConfig{
		BaseURL:       server.URL,
		HTTPCheckPath: "/v1/models",
		Timeout:       config.Duration{Duration: 2 * time.Second},
	}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	result := client.RunChatStream(context.Background(), ChatRequest{
		Model:       "gpt-test",
		Prompt:      "hello",
		MaxTokens:   4,
		Temperature: 0,
	})

	if !result.OK {
		t.Fatalf("stream result failed: %s", result.Error)
	}
	if result.TTFT == nil || result.ITL == nil || result.TPOT == nil || result.RequestLatency == nil {
		t.Fatalf("missing streaming metrics: %#v", result)
	}
	if result.InputTokens == nil || *result.InputTokens != 3 {
		t.Fatalf("input tokens = %#v, want 3", result.InputTokens)
	}
	if result.OutputTokens == nil || *result.OutputTokens != 2 {
		t.Fatalf("output tokens = %#v, want 2", result.OutputTokens)
	}
	if result.OutputTokensPerSecond == nil || *result.OutputTokensPerSecond <= 0 {
		t.Fatalf("missing output token rate: %#v", result.OutputTokensPerSecond)
	}
}

func assertModelIDs(t *testing.T, models []ProviderModel, want []string) {
	t.Helper()
	if len(models) != len(want) {
		t.Fatalf("models len = %d, want %d: %#v", len(models), len(want), models)
	}
	for i := range want {
		if models[i].ID != want[i] {
			t.Fatalf("model ids = %#v, want %v", models, want)
		}
	}
}

// writeServerCertificate stores the test server certificate as a PEM CA file.
func writeServerCertificate(t *testing.T, server *httptest.Server) string {
	t.Helper()
	certs := server.TLS.Certificates[0].Certificate
	if len(certs) == 0 {
		t.Fatal("test server has no certificate")
	}
	parsed, err := x509.ParseCertificate(certs[0])
	if err != nil {
		t.Fatal(err)
	}
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: parsed.Raw})
	path := filepath.Join(t.TempDir(), "target-ca.pem")
	if err := os.WriteFile(path, pemBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
