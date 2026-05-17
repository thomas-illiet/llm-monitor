package llm

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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
	if len(models) != 1 || models[0] != "gpt-test" {
		t.Fatalf("unexpected models: %#v", models)
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
