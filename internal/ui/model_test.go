package ui

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/saforem2/herdr-hermes-kanban/internal/kanban"
)

type fakeService struct {
	boards                                  []kanban.Board
	tasks                                   []kanban.Task
	createdTitle, createdBody, createdBoard string
	comment                                 string
	transition                              string
	createKeys                              []string
	createCalls, commentCalls, transitCalls int
}

func (f *fakeService) Boards(context.Context) ([]kanban.Board, error)      { return f.boards, nil }
func (f *fakeService) List(context.Context, string) ([]kanban.Task, error) { return f.tasks, nil }
func (f *fakeService) Show(context.Context, string, string) (kanban.Detail, error) {
	return kanban.Detail{Task: f.tasks[0]}, nil
}
func (f *fakeService) CreateTriage(_ context.Context, b, t, body, key string) ([]byte, error) {
	f.createCalls++
	f.createKeys = append(f.createKeys, key)
	f.createdBoard = b
	f.createdTitle = t
	f.createdBody = body
	return nil, nil
}
func (f *fakeService) Comment(_ context.Context, _, _, text string) ([]byte, error) {
	f.commentCalls++
	f.comment = text
	return nil, nil
}
func (f *fakeService) Transition(_ context.Context, _, _, _, status, _ string) ([]byte, error) {
	f.transitCalls++
	f.transition = status
	return nil, nil
}

func send(m tea.Model, keys ...string) tea.Model {
	for _, key := range keys {
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
		if key == "enter" {
			msg = tea.KeyMsg{Type: tea.KeyEnter}
		}
		m, _ = m.Update(msg)
	}
	return m
}

func TestBoardModelColumnsAndSelection(t *testing.T) {
	f := &fakeService{boards: []kanban.Board{{Slug: "alpha", Current: true}}, tasks: []kanban.Task{{ID: "t1", Title: "idea", Status: "triage"}, {ID: "t2", Title: "ship", Status: "done"}}}
	m := NewBoardModel(context.Background(), f)
	next, cmd := m.Update(boardsLoadedMsg(f.boards))
	if cmd == nil {
		t.Fatal("expected task load")
	}
	m = next.(BoardModel)
	next, _ = m.Update(tasksLoadedMsg(f.tasks))
	m = next.(BoardModel)
	view := m.View()
	for _, col := range Columns {
		if !contains(view, col) {
			t.Fatalf("missing column %s", col)
		}
	}
	if !contains(view, "idea") || !contains(view, "ship") {
		t.Fatalf("missing tasks: %s", view)
	}
}

func TestCreateFlowDefaultsToTriage(t *testing.T) {
	f := &fakeService{boards: []kanban.Board{{Slug: "alpha", Current: true}}}
	var model tea.Model = NewBoardModel(context.Background(), f)
	model, _ = model.Update(boardsLoadedMsg(f.boards))
	model = send(model, "n", "new thought", "enter", "details", "enter")
	m := model.(BoardModel)
	cmd := m.pending
	if cmd == nil {
		t.Fatal("expected create command")
	}
	cmd()
	if f.createdBoard != "alpha" || f.createdTitle != "new thought" || f.createdBody != "details" {
		t.Fatalf("unexpected create: %#v", f)
	}
}

func TestQuickCaptureSubmitsUnassignedTriage(t *testing.T) {
	f := &fakeService{boards: []kanban.Board{{Slug: "alpha", Current: true}}}
	var model tea.Model = NewCaptureModel(context.Background(), f)
	model, _ = model.Update(boardsLoadedMsg(f.boards))
	model = send(model, "thought", "enter")
	m := model.(CaptureModel)
	if m.pending == nil {
		t.Fatal("expected create command")
	}
	m.pending()
	if f.createdTitle != "thought" || f.createdBoard != "alpha" {
		t.Fatalf("unexpected capture: %#v", f)
	}
}

func TestQuickCaptureRepeatedEnterWhilePendingSubmitsOnce(t *testing.T) {
	f := &fakeService{boards: []kanban.Board{{Slug: "alpha", Current: true}}}
	var model tea.Model = NewCaptureModel(context.Background(), f)
	model, _ = model.Update(boardsLoadedMsg(f.boards))
	model = send(model, "thought", "enter")
	cmd := model.(CaptureModel).pending
	model = send(model, "enter", "enter")
	cmd()
	if f.createCalls != 1 {
		t.Fatalf("create ran %d times", f.createCalls)
	}
}

func TestSafeTransitionsExcludeRunning(t *testing.T) {
	want := map[string][]string{
		"triage": nil, "todo": {"ready"},
		"ready":   {"review", "done"},
		"running": nil, "blocked": {"ready", "done"},
		"scheduled": {"ready"}, "review": {"done"}, "done": nil,
	}
	for from, targets := range want {
		if got := SafeTransitions(from); !reflect.DeepEqual(got, targets) {
			t.Errorf("%s: got %v, want %v", from, got, targets)
		}
	}
}

func TestRepeatedEnterWhilePendingDoesNotDuplicateMutations(t *testing.T) {
	tests := []struct {
		name  string
		setup func(tea.Model) tea.Model
		calls func(*fakeService) int
	}{
		{"create", func(m tea.Model) tea.Model { return send(m, "n", "one", "enter", "body", "enter") }, func(f *fakeService) int { return f.createCalls }},
		{"comment", func(m tea.Model) tea.Model { return send(m, "c", "note", "enter") }, func(f *fakeService) int { return f.commentCalls }},
		{"transition", func(m tea.Model) tea.Model { return send(m, "s", "ready", "enter") }, func(f *fakeService) int { return f.transitCalls }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &fakeService{boards: []kanban.Board{{Slug: "alpha", Current: true}}, tasks: []kanban.Task{{ID: "t1", Status: "todo"}}}
			var model tea.Model = NewBoardModel(context.Background(), f)
			model, _ = model.Update(boardsLoadedMsg(f.boards))
			model, _ = model.Update(tasksLoadedMsg(f.tasks))
			model = send(model, "right")
			model = tt.setup(model)
			first := model.(BoardModel).pending
			model = send(model, "enter", "enter")
			if model.(BoardModel).pending == nil {
				t.Fatal("pending command was cleared")
			}
			first()
			if got := tt.calls(f); got != 1 {
				t.Fatalf("mutation ran %d times", got)
			}
		})
	}
}

func TestCreateIdempotencyKeyIsStableForSubmission(t *testing.T) {
	f := &fakeService{boards: []kanban.Board{{Slug: "alpha", Current: true}}}
	var model tea.Model = NewBoardModel(context.Background(), f)
	model, _ = model.Update(boardsLoadedMsg(f.boards))
	model = send(model, "n", "one", "enter", "body", "enter")
	cmd := model.(BoardModel).pending
	cmd()
	cmd()
	if len(f.createKeys) != 2 || f.createKeys[0] == "" || f.createKeys[0] != f.createKeys[1] {
		t.Fatalf("unstable idempotency keys: %v", f.createKeys)
	}
}

func TestViewSanitizesUntrustedTerminalControls(t *testing.T) {
	evil := "safe\x1b]52;c;owned\a\x1b[31m\rBAD\x00\nINJECT	TAB"
	f := &fakeService{boards: []kanban.Board{{Slug: "alpha" + evil, Current: true}}, tasks: []kanban.Task{{ID: "t1" + evil, Title: "title" + evil, Body: "body" + evil, Status: "triage"}}}
	m := NewBoardModel(context.Background(), f)
	m.boards, m.tasks = f.boards, f.tasks
	d := kanban.Detail{Task: f.tasks[0], Comments: []kanban.Comment{{Author: "author" + evil, Body: "comment" + evil}}}
	runSummary, runError := "summary"+evil, "run-error"+evil
	d.LatestSummary = &runSummary
	d.Events = []kanban.Event{{Kind: "event" + evil, Payload: map[string]any{"evil": evil}}}
	d.Runs = []kanban.Run{{ID: 1, Profile: "profile" + evil, Status: "failed", Summary: &runSummary, Error: &runError, Metadata: map[string]any{"evil": evil}}}
	m.detail, m.err = &d, fmt.Errorf("error%s", evil)
	view := m.View()
	if strings.ContainsAny(view, "\x00\r\a") || strings.Contains(view, "\x1b]52") || strings.Contains(view, "\x1b[31m") {
		t.Fatalf("unsafe control sequence rendered: %q", view)
	}
	if strings.Contains(view, "alpha safe\n") || strings.Contains(view, "author safe\n") || strings.Contains(view, "t1 safe\n") {
		t.Fatalf("single-line field injected a newline: %q", view)
	}
}

func TestViewportKeepsSelectedColumnAndRowVisible(t *testing.T) {
	f := &fakeService{boards: []kanban.Board{{Slug: "alpha", Current: true}}}
	for i := 0; i < 20; i++ {
		f.tasks = append(f.tasks, kanban.Task{ID: fmt.Sprintf("t%d", i), Title: fmt.Sprintf("card-%d", i), Status: "done"})
	}
	m := NewBoardModel(context.Background(), f)
	m.boards, m.tasks, m.width, m.height, m.col, m.row = f.boards, f.tasks, 50, 12, 7, 19
	view := m.View()
	if !strings.Contains(view, "done (20)") || !strings.Contains(view, "card-19") || strings.Contains(view, "triage (0)") {
		t.Fatalf("viewport missed selection:\n%s", view)
	}
}

func TestDetailViewportPagesLongContent(t *testing.T) {
	m := NewBoardModel(context.Background(), &fakeService{})
	m.boards = []kanban.Board{{Slug: "alpha", Current: true}}
	m.height = 12
	d := kanban.Detail{Task: kanban.Task{Body: "line-0\nline-1\nline-2\nline-3\nline-4\nline-5"}}
	m.detail = &d
	before := m.View()
	var next tea.Model = m
	for range 5 {
		next, _ = next.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	}
	after := next.(BoardModel).View()
	if strings.Contains(after, "line-0") || !strings.Contains(after, "line-5") {
		t.Fatalf("detail did not page; before=%q after=%q", before, after)
	}
}

func TestDetailRendersProvenanceChainProgressAndAuditTimeline(t *testing.T) {
	assignee, workspace, branch, project, session := "worker", "/work/tree", "feat/provenance", "project-x", "session-1"
	summary, runSummary := "Latest verified checkpoint", "Worker reached tests"
	runID, pid := int64(7), 4321
	m := NewBoardModel(context.Background(), &fakeService{})
	m.boards = []kanban.Board{{Slug: "alpha", Current: true}}
	m.width, m.height = 160, 60
	m.detail = &kanban.Detail{
		Task:          kanban.Task{ID: "task", Title: "Current", Body: "body", Status: "running", CreatedBy: "orchestrator", Assignee: &assignee, WorkspaceKind: "worktree", WorkspacePath: &workspace, BranchName: &branch, ProjectID: &project, SessionID: &session, CurrentStepKey: strptr("verify")},
		LatestSummary: &summary,
		Parents:       []string{"parent"}, Children: []string{"child"},
		Events: []kanban.Event{{Kind: "claimed", Payload: map[string]any{"profile": "worker"}, CreatedAt: 1700000000, RunID: &runID}},
		Runs:   []kanban.Run{{ID: 7, Profile: "worker", Status: "running", Summary: &runSummary, WorkerPID: &pid, StartedAt: 1700000000}},
	}
	view := m.View()
	for _, want := range []string{"Task", "task", "Current", "running", "Assignment", "worker", "orchestrator", "session-1", "project-x", "/work/tree", "feat/provenance", "Progress", "verify", summary, "Chain", "parent → task → child", "Audit timeline", "claimed", "run #7", runSummary} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in detail:\n%s", want, view)
		}
	}
}

func TestCardsShowAssignmentAndCurrentProgress(t *testing.T) {
	assignee, step := "worker", "verify"
	m := NewBoardModel(context.Background(), &fakeService{})
	m.boards = []kanban.Board{{Slug: "alpha", Current: true}}
	m.tasks = []kanban.Task{{ID: "t1", Title: "ship", Status: "running", Assignee: &assignee, CurrentStepKey: &step}}
	m.width, m.height, m.col = 240, 20, 3
	view := m.View()
	if !strings.Contains(view, "@worker") || !strings.Contains(view, "↳ verify") {
		t.Fatalf("card lacks assignment/progress:\n%s", view)
	}
}

func strptr(s string) *string { return &s }

func TestViewFitsConfiguredViewport(t *testing.T) {
	f := &fakeService{boards: []kanban.Board{{Slug: "alpha", Current: true}}}
	for i := 0; i < 20; i++ {
		f.tasks = append(f.tasks, kanban.Task{ID: fmt.Sprintf("t%d", i), Title: strings.Repeat("x", 80), Status: "triage"})
	}
	m := NewBoardModel(context.Background(), f)
	m.boards, m.tasks, m.width, m.height = f.boards, f.tasks, 46, 12
	assertViewFits(t, m)
}

func TestSmallOverlayWithDetailFitsConfiguredViewport(t *testing.T) {
	f := &fakeService{boards: []kanban.Board{{Slug: "alpha", Current: true}}}
	for i := 0; i < 20; i++ {
		f.tasks = append(f.tasks, kanban.Task{ID: fmt.Sprintf("t%d", i), Title: fmt.Sprintf("card-%d", i), Status: "done"})
	}
	m := NewBoardModel(context.Background(), f)
	m.boards, m.tasks, m.width, m.height, m.col, m.row = f.boards, f.tasks, 46, 12, 7, 19
	m.detail = &kanban.Detail{Task: f.tasks[19]}

	view := assertViewFits(t, m)
	for _, want := range []string{"done (20)", "card-19", "Details", "t19"} {
		if !strings.Contains(view, want) {
			t.Fatalf("small overlay omitted %q:\n%s", want, view)
		}
	}
}

func TestViewFitsViewportWithInputAndError(t *testing.T) {
	for _, height := range []int{6, 8, 12, 24} {
		for _, withDetail := range []bool{false, true} {
			t.Run(fmt.Sprintf("height=%d/detail=%t", height, withDetail), func(t *testing.T) {
				m := NewBoardModel(context.Background(), &fakeService{})
				m.boards = []kanban.Board{{Slug: "alpha", Current: true}}
				m.tasks = []kanban.Task{{ID: "t1", Title: "card", Status: "triage"}}
				m.width, m.height, m.mode = 46, height, modeComment
				m.input.SetValue("input")
				m.err = fmt.Errorf("error")
				if withDetail {
					m.detail = &kanban.Detail{Task: m.tasks[0]}
				}

				view := assertViewFits(t, m)
				for _, want := range []string{"input", "error"} {
					if !strings.Contains(view, want) {
						t.Fatalf("viewport omitted %q:\n%s", want, view)
					}
				}
			})
		}
	}
}

func assertViewFits(t *testing.T, m BoardModel) string {
	t.Helper()
	view := m.View()
	for _, line := range strings.Split(view, "\n") {
		if ansi.StringWidth(line) > m.width {
			t.Fatalf("line width %d exceeds %d: %q", ansi.StringWidth(line), m.width, line)
		}
	}
	if lines := len(strings.Split(view, "\n")); lines > m.height {
		t.Fatalf("view has %d lines, exceeds height %d:\n%s", lines, m.height, view)
	}
	return view
}

func TestDetailLinesFitConfiguredWidth(t *testing.T) {
	long := strings.Repeat("x", 100)
	m := NewBoardModel(context.Background(), &fakeService{})
	m.boards = []kanban.Board{{Slug: "alpha", Current: true}}
	m.width, m.height = 40, 30
	m.detail = &kanban.Detail{Task: kanban.Task{ID: "t1", Body: long, WorkspacePath: &long}, Events: []kanban.Event{{Kind: "event", Payload: map[string]any{"value": long}}}}
	for _, line := range strings.Split(m.View(), "\n") {
		if ansi.StringWidth(line) > m.width {
			t.Fatalf("detail line width %d exceeds %d: %q", ansi.StringWidth(line), m.width, line)
		}
	}
}

func TestCreateCommandRunsExactlyOnce(t *testing.T) {
	f := &fakeService{boards: []kanban.Board{{Slug: "alpha", Current: true}}}
	var model tea.Model = NewBoardModel(context.Background(), f)
	model, _ = model.Update(boardsLoadedMsg(f.boards))
	model = send(model, "n", "one", "enter", "-", "enter")
	m := model.(BoardModel)
	if m.pending == nil {
		t.Fatal("missing create command")
	}
	msg := m.pending()
	if f.createdTitle != "one" {
		t.Fatal("create did not run")
	}
	_, cmd := m.Update(msg)
	if cmd == nil {
		t.Fatal("expected refresh after create")
	}
	if f.createdTitle != "one" {
		t.Fatal("create ran more than once")
	}
}

func contains(s, sub string) bool {
	return len(sub) == 0 || len(s) >= len(sub) && (s == sub || index(s, sub) >= 0)
}
func index(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
