package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestLoadAppliesDefaults verifies that omitted optional fields get defaults.
func TestLoadAppliesDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	data := []byte(`
postgres:
  dsn: postgres://user:pass@localhost:5432/monitor
target:
  base_url: https://llm.example.test
  ca_file: /run/certs/llm-api-ca.crt
dashboard:
  default_window: 24h
`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Server.Address != ":8080" {
		t.Fatalf("unexpected address %q", cfg.Server.Address)
	}
	if cfg.Target.CAFile != "/run/certs/llm-api-ca.crt" {
		t.Fatalf("unexpected target ca file %q", cfg.Target.CAFile)
	}
	if cfg.Schedules.HTTPCheck.Duration != 30*time.Second {
		t.Fatalf("unexpected http schedule %s", cfg.Schedules.HTTPCheck.Duration)
	}
	if cfg.Models.MaxConcurrency != 4 {
		t.Fatalf("unexpected max concurrency %d", cfg.Models.MaxConcurrency)
	}
	if cfg.Dashboard.DefaultWindow.Duration != 24*time.Hour {
		t.Fatalf("unexpected dashboard window %s", cfg.Dashboard.DefaultWindow.Duration)
	}
	if cfg.Dashboard.SLO.TTFTP99MS != 200 || cfg.Dashboard.SLO.ITLP99MS != 50 || cfg.Dashboard.SLO.RequestLatencyP99MS != 3000 {
		t.Fatalf("unexpected dashboard slo defaults: %#v", cfg.Dashboard.SLO)
	}
}

// TestExampleConfigsLoad verifies shipped config examples stay valid.
func TestExampleConfigsLoad(t *testing.T) {
	for _, path := range []string{"../../config.example.yaml", "../../examples/config.compose.yaml"} {
		if _, err := Load(path); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
	}
}
