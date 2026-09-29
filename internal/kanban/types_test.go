package kanban

import (
	"encoding/json"
	"os"
	"testing"
)

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

func TestDetailDecodesRealHermesShowFixture(t *testing.T) {
	raw, err := os.ReadFile("testdata/show.json")
	if err != nil {
		t.Fatal(err)
	}
	var detail Detail
	if err := json.Unmarshal(raw, &detail); err != nil {
		t.Fatal(err)
	}
	if detail.Task.Assignee == nil || *detail.Task.Assignee != "default" || detail.Task.SessionID == nil {
		t.Fatalf("missing assignment provenance: %#v", detail.Task)
	}
	if len(detail.Parents) != 1 || len(detail.Children) != 2 || detail.LatestSummary == nil {
		t.Fatalf("missing chain or summary: %#v", detail)
	}
	if len(detail.Events) != 2 || detail.Events[1].RunID == nil || detail.Events[1].Payload["lock"] != "mbph:73037" {
		t.Fatalf("missing typed event: %#v", detail.Events)
	}
	if len(detail.Runs) != 1 || detail.Runs[0].Profile != "default" || detail.Runs[0].WorkerPID == nil || detail.Runs[0].Metadata["model"] != "gpt-5" {
		t.Fatalf("missing typed run: %#v", detail.Runs)
	}
}
