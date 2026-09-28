package ui

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/saforem2/herdr-hermes-kanban/internal/kanban"
)

type boardsLoadedMsg []kanban.Board
type tasksLoadedMsg []kanban.Task
type detailLoadedMsg kanban.Detail
type failureMsg error
type changedMsg struct{}

type mode int

const (
	modeBoard mode = iota
	modeCreateTitle
	modeCreateBody
	modeComment
	modeTransition
)

var (
	activeStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	headerStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))
	dimStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("242"))
	errorStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
)

type BoardModel struct {
	ctx           context.Context
	service       Service
	boards        []kanban.Board
	board         int
	tasks         []kanban.Task
	col, row      int
	width, height int
	mode          mode
	input         textinput.Model
	draftTitle    string
	detail        *kanban.Detail
	err           error
	status        string
	pending       tea.Cmd
}

func NewBoardModel(ctx context.Context, s Service) BoardModel {
	in := textinput.New()
	in.Prompt = "> "
	in.CharLimit = 4096
	return BoardModel{ctx: ctx, service: s, input: in}
}
func (m BoardModel) Init() tea.Cmd { return m.loadBoards() }
func (m BoardModel) loadBoards() tea.Cmd {
	return func() tea.Msg {
		b, e := m.service.Boards(m.ctx)
		if e != nil {
			return failureMsg(e)
		}
		return boardsLoadedMsg(b)
	}
}
func (m BoardModel) loadTasks() tea.Cmd {
	board := m.boardSlug()
	return func() tea.Msg {
		v, e := m.service.List(m.ctx, board)
		if e != nil {
			return failureMsg(e)
		}
		return tasksLoadedMsg(v)
	}
}
func (m BoardModel) loadDetail() tea.Cmd {
	t, ok := m.selected()
	if !ok {
		return nil
	}
	board := m.boardSlug()
	return func() tea.Msg {
		v, e := m.service.Show(m.ctx, board, t.ID)
		if e != nil {
			return failureMsg(e)
		}
		return detailLoadedMsg(v)
	}
}
func (m BoardModel) boardSlug() string {
	if m.board >= 0 && m.board < len(m.boards) {
		return m.boards[m.board].Slug
	}
	return ""
}
func (m BoardModel) columnTasks() []kanban.Task {
	var out []kanban.Task
	for _, t := range m.tasks {
		if t.Status == Columns[m.col] {
			out = append(out, t)
		}
	}
	return out
}
func (m BoardModel) selected() (kanban.Task, bool) {
	v := m.columnTasks()
	if len(v) == 0 {
		return kanban.Task{}, false
	}
	i := m.row
	if i >= len(v) {
		i = len(v) - 1
	}
	return v[i], true
}
func (m BoardModel) clamp() {
	n := len(m.columnTasks())
	if n == 0 {
		m.row = 0
	} else if m.row >= n {
		m.row = n - 1
	}
}

func (m BoardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch x := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = x.Width
		m.height = x.Height
	case boardsLoadedMsg:
		m.boards = []kanban.Board(x)
		for i, b := range m.boards {
			if b.Current {
				m.board = i
			}
		}
		return m, m.loadTasks()
	case tasksLoadedMsg:
		m.tasks = []kanban.Task(x)
		m.clamp()
		m.status = fmt.Sprintf("loaded %d tasks", len(m.tasks))
		m.err = nil
		m.pending = nil
	case detailLoadedMsg:
		d := kanban.Detail(x)
		m.detail = &d
		m.err = nil
	case failureMsg:
		m.err = error(x)
		m.pending = nil
	case changedMsg:
		m.mode = modeBoard
		m.input.SetValue("")
		m.pending = nil
		return m, m.loadTasks()
	case tea.KeyMsg:
		if m.mode != modeBoard {
			return m.updateInput(x)
		}
		switch x.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "r":
			return m, m.loadTasks()
		case "left", "h":
			if m.col > 0 {
				m.col--
				m.row = 0
				m.detail = nil
			}
		case "right", "l":
			if m.col < len(Columns)-1 {
				m.col++
				m.row = 0
				m.detail = nil
			}
		case "up", "k":
			if m.row > 0 {
				m.row--
			}
			m.detail = nil
		case "down", "j":
			if m.row+1 < len(m.columnTasks()) {
				m.row++
			}
			m.detail = nil
		case "[":
			if len(m.boards) > 0 {
				m.board = (m.board - 1 + len(m.boards)) % len(m.boards)
				m.row = 0
				m.detail = nil
				return m, m.loadTasks()
			}
		case "]":
			if len(m.boards) > 0 {
				m.board = (m.board + 1) % len(m.boards)
				m.row = 0
				m.detail = nil
				return m, m.loadTasks()
			}
		case "enter":
			return m, m.loadDetail()
		case "n":
			m.mode = modeCreateTitle
			m.input.Placeholder = "task title"
			m.input.Focus()
		case "c":
			if _, ok := m.selected(); ok {
				m.mode = modeComment
				m.input.Placeholder = "comment"
				m.input.Focus()
			}
		case "s":
			if _, ok := m.selected(); ok {
				m.mode = modeTransition
				m.input.Placeholder = "status: " + strings.Join(SafeTransitions(m.columnTasks()[m.row].Status), ", ")
				m.input.Focus()
			}
		}
	}
	return m, nil
}

func (m BoardModel) updateInput(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.Type == tea.KeyEsc {
		m.mode = modeBoard
		m.input.SetValue("")
		return m, nil
	}
	if key.Type == tea.KeyEnter {
		value := strings.TrimSpace(m.input.Value())
		if value == "" {
			return m, nil
		}
		switch m.mode {
		case modeCreateTitle:
			m.draftTitle = value
			m.mode = modeCreateBody
			m.input.SetValue("")
			m.input.Placeholder = "details (required by enter, may be '-')"
			return m, nil
		case modeCreateBody:
			title, body, board := m.draftTitle, value, m.boardSlug()
			if body == "-" {
				body = ""
			}
			m.pending = func() tea.Msg {
				_, e := m.service.CreateTriage(m.ctx, board, title, body)
				if e != nil {
					return failureMsg(e)
				}
				return changedMsg{}
			}
			return m, m.pending
		case modeComment:
			t, _ := m.selected()
			board := m.boardSlug()
			m.pending = func() tea.Msg {
				_, e := m.service.Comment(m.ctx, board, t.ID, value)
				if e != nil {
					return failureMsg(e)
				}
				return changedMsg{}
			}
			return m, m.pending
		case modeTransition:
			t, _ := m.selected()
			allowed := SafeTransitions(t.Status)
			if !has(allowed, value) {
				m.err = fmt.Errorf("allowed: %s", strings.Join(allowed, ", "))
				return m, nil
			}
			board := m.boardSlug()
			m.pending = func() tea.Msg {
				_, e := m.service.Transition(m.ctx, board, t.ID, t.Status, value, "changed in Herdr Kanban")
				if e != nil {
					return failureMsg(e)
				}
				return changedMsg{}
			}
			return m, m.pending
		}
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(key)
	return m, cmd
}
func has(v []string, s string) bool {
	for _, x := range v {
		if x == s {
			return true
		}
	}
	return false
}
func (m BoardModel) View() string {
	if len(m.boards) == 0 {
		return "Hermes Kanban\n\nLoading boards…"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s  board: %s  %s\n\n", headerStyle.Render("Hermes Kanban"), m.boardSlug(), dimStyle.Render("[ / ] switch"))
	columnWidth := 22
	if m.width > 0 && m.width/len(Columns) > columnWidth {
		columnWidth = m.width / len(Columns)
	}
	panels := make([]string, 0, len(Columns))
	for ci, col := range Columns {
		style := headerStyle
		if ci == m.col {
			style = activeStyle
		}
		cards := []string{style.Render(fmt.Sprintf("%s (%d)", col, countStatus(m.tasks, col)))}
		for _, t := range m.tasks {
			if t.Status == col {
				mark := "  "
				if ci == m.col {
					selected, _ := m.selected()
					if selected.ID == t.ID {
						mark = "> "
					}
				}
				cards = append(cards, mark+lipgloss.NewStyle().MaxWidth(columnWidth-2).Render(t.Title), dimStyle.Render("  "+t.ID))
			}
		}
		if len(cards) == 1 {
			cards = append(cards, dimStyle.Render("  —"))
		}
		panels = append(panels, lipgloss.NewStyle().Width(columnWidth).PaddingRight(1).Render(strings.Join(cards, "\n")))
	}
	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, panels...))
	b.WriteString("\n\n")
	if m.detail != nil {
		fmt.Fprintf(&b, "%s\n%s\n", headerStyle.Render("Details"), m.detail.Task.Body)
		for _, c := range m.detail.Comments {
			fmt.Fprintf(&b, "  %s: %s\n", c.Author, c.Body)
		}
	}
	if m.mode != modeBoard {
		fmt.Fprintf(&b, "\n%s\n", m.input.View())
	}
	if m.err != nil {
		fmt.Fprintf(&b, "\n%s\n", errorStyle.Render(m.err.Error()))
	}
	b.WriteString(dimStyle.Render("h/l columns  j/k cards  enter details  n new triage  c comment  s status  r refresh  q quit"))
	return b.String()
}

func countStatus(tasks []kanban.Task, status string) int {
	n := 0
	for _, task := range tasks {
		if task.Status == status {
			n++
		}
	}
	return n
}
