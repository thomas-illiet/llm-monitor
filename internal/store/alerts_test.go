package store

import (
	"strings"
	"testing"
)

// TestEmailAlertDedupeSQLOnlyCountsSuccessfulDeliveries verifies failed attempts remain retryable.
func TestEmailAlertDedupeSQLOnlyCountsSuccessfulDeliveries(t *testing.T) {
	if !strings.Contains(emailAlertExistsSQL, "error=''") {
		t.Fatalf("dedupe SQL must only match successful alert rows:\n%s", emailAlertExistsSQL)
	}
}

// TestRecordEmailAlertSQLAllowsRetryAfterFailure verifies successful retries can replace failed attempts.
func TestRecordEmailAlertSQLAllowsRetryAfterFailure(t *testing.T) {
	sql := strings.Join(strings.Fields(recordEmailAlertSQL), " ")
	for _, expected := range []string{
		"ON CONFLICT(alert_key) DO UPDATE SET",
		"error=EXCLUDED.error",
		"WHERE email_alerts.error <> ''",
	} {
		if !strings.Contains(sql, expected) {
			t.Fatalf("alert upsert SQL missing %q in:\n%s", expected, sql)
		}
	}
}
