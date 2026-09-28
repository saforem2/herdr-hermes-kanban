package kanban

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	args := helperArgs(os.Args)
	mode := os.Getenv("HELPER_MODE")
	switch mode {
	case "capture":
		_, _ = os.Stdout.WriteString(strings.Join(args, "\x1f"))
	case "list":
		_, _ = os.Stdout.WriteString(`[{"id":"t_1","title":"one","body":"body","assignee":null,"status":"triage","priority":2,"tenant":null,"workspace_kind":"scratch","workspace_path":null,"branch_name":null,"project_id":null,"created_by":"user","created_at":123,"started_at":null,"completed_at":null,"result":null,"skills":[],"max_runtime_seconds":null,"max_retries":null,"model_override":null,"provider_override":null,"session_id":null,"workflow_template_id":null,"current_step_key":null,"completion_contract":"local-only","last_failure_error":null}]`)
	case "boards":
		_, _ = os.Stdout.WriteString(`[{"slug":"alpha","name":"Alpha","description":"work","icon":"","color":"","default_workdir":null,"project_id":null,"created_at":null,"archived":false,"db_path":"/hidden","is_current":true,"counts":{"triage":1},"total":1}]`)
	case "show":
		_, _ = os.Stdout.WriteString(`{"task":{"id":"t_1","title":"one","status":"triage"},"comments":[{"author":"sam","body":"note","created_at":124}],"events":[],"parents":[],"children":[],"runs":[]}`)
	case "large":
		_, _ = os.Stdout.WriteString(strings.Repeat("x", 4096))
	case "fail":
		_, _ = os.Stderr.WriteString("specific failure")
		os.Exit(7)
	case "sleep":
		time.Sleep(time.Second)
	}
	os.Exit(0)
}

func helperArgs(args []string) []string {
	for i, arg := range args {
		if arg == "--" {
			return args[i+1:]
		}
	}
	return nil
}

func fakeClient(t *testing.T, mode string) *Client {
	t.Helper()
	return &Client{
		Command:   []string{os.Args[0], "-test.run=TestHelperProcess", "--"},
		Timeout:   2 * time.Second,
		MaxOutput: 1024,
		Env:       []string{"GO_WANT_HELPER_PROCESS=1", "HELPER_MODE=" + mode},
	}
}

func TestListUsesBoardAndParsesTasks(t *testing.T) {
	client := fakeClient(t, "list")
	tasks, err := client.List(context.Background(), "alpha")
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 1 || tasks[0].ID != "t_1" || tasks[0].Assignee != nil {
		t.Fatalf("unexpected tasks: %#v", tasks)
	}
}

func TestCreateTriageIsUnassignedAndUsesArgv(t *testing.T) {
	client := fakeClient(t, "capture")
	out, err := client.CreateTriage(context.Background(), "alpha", "title; touch /tmp/nope", "body")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"kanban", "--board", "alpha", "create", "title; touch /tmp/nope", "--triage", "--body", "body", "--json"}
	if !reflect.DeepEqual(strings.Split(string(out), "\x1f"), want) {
		t.Fatalf("argv mismatch: %q", out)
	}
}

func TestBoardsParsesJSON(t *testing.T) {
	boards, err := fakeClient(t, "boards").Boards(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(boards) != 1 || boards[0].Slug != "alpha" || !boards[0].Current {
		t.Fatalf("unexpected boards: %#v", boards)
	}
}

func TestShowAcceptsNestedTaskShape(t *testing.T) {
	detail, err := fakeClient(t, "show").Show(context.Background(), "alpha", "t_1")
	if err != nil {
		t.Fatal(err)
	}
	if detail.Task.ID != "t_1" || len(detail.Comments) != 1 || detail.Comments[0].Body != "note" {
		t.Fatalf("unexpected detail: %#v", detail)
	}
}

func TestCommentUsesLiteralArguments(t *testing.T) {
	out, err := fakeClient(t, "capture").Comment(context.Background(), "alpha", "t_1", "$(bad)")
	if err != nil {
		t.Fatal(err)
	}
	want := "kanban\x1f--board\x1falpha\x1fcomment\x1ft_1\x1f$(bad)\x1f--author\x1fherdr-kanban"
	if string(out) != want {
		t.Fatalf("got %q", out)
	}
}

func TestTransitionAllowlistAndArguments(t *testing.T) {
	client := fakeClient(t, "capture")
	out, err := client.Transition(context.Background(), "alpha", "t_1", "todo", "ready", "reviewed")
	if err != nil {
		t.Fatal(err)
	}
	want := "kanban\x1f--board\x1falpha\x1fpromote\x1ft_1\x1freviewed\x1f--json"
	if string(out) != want {
		t.Fatalf("got %q", out)
	}
	_, err = client.Transition(context.Background(), "alpha", "t_1", "todo", "running", "")
	if !errors.Is(err, ErrUnsafeTransition) {
		t.Fatalf("expected unsafe transition, got %v", err)
	}
}

func TestTransitionUnblocksBlockedTask(t *testing.T) {
	out, err := fakeClient(t, "capture").Transition(context.Background(), "alpha", "t_1", "blocked", "ready", "fixed")
	if err != nil {
		t.Fatal(err)
	}
	want := "kanban\x1f--board\x1falpha\x1funblock\x1ft_1\x1f--reason\x1ffixed"
	if string(out) != want {
		t.Fatalf("got %q", out)
	}
}

func TestRunReportsExitAndStderr(t *testing.T) {
	_, err := fakeClient(t, "fail").run(context.Background(), "kanban", "list", "--json")
	var commandErr *CommandError
	if !errors.As(err, &commandErr) || commandErr.ExitCode != 7 || !strings.Contains(err.Error(), "specific failure") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunBoundsOutput(t *testing.T) {
	_, err := fakeClient(t, "large").run(context.Background(), "kanban", "list", "--json")
	if !errors.Is(err, ErrOutputLimit) {
		t.Fatalf("expected output limit, got %v", err)
	}
}

func TestRunTimesOut(t *testing.T) {
	client := fakeClient(t, "sleep")
	client.Timeout = 20 * time.Millisecond
	_, err := client.run(context.Background(), "kanban", "list", "--json")
	if !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, exec.ErrWaitDelay) {
		t.Fatalf("expected timeout, got %v", err)
	}
}
