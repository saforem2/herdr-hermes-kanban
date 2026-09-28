package kanban

import "testing"

func TestColumnsMatchHermesLifecycle(t *testing.T) {
	want := []string{"triage", "todo", "ready", "running", "blocked", "scheduled", "review", "done"}
	if len(StatusColumns) != len(want) {
		t.Fatalf("got %v", StatusColumns)
	}
	for i := range want {
		if StatusColumns[i] != want[i] {
			t.Fatalf("got %v", StatusColumns)
		}
	}
}
