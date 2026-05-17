package store

import "testing"

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
