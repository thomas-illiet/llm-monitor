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

// TestModelEventSQLIncludesChangedColumn verifies event writes and dashboard reads use the changed flag.
func TestModelEventSQLIncludesChangedColumn(t *testing.T) {
	assertContains(t, insertModelEventSQL, "message, changed, details")
	assertContains(t, insertModelEventSQL, "RETURNING id, model_id, event_type, capability, observed_at, changed")
	assertContains(t, recordModelEventSQL, "message, changed, details")
	assertContains(t, modelEventSelectColumns, "changed")
	assertContains(t, recentModelEventsSQL, "WHERE changed")
}

// TestModelEventWhereClauseBuildsStableFilters verifies filter ordering and parameters.
func TestModelEventWhereClauseBuildsStableFilters(t *testing.T) {
	where, args := modelEventWhereClause(ModelEventQuery{
		ModelID:    "model-a",
		Statuses:   []string{"ok", "error"},
		Sources:    []string{"scheduled_run"},
		EventTypes: []string{"capability_probe"},
	})

	wantWhere := "WHERE model_id=$1 AND status = ANY($2) AND source = ANY($3) AND event_type = ANY($4)"
	if where != wantWhere {
		t.Fatalf("where = %q, want %q", where, wantWhere)
	}
	if len(args) != 4 {
		t.Fatalf("args len = %d, want 4", len(args))
	}
	if args[0] != "model-a" {
		t.Fatalf("model arg = %#v, want model-a", args[0])
	}
}

// TestScanRecentEventsReadsChangedFlag verifies the dashboard event DTO exposes changed.
func TestScanRecentEventsReadsChangedFlag(t *testing.T) {
	observedAt := time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)
	rows := &fakeEventRows{
		rows: []fakeEventRow{{
			id:         42,
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
	if event.ID != 42 || event.ModelID != "model-a" || event.EventType != "inactive" || !event.ObservedAt.Equal(observedAt) {
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
	if len(dest) != 12 {
		return errors.New("unexpected destination count")
	}
	row := r.rows[r.index-1]
	*(dest[0].(*int64)) = row.id
	*(dest[1].(*string)) = row.modelID
	*(dest[2].(*string)) = row.eventType
	*(dest[3].(*string)) = row.source
	*(dest[4].(*string)) = row.severity
	*(dest[5].(*string)) = row.status
	*(dest[6].(*string)) = row.capability
	*(dest[7].(*time.Time)) = row.observedAt
	*(dest[8].(*string)) = row.title
	*(dest[9].(*string)) = row.message
	*(dest[10].(*bool)) = row.changed
	*(dest[11].(*[]byte)) = row.details
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
