package notify

import (
	"strings"
	"testing"
	"time"
)

// TestModelAlertMessageBuildsMultipartHTML verifies model alerts render text and HTML parts.
func TestModelAlertMessageBuildsMultipartHTML(t *testing.T) {
	message := NewModelAlertMessage(ModelAlert{
		Type:    "missing",
		Subject: "LLM model missing for more than 24h",
		ModelID: "gpt-4.1",
		Summary: "Model gpt-4.1 has been absent since 2026-05-15T08:30:00Z, which is longer than 24h.",
		Fields: []AlertField{
			{Label: "Model", Value: "gpt-4.1"},
			{Label: "Target", Value: "production-llm"},
			{Label: "Alert threshold", Value: "24h"},
		},
		SentAt: time.Date(2026, 5, 16, 9, 30, 0, 0, time.UTC),
	})

	raw, err := buildEmailMessage("llm-monitor@example.com", []string{"platform@example.com"}, message)
	if err != nil {
		t.Fatalf("buildEmailMessage returned error: %v", err)
	}

	email := string(raw)
	for _, expected := range []string{
		"Content-Type: multipart/alternative;",
		"Content-Type: text/plain; charset=utf-8",
		"Content-Type: text/html; charset=utf-8",
		"#b42318",
		"LLM Service Monitor",
		"Attention required",
		"gpt-4.1",
		"production-llm",
	} {
		if !strings.Contains(email, expected) {
			t.Fatalf("email did not contain %q:\n%s", expected, email)
		}
	}
	if strings.Contains(email, "ZgotmplZ") {
		t.Fatalf("template emitted unsafe placeholder:\n%s", email)
	}
}

// TestPlainMessageBuildsTextOnlyEmail verifies plain messages avoid multipart HTML output.
func TestPlainMessageBuildsTextOnlyEmail(t *testing.T) {
	raw, err := buildEmailMessage("llm-monitor@example.com", []string{"platform@example.com"}, PlainMessage("Plain alert", "Body only"))
	if err != nil {
		t.Fatalf("buildEmailMessage returned error: %v", err)
	}

	email := string(raw)
	if !strings.Contains(email, "Content-Type: text/plain; charset=utf-8") {
		t.Fatalf("expected text/plain email:\n%s", email)
	}
	if strings.Contains(email, "multipart/alternative") {
		t.Fatalf("did not expect multipart email:\n%s", email)
	}
}
