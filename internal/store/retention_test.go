package store

import (
	"strings"
	"testing"
	"time"
)

// TestHistoryPruneStatementsCoverHistoricalTables verifies retention deletes only historical tables.
func TestHistoryPruneStatementsCoverHistoricalTables(t *testing.T) {
	statements := historyPruneStatements(time.Unix(42, 0).UTC())
	joined := make([]string, 0, len(statements))
	for _, statement := range statements {
		joined = append(joined, strings.Join(strings.Fields(statement.query), " "))
		if len(statement.args) != 1 {
			t.Fatalf("statement %q args len = %d, want 1", statement.query, len(statement.args))
		}
	}
	sql := strings.Join(joined, "\n")
	for _, expected := range []string{
		"DELETE FROM model_events WHERE observed_at < $1",
		"DELETE FROM http_checks WHERE checked_at < $1",
		"DELETE FROM auth_checks WHERE checked_at < $1",
		"DELETE FROM chat_runs WHERE started_at < $1",
		"DELETE FROM embedding_runs WHERE started_at < $1",
		"DELETE FROM email_alerts alerts WHERE alerts.sent_at < $1",
		"DELETE FROM model_snapshots WHERE observed_at < $1",
	} {
		if !strings.Contains(sql, expected) {
			t.Fatalf("prune SQL missing %q in:\n%s", expected, sql)
		}
	}
	if strings.Contains(sql, "DELETE FROM model_states") {
		t.Fatalf("prune SQL must not delete model_states:\n%s", sql)
	}
}

// TestHistoryPruneStatementsPreserveCurrentMissingAlerts verifies missing alert dedupe is protected.
func TestHistoryPruneStatementsPreserveCurrentMissingAlerts(t *testing.T) {
	statements := historyPruneStatements(time.Unix(42, 0).UTC())
	var emailSQL string
	for _, statement := range statements {
		if strings.Contains(statement.query, "DELETE FROM email_alerts") {
			emailSQL = strings.Join(strings.Fields(statement.query), " ")
			break
		}
	}
	for _, expected := range []string{
		"alerts.alert_type = 'missing'",
		"states.model_id = alerts.model_id",
		"states.status = 'missing'",
		"states.missing_since IS NOT NULL",
	} {
		if !strings.Contains(emailSQL, expected) {
			t.Fatalf("email prune SQL missing %q in:\n%s", expected, emailSQL)
		}
	}
}
