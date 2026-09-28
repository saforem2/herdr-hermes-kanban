package ui

import (
	"context"
	"crypto/rand"
	"fmt"
	"strings"
	"unicode"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
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
	createKey     string
	detail        *kanban.Detail
	detailOffset  int
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
		m.detailOffset = 0
		m.err = nil
	case failureMsg:
		m.err = error(x)
		m.pending = nil
	case changedMsg:
		m.mode = modeBoard
		m.input.SetValue("")
		m.pending = nil
		m.createKey = ""
		return m, m.loadTasks()
	case tea.KeyMsg:
		if m.pending != nil {
			return m, nil
		}
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
		case "pgup":
			m.detailOffset = max(0, m.detailOffset-max(1, m.height/3))
		case "pgdown":
			m.detailOffset += max(1, m.height/3)
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
			m.createKey = ""
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
			if t, ok := m.selected(); ok && len(SafeTransitions(t.Status)) > 0 {
				m.mode = modeTransition
				m.input.Placeholder = "status: " + strings.Join(SafeTransitions(t.Status), ", ")
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
		m.createKey = ""
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
			if m.createKey == "" {
				m.createKey = newIdempotencyKey()
			}
			key := m.createKey
			if body == "-" {
				body = ""
			}
			m.pending = func() tea.Msg {
				_, e := m.service.CreateTriage(m.ctx, board, title, body, key)
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
	m.input.SetValue(sanitize(m.input.Value()))
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
	header := fmt.Sprintf("%s  board: %s  %s", headerStyle.Render("Hermes Kanban"), sanitizeLine(m.boardSlug()), dimStyle.Render("[ / ] switch"))
	b.WriteString(ansi.Truncate(header, max(1, m.width), "…") + "\n\n")
	columnWidth := 22
	if m.width > 0 && m.width/len(Columns) > columnWidth+1 {
		columnWidth = m.width/len(Columns) - 1
	}
	firstCol, lastCol := visibleColumns(m.col, m.width, columnWidth)
	panels := make([]string, 0, lastCol-firstCol)
	for ci := firstCol; ci < lastCol; ci++ {
		col := Columns[ci]
		style := headerStyle
		if ci == m.col {
			style = activeStyle
		}
		cards := []string{style.Render(fmt.Sprintf("%s (%d)", col, countStatus(m.tasks, col)))}
		visibleRows := max(1, (m.height-5)/3)
		startRow := 0
		if ci == m.col {
			startRow = max(0, m.row-visibleRows+1)
		}
		row := 0
		for _, t := range m.tasks {
			if t.Status == col {
				if row < startRow || row >= startRow+visibleRows {
					row++
					continue
				}
				mark := "  "
				if ci == m.col {
					selected, _ := m.selected()
					if selected.ID == t.ID {
						mark = "> "
					}
				}
				cards = append(cards, mark+lipgloss.NewStyle().MaxWidth(columnWidth-2).Render(sanitizeLine(t.Title)), dimStyle.Render("  "+sanitizeLine(t.ID)))
				row++
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
		detailLines := []string{sanitize(m.detail.Task.Body)}
		for _, c := range m.detail.Comments {
			detailLines = append(detailLines, "  "+sanitize(c.Author)+": "+sanitize(c.Body))
		}
		detailLines = strings.Split(strings.Join(detailLines, "\n"), "\n")
		page := max(1, m.height/3)
		maxOffset := max(0, len(detailLines)-page)
		offset := min(m.detailOffset, maxOffset)
		fmt.Fprintf(&b, "%s\n%s\n", headerStyle.Render("Details"), strings.Join(detailLines[offset:min(len(detailLines), offset+page)], "\n"))
	}
	if m.mode != modeBoard {
		fmt.Fprintf(&b, "\n%s\n", m.input.View())
	}
	if m.err != nil {
		fmt.Fprintf(&b, "\n%s\n", errorStyle.Render(sanitize(m.err.Error())))
	}
	help := "h/l columns  j/k cards  enter details  pgup/pgdown details  n new triage  c comment  s status  r refresh  q quit"
	b.WriteString(dimStyle.Render(ansi.Truncate(help, max(1, m.width), "…")))
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

func visibleColumns(selected, width, columnWidth int) (int, int) {
	count := len(Columns)
	if width > 0 {
		count = max(1, min(count, width/(columnWidth+1)))
	}
	start := max(0, min(selected-count/2, len(Columns)-count))
	return start, start + count
}

func sanitize(s string) string {
	s = ansi.Strip(s)
	return strings.Map(func(r rune) rune {
		if r == '\n' || r == '	' || !unicode.IsControl(r) {
			return r
		}
		return -1
	}, s)
}

func sanitizeLine(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '\n' || r == '\t' {
			return ' '
		}
		return r
	}, sanitize(s))
}

func newIdempotencyKey() string {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		panic("crypto/rand unavailable: " + err.Error())
	}
	return fmt.Sprintf("herdr-kanban-%x", raw[:])
}
