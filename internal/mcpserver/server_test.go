package mcpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"llmservicemonitor/internal/config"
	"llmservicemonitor/internal/store"
)

type fakeStore struct {
	models       []store.ModelState
	authCheck    *store.CheckRecord
	httpCheck    *store.CheckRecord
	kpis         store.KPISummary
	performance  []store.ModelPerformanceRow
	performanceQ store.ModelPerformanceQuery
}

func (f *fakeStore) ListModelStates(context.Context) ([]store.ModelState, error) {
	return f.models, nil
}

func (f *fakeStore) LatestAuthCheck(context.Context, string) (*store.CheckRecord, error) {
	return f.authCheck, nil
}

func (f *fakeStore) LatestHTTPCheck(context.Context, string) (*store.CheckRecord, error) {
	return f.httpCheck, nil
}

func (f *fakeStore) KPISummary(context.Context, time.Time, store.SLOThresholds) (store.KPISummary, error) {
	return f.kpis, nil
}

func (f *fakeStore) ModelPerformance(_ context.Context, query store.ModelPerformanceQuery) ([]store.ModelPerformanceRow, error) {
	f.performanceQ = query
	return f.performance, nil
}

func TestNewHandlerRejectsEmptyBearerToken(t *testing.T) {
	cfg := testConfig()
	cfg.MCP.BearerToken = ""
	if _, err := NewHandler(cfg, testStore(), nil); err == nil {
		t.Fatal("NewHandler() error = nil, want empty bearer token error")
	}
}

func TestHandlerRequiresBearerAndAllowsOriginHeader(t *testing.T) {
	cfg := testConfig()
	handler, err := NewHandler(cfg, testStore(), nil)
	if err != nil {
		t.Fatalf("NewHandler() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader([]byte(initializeRequest(1))))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("missing bearer status = %d, want 401", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader([]byte(initializeRequest(1))))
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("Origin", "https://evil.example.com")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("origin request status = %d, want 200, body = %s", rec.Code, rec.Body.String())
	}
}

func TestToolsListExposesOnlyReadOnlyV1Tools(t *testing.T) {
	ts, _ := newMCPTestServer(t, testStore())
	sessionID := initializeMCP(t, ts)

	resp := mcpPOST(t, ts, sessionID, `{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("tools/list status = %d, body = %s", resp.StatusCode, resp.Body)
	}
	var decoded struct {
		Result struct {
			Tools []struct {
				Name string `json:"name"`
			} `json:"tools"`
		} `json:"result"`
	}
	decodeRPC(t, resp.Body, &decoded)
	var names []string
	for _, tool := range decoded.Result.Tools {
		names = append(names, tool.Name)
	}
	slices.Sort(names)
	want := []string{
		"llm_monitor.kpis",
		"llm_monitor.model_performance",
		"llm_monitor.models",
		"llm_monitor.status",
	}
	if !slices.Equal(names, want) {
		t.Fatalf("tools = %v, want %v", names, want)
	}
}

func TestStatusToolReturnsStructuredContentAndTextJSON(t *testing.T) {
	ts, _ := newMCPTestServer(t, testStore())
	sessionID := initializeMCP(t, ts)

	resp := mcpPOST(t, ts, sessionID, `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"llm_monitor.status","arguments":{}}}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("tools/call status = %d, body = %s", resp.StatusCode, resp.Body)
	}
	var decoded toolCallResponse
	decodeRPC(t, resp.Body, &decoded)
	if decoded.Result.IsError {
		t.Fatalf("status tool returned error: %s", decoded.Result.Content[0].Text)
	}
	if len(decoded.Result.Content) != 1 || decoded.Result.Content[0].Type != "text" {
		t.Fatalf("content = %#v, want one text block", decoded.Result.Content)
	}
	if string(decoded.Result.StructuredContent) != decoded.Result.Content[0].Text {
		t.Fatalf("structuredContent = %s, content text = %s", decoded.Result.StructuredContent, decoded.Result.Content[0].Text)
	}
	var structured statusOutput
	if err := json.Unmarshal(decoded.Result.StructuredContent, &structured); err != nil {
		t.Fatal(err)
	}
	if structured.OK {
		t.Fatal("status OK = true, want false while one model is inactive")
	}
	if structured.Models.Total != 3 || structured.Models.Active != 2 || structured.Models.Inactive != 1 || structured.Models.Missing != 1 || structured.Models.Skipped != 1 {
		t.Fatalf("model counts = %#v", structured.Models)
	}
	if structured.Checks.HTTP.StatusCode != 200 {
		t.Fatalf("http status code = %d, want 200", structured.Checks.HTTP.StatusCode)
	}
}

func TestKPIsRejectsInvalidRangeAsToolError(t *testing.T) {
	ts, _ := newMCPTestServer(t, testStore())
	sessionID := initializeMCP(t, ts)

	resp := mcpPOST(t, ts, sessionID, `{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"llm_monitor.kpis","arguments":{"range":"not-a-duration"}}}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("tools/call status = %d, body = %s", resp.StatusCode, resp.Body)
	}
	var decoded toolCallResponse
	decodeRPC(t, resp.Body, &decoded)
	if !decoded.Result.IsError {
		t.Fatal("IsError = false, want true")
	}
	var structured errorOutput
	if err := json.Unmarshal(decoded.Result.StructuredContent, &structured); err != nil {
		t.Fatal(err)
	}
	if structured.Error != "invalid_range" {
		t.Fatalf("error = %q, want invalid_range", structured.Error)
	}
}

func TestModelPerformanceBoundsLimitAndSort(t *testing.T) {
	store := testStore()
	ts, fake := newMCPTestServer(t, store)
	sessionID := initializeMCP(t, ts)

	resp := mcpPOST(t, ts, sessionID, `{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"llm_monitor.model_performance","arguments":{"range":"48h","sort":"p95_latency_ms","limit":500}}}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("tools/call status = %d, body = %s", resp.StatusCode, resp.Body)
	}
	var decoded toolCallResponse
	decodeRPC(t, resp.Body, &decoded)
	if decoded.Result.IsError {
		t.Fatalf("model_performance returned error: %s", decoded.Result.Content[0].Text)
	}
	if fake.performanceQ.Sort != "p95_latency_ms" {
		t.Fatalf("sort = %q, want p95_latency_ms", fake.performanceQ.Sort)
	}
	if fake.performanceQ.Limit != 100 {
		t.Fatalf("limit = %d, want capped 100", fake.performanceQ.Limit)
	}
	if time.Since(fake.performanceQ.Since) > 49*time.Hour || time.Since(fake.performanceQ.Since) < 47*time.Hour {
		t.Fatalf("since = %s, want about 48h ago", fake.performanceQ.Since)
	}
}

type httpResponse struct {
	StatusCode int
	Header     http.Header
	Body       []byte
}

type toolCallResponse struct {
	Result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		StructuredContent json.RawMessage `json:"structuredContent"`
		IsError           bool            `json:"isError"`
	} `json:"result"`
}

func newMCPTestServer(t *testing.T, fake *fakeStore) (*httptest.Server, *fakeStore) {
	t.Helper()
	cfg := testConfig()
	handler, err := NewHandler(cfg, fake, nil)
	if err != nil {
		t.Fatalf("NewHandler() error = %v", err)
	}
	mux := http.NewServeMux()
	mux.Handle("/mcp", handler)
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return ts, fake
}

func initializeMCP(t *testing.T, ts *httptest.Server) string {
	t.Helper()
	resp := mcpPOST(t, ts, "", initializeRequest(1))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("initialize status = %d, body = %s", resp.StatusCode, resp.Body)
	}
	sessionID := resp.Header.Get("Mcp-Session-Id")
	if sessionID == "" {
		t.Fatal("initialize response missing Mcp-Session-Id")
	}
	initialized := mcpPOST(t, ts, sessionID, `{"jsonrpc":"2.0","method":"notifications/initialized"}`)
	if initialized.StatusCode != http.StatusAccepted {
		t.Fatalf("initialized notification status = %d, body = %s", initialized.StatusCode, initialized.Body)
	}
	return sessionID
}

func mcpPOST(t *testing.T, ts *httptest.Server, sessionID, body string) httpResponse {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, ts.URL+"/mcp", bytes.NewReader([]byte(body)))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	if sessionID != "" {
		req.Header.Set("Mcp-Session-Id", sessionID)
		req.Header.Set("Mcp-Protocol-Version", "2025-11-25")
	}
	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(resp.Body); err != nil {
		t.Fatal(err)
	}
	return httpResponse{StatusCode: resp.StatusCode, Header: resp.Header.Clone(), Body: buf.Bytes()}
}

func initializeRequest(id int) string {
	return `{"jsonrpc":"2.0","id":` + jsonNumber(id) + `,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"test-client","version":"1.0.0"}}}`
}

func jsonNumber(value int) string {
	raw, _ := json.Marshal(value)
	return string(raw)
}

func decodeRPC(t *testing.T, raw []byte, target any) {
	t.Helper()
	var envelope struct {
		Error *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		t.Fatalf("decode RPC envelope: %v; body = %s", err, raw)
	}
	if envelope.Error != nil {
		t.Fatalf("RPC error = %#v; body = %s", envelope.Error, raw)
	}
	if err := json.Unmarshal(raw, target); err != nil {
		t.Fatalf("decode RPC body: %v; body = %s", err, raw)
	}
}

func testConfig() config.Config {
	cfg := config.Config{
		Postgres: config.PostgresConfig{DSN: "postgres://user:pass@localhost:5432/monitor"},
		Providers: []config.ProviderConfig{
			{ID: "openai", Name: "OpenAI", BaseURL: "https://llm.example.test"},
		},
		MCP: config.MCPConfig{
			Enabled:     true,
			BearerToken: "test-token",
		},
		Dashboard: config.DashboardConfig{
			DefaultWindow: config.Duration{Duration: 24 * time.Hour},
		},
	}
	cfg.ApplyDefaults()
	return cfg
}

func testStore() *fakeStore {
	now := time.Date(2026, 5, 17, 16, 41, 49, 0, time.UTC)
	return &fakeStore{
		models: []store.ModelState{
			{ProviderID: "openai", ModelID: "gpt-4o", ModelKey: store.ModelKey("gpt-4o"), Capability: "chat", Status: "active", FirstSeenAt: now.Add(-24 * time.Hour), LastSeenAt: now, LastProbeAt: &now},
			{ProviderID: "openai", ModelID: "skipped", ModelKey: store.ModelKey("skipped"), Capability: "skip", Status: "active", FirstSeenAt: now.Add(-24 * time.Hour), LastSeenAt: now},
			{ProviderID: "openai", ModelID: "inactive", ModelKey: store.ModelKey("inactive"), Capability: "chat", Status: "inactive", FirstSeenAt: now.Add(-24 * time.Hour), LastSeenAt: now.Add(-time.Hour), MissingSince: ptrTime(now.Add(-time.Hour))},
		},
		authCheck: &store.CheckRecord{At: now, OK: true, StatusCode: 0, LatencyMS: 0},
		httpCheck: &store.CheckRecord{At: now, OK: true, StatusCode: 200, LatencyMS: 15.704},
		kpis: store.KPISummary{
			TotalRuns:             737,
			SuccessRate:           0.1384,
			ErrorCount:            635,
			SLOViolationCount:     717,
			DegradedModels:        58,
			LatencyP50MS:          4.432,
			LatencyP95MS:          1567.518,
			LatencyP99MS:          5002.627,
			TTFTP99MS:             2217.251,
			ITLP99MS:              326.886,
			OutputTokensPerSecond: 37.938,
			InputTokens:           1954,
			OutputTokens:          3298,
		},
		performance: []store.ModelPerformanceRow{
			{
				ProviderID:   "openai",
				ModelID:      "@cf/openai/gpt-oss-120b",
				Runs:         13,
				SuccessRate:  0,
				ErrorCount:   13,
				AvgLatencyMS: 56.77,
				P50LatencyMS: 4.5,
				P95LatencyMS: 249.93,
				P99LatencyMS: 249.93,
				FirstRunAt:   now.Add(-20 * time.Hour),
				LastRunAt:    now.Add(-2 * time.Hour),
				LatestError:  &store.ModelPerformanceError{StatusCode: 429, Message: "llm returned 429: ..."},
			},
		},
	}
}

func ptrTime(value time.Time) *time.Time {
	return &value
}
