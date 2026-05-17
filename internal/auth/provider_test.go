package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"llmservicemonitor/internal/config"
)

// TestClientCredentialsProviderFetchesAndCachesToken verifies OAuth caching.
func TestClientCredentialsProviderFetchesAndCachesToken(t *testing.T) {
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if got := r.FormValue("grant_type"); got != "client_credentials" {
			t.Fatalf("grant_type = %q", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "token-1",
			"expires_in":   3600,
		})
	}))
	defer server.Close()

	provider, err := NewProvider(config.AuthConfig{
		Enabled:      true,
		TokenURL:     server.URL,
		ClientID:     "client",
		ClientSecret: "secret",
		Timeout:      config.Duration{Duration: 2 * time.Second},
		RefreshSkew:  config.Duration{Duration: time.Minute},
	}, config.TargetConfig{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	token, expiresAt, err := provider.Token(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if token != "token-1" || time.Until(expiresAt) <= 0 {
		t.Fatalf("unexpected token %q expires %s", token, expiresAt)
	}
	if _, _, err := provider.Token(context.Background()); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("expected cache hit, got %d calls", calls)
	}
}
