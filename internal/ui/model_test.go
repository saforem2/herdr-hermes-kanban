package ui

import (
	"context"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/saforem2/herdr-hermes-kanban/internal/kanban"
)

type fakeService struct {
	boards                                  []kanban.Board
	tasks                                   []kanban.Task
	createdTitle, createdBody, createdBoard string
	comment                                 string
	transition                              string
}

func (f *fakeService) Boards(context.Context) ([]kanban.Board, error)      { return f.boards, nil }
func (f *fakeService) List(context.Context, string) ([]kanban.Task, error) { return f.tasks, nil }
func (f *fakeService) Show(context.Context, string, string) (kanban.Detail, error) {
	return kanban.Detail{Task: f.tasks[0]}, nil
}
func (f *fakeService) CreateTriage(_ context.Context, b, t, body string) ([]byte, error) {
	f.createdBoard = b
	f.createdTitle = t
	f.createdBody = body
	return nil, nil
}
func (f *fakeService) Comment(_ context.Context, _, _, text string) ([]byte, error) {
	f.comment = text
	return nil, nil
}
func (f *fakeService) Transition(_ context.Context, _, _, _, status, _ string) ([]byte, error) {
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

func TestSafeTransitionsExcludeRunning(t *testing.T) {
	for _, status := range SafeTransitions("todo") {
		if status == "running" {
			t.Fatal("running must never be manually selected")
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
