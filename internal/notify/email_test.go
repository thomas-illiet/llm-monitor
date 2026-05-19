package notify

import (
	"io"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"llmservicemonitor/internal/config"
)

// TestModelAlertMessageBuildsMultipartHTML verifies model alerts render text and HTML parts.
func TestModelAlertMessageBuildsMultipartHTML(t *testing.T) {
	message := NewModelAlertMessage(ModelAlert{
		Type:     "inactive",
		Subject:  "LLM model inactive for more than 24h",
		ModelID:  "gpt-4.1",
		Summary:  "Model gpt-4.1 has been inactive since 2026-05-15T08:30:00Z, which is longer than 24h.",
		SiteName: "Platform Monitor",
		SiteURL:  "https://monitor.example.test",
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
		"#f7f7f4",
		"#10a37f",
		"Platform Monitor",
		"Attention required",
		"Check the dashboard",
		"https://monitor.example.test",
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
	if !strings.Contains(email, "Dashboard: https://monitor.example.test") {
		t.Fatalf("text fallback did not include dashboard URL:\n%s", email)
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

// TestSendMailDevPreview sends one manually-triggered preview email to local MailDev.
func TestSendMailDevPreview(t *testing.T) {
	if os.Getenv("MAILDEV_PREVIEW") != "1" {
		t.Skip("set MAILDEV_PREVIEW=1 to send a preview email to MailDev")
	}

	port := 1025
	if rawPort := os.Getenv("MAILDEV_PORT"); rawPort != "" {
		parsed, err := strconv.Atoi(rawPort)
		if err != nil {
			t.Fatalf("parse MAILDEV_PORT: %v", err)
		}
		port = parsed
	}

	notifier, err := NewSMTPNotifier(config.SMTPConfig{
		Enabled:  true,
		Host:     envOrDefault("MAILDEV_HOST", "localhost"),
		Port:     port,
		From:     "llm-monitor@local.test",
		To:       []string{"platform@local.test"},
		StartTLS: false,
	}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("build notifier: %v", err)
	}

	message := NewModelAlertMessage(ModelAlert{
		Type:     "inactive",
		Subject:  "LLM model inactive for more than 24h",
		ModelID:  "gpt-oss:20b",
		Summary:  "Model gpt-oss:20b has been inactive since 2026-05-17T08:30:00Z, which is longer than 24h.",
		SiteName: "Local LLM Monitor",
		SiteURL:  "http://localhost:18080",
		Fields: []AlertField{
			{Label: "Model", Value: "gpt-oss:20b"},
			{Label: "Target", Value: "local-ollama"},
			{Label: "Inactive since", Value: "2026-05-17T08:30:00Z"},
			{Label: "Alert threshold", Value: "24h"},
		},
		SentAt: time.Now().UTC(),
	})

	if err := notifier.Send(message); err != nil {
		t.Fatalf("send preview email: %v", err)
	}
}

func envOrDefault(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
