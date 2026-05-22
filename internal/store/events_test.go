package store

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// TestMigrationDefinesModelEventChangedFlag verifies fresh databases can filter lifecycle changes directly.
func TestMigrationDefinesModelEventChangedFlag(t *testing.T) {
	assertContains(t, migrationSQL, "changed BOOLEAN NOT NULL DEFAULT FALSE")
	assertContains(t, migrationSQL, "CREATE INDEX IF NOT EXISTS model_events_changed_observed_idx ON model_events(observed_at DESC) WHERE changed")
}

func TestMigrationDefinesProviderMetadataColumns(t *testing.T) {
	assertContains(t, migrationSQL, "provider_metadata JSONB NOT NULL DEFAULT '{}'::jsonb")
	assertContains(t, migrationSQL, "provider_id TEXT NOT NULL")
	assertContains(t, migrationSQL, "PRIMARY KEY (provider_id, model_id)")
}

func TestMigrationDefinesTaskSpacingState(t *testing.T) {
	assertContains(t, migrationSQL, "CREATE TABLE IF NOT EXISTS task_spacing_state")
	assertContains(t, migrationSQL, "key TEXT PRIMARY KEY")
	assertContains(t, migrationSQL, "next_allowed_at TIMESTAMPTZ NOT NULL")
}

func TestSanitizeObservedModelsRedactsProviderMetadata(t *testing.T) {
	observed := []ObservedModel{{
		ID: "model-a",
		ProviderMetadata: map[string]any{
			"api_key":  "secret",
			"owned_by": "acme",
		},
	}}

	got := sanitizeObservedModels(observed)

	if got[0].ProviderMetadata["api_key"] != "[redacted]" {
		t.Fatalf("api_key = %#v, want redacted", got[0].ProviderMetadata["api_key"])
	}
	if got[0].ProviderMetadata["owned_by"] != "acme" {
		t.Fatalf("owned_by = %#v, want preserved", got[0].ProviderMetadata["owned_by"])
	}
	if observed[0].ProviderMetadata["api_key"] != "secret" {
		t.Fatalf("original metadata mutated: %#v", observed[0].ProviderMetadata)
	}
}

// TestModelEventSQLIncludesChangedColumn verifies event writes and dashboard reads use the changed flag.
func TestModelEventSQLIncludesChangedColumn(t *testing.T) {
	assertContains(t, insertModelEventSQL, "message, changed, details")
	assertContains(t, insertModelEventSQL, "RETURNING id, provider_id, model_id, event_type, capability, observed_at, changed")
	assertContains(t, recordModelEventSQL, "message, changed, details")
	assertContains(t, modelEventSelectColumns, "changed")
	assertContains(t, recentModelEventsSQL, "WHERE changed")
}

// TestModelEventWhereClauseBuildsStableFilters verifies filter ordering and parameters.
func TestModelEventWhereClauseBuildsStableFilters(t *testing.T) {
	where, args := modelEventWhereClause(ModelEventQuery{
		ProviderID: "openai",
		ModelID:    "model-a",
		Statuses:   []string{"ok", "error"},
		Sources:    []string{"scheduled_run"},
		EventTypes: []string{"capability_probe"},
	})

	wantWhere := "WHERE provider_id=$1 AND model_id=$2 AND status = ANY($3) AND source = ANY($4) AND event_type = ANY($5)"
	if where != wantWhere {
		t.Fatalf("where = %q, want %q", where, wantWhere)
	}
	if len(args) != 5 {
		t.Fatalf("args len = %d, want 5", len(args))
	}
	if args[0] != "openai" || args[1] != "model-a" {
		t.Fatalf("identity args = %#v, want openai/model-a", args[:2])
	}
}

// TestScanRecentEventsReadsChangedFlag verifies the dashboard event DTO exposes changed.
func TestScanRecentEventsReadsChangedFlag(t *testing.T) {
	observedAt := time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)
	rows := &fakeEventRows{
		rows: []fakeEventRow{{
			id:         42,
			providerID: "openai",
			modelID:    "model-a",
			eventType:  "inactive",
			source:     "inventory",
			severity:   "warning",
			status:     "inactive",
			capability: "chat",
			observedAt: observedAt,
			title:      "Model inactive",
			message:    "Model model-a disappeared from /v1/models.",
			changed:    true,
			details:    []byte(`{"missing_since":"2026-05-18T12:00:00Z"}`),
		}},
	}

	events, err := scanRecentEvents(rows)
	if err != nil {
		t.Fatalf("scanRecentEvents() error = %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("events len = %d, want 1", len(events))
	}
	event := events[0]
	if !event.Changed {
		t.Fatal("event.Changed = false, want true")
	}
	if event.ID != 42 || event.ProviderID != "openai" || event.ModelID != "model-a" || event.ModelKey == "" || event.EventType != "inactive" || !event.ObservedAt.Equal(observedAt) {
		t.Fatalf("event = %#v, want scanned lifecycle event", event)
	}
	if got := event.Details["missing_since"]; got != "2026-05-18T12:00:00Z" {
		t.Fatalf("missing_since detail = %#v, want timestamp", got)
	}
}

func assertContains(t *testing.T, haystack, needle string) {
	t.Helper()
	if !strings.Contains(haystack, needle) {
		t.Fatalf("missing %q in:\n%s", needle, haystack)
	}
}

type fakeEventRow struct {
	id         int64
	providerID string
	modelID    string
	eventType  string
	source     string
	severity   string
	status     string
	capability string
	observedAt time.Time
	title      string
	message    string
	changed    bool
	details    []byte
}

type fakeEventRows struct {
	rows   []fakeEventRow
	index  int
	closed bool
	err    error
}

func (r *fakeEventRows) Close() {
	r.closed = true
}

func (r *fakeEventRows) Err() error {
	return r.err
}

func (r *fakeEventRows) CommandTag() pgconn.CommandTag {
	return pgconn.CommandTag{}
}

func (r *fakeEventRows) FieldDescriptions() []pgconn.FieldDescription {
	return nil
}

func (r *fakeEventRows) Next() bool {
	if r.index >= len(r.rows) {
		r.Close()
		return false
	}
	r.index++
	return true
}

func (r *fakeEventRows) Scan(dest ...any) error {
	if r.index == 0 || r.index > len(r.rows) {
		return errors.New("scan called without current row")
	}
	if len(dest) != 13 {
		return errors.New("unexpected destination count")
	}
	row := r.rows[r.index-1]
	*(dest[0].(*int64)) = row.id
	*(dest[1].(*string)) = row.providerID
	*(dest[2].(*string)) = row.modelID
	*(dest[3].(*string)) = row.eventType
	*(dest[4].(*string)) = row.source
	*(dest[5].(*string)) = row.severity
	*(dest[6].(*string)) = row.status
	*(dest[7].(*string)) = row.capability
	*(dest[8].(*time.Time)) = row.observedAt
	*(dest[9].(*string)) = row.title
	*(dest[10].(*string)) = row.message
	*(dest[11].(*bool)) = row.changed
	*(dest[12].(*[]byte)) = row.details
	return nil
}

func (r *fakeEventRows) Values() ([]any, error) {
	return nil, nil
}

func (r *fakeEventRows) RawValues() [][]byte {
	return nil
}

func (r *fakeEventRows) Conn() *pgx.Conn {
	return nil
}
