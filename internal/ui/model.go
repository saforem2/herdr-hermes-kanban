package ui

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
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
		return fitView([]string{"Hermes Kanban", "", "Loading boards…"}, m.width, m.height)
	}

	help := "h/l columns  j/k cards  enter details  pgup/pgdown details  n new triage  c comment  s status  r refresh  q quit"
	footer := []string{}
	if m.mode != modeBoard {
		footer = append(footer, m.input.View())
	}
	if m.err != nil {
		footer = append(footer, errorStyle.Render(sanitizeLine(m.err.Error())))
	}
	footer = append(footer, dimStyle.Render(help))

	height := m.height
	if height <= 0 {
		height = 1 << 20
	}
	available := max(0, height-len(footer))
	lines := make([]string, 0, min(height, 64))
	if available > 0 {
		header := fmt.Sprintf("%s  board: %s  %s", headerStyle.Render("Hermes Kanban"), sanitizeLine(m.boardSlug()), dimStyle.Render("[ / ] switch"))
		lines = append(lines, header)
		available--
	}

	boardBudget, detailBudget := available, 0
	if m.detail != nil && available >= 4 {
		detailBudget = min(available-2, max(2, m.height/3+1))
		boardBudget = available - detailBudget
	}
	if boardBudget > 0 {
		lines = append(lines, m.renderBoard(boardBudget)...)
	}
	if detailBudget > 0 {
		lines = append(lines, m.renderDetailViewport(detailBudget)...)
	}
	lines = append(lines, footer...)
	return fitView(lines, m.width, m.height)
}

func (m BoardModel) renderBoard(lineBudget int) []string {
	columnWidth := 22
	if m.width > 0 && m.width/len(Columns) > columnWidth+1 {
		columnWidth = m.width/len(Columns) - 1
	}
	firstCol, lastCol := visibleColumns(m.col, m.width, columnWidth)
	panels := make([]string, 0, lastCol-firstCol)
	visibleRows := max(1, (lineBudget-1)/2)
	for ci := firstCol; ci < lastCol; ci++ {
		col := Columns[ci]
		style := headerStyle
		if ci == m.col {
			style = activeStyle
		}
		cards := []string{style.Render(fmt.Sprintf("%s (%d)", col, countStatus(m.tasks, col)))}
		startRow := 0
		if ci == m.col {
			startRow = max(0, m.row-visibleRows+1)
		}
		row := 0
		for _, t := range m.tasks {
			if t.Status != col {
				continue
			}
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
			meta := "  " + sanitizeLine(t.ID)
			if t.Assignee != nil && *t.Assignee != "" {
				meta = "  @" + sanitizeLine(*t.Assignee)
			}
			if t.CurrentStepKey != nil && *t.CurrentStepKey != "" {
				meta += " ↳ " + sanitizeLine(*t.CurrentStepKey)
			}
			cards = append(cards, mark+lipgloss.NewStyle().MaxWidth(columnWidth-2).Render(sanitizeLine(t.Title)), dimStyle.Render(ansi.Truncate(meta, columnWidth-2, "…")))
			row++
		}
		if len(cards) == 1 {
			cards = append(cards, dimStyle.Render("  —"))
		}
		cards = cards[:min(len(cards), lineBudget)]
		panels = append(panels, lipgloss.NewStyle().Width(columnWidth).PaddingRight(1).Render(strings.Join(cards, "\n")))
	}
	return strings.Split(lipgloss.JoinHorizontal(lipgloss.Top, panels...), "\n")
}

func (m BoardModel) renderDetailViewport(lineBudget int) []string {
	detailLines := renderDetail(*m.detail)
	page := max(1, lineBudget-1)
	maxOffset := max(0, len(detailLines)-page)
	offset := min(m.detailOffset, maxOffset)
	lines := []string{headerStyle.Render("Details")}
	return append(lines, detailLines[offset:min(len(detailLines), offset+page)]...)
}

func fitView(lines []string, width, height int) string {
	if height > 0 && len(lines) > height {
		lines = lines[:height]
	}
	if width > 0 {
		for i := range lines {
			lines[i] = ansi.Truncate(lines[i], width, "…")
		}
	}
	return strings.Join(lines, "\n")
}

func renderDetail(d kanban.Detail) []string {
	t := d.Task
	assignee := "unassigned"
	if t.Assignee != nil && *t.Assignee != "" {
		assignee = *t.Assignee
	}
	lines := []string{
		headerStyle.Render("Task"),
		fmt.Sprintf("  %s  %s  [%s]", sanitizeLine(orDash(t.ID)), sanitizeLine(orDash(t.Title)), sanitizeLine(orDash(t.Status))),
		headerStyle.Render("Assignment"),
		fmt.Sprintf("  assignee: %s  created by: %s", sanitizeLine(assignee), sanitizeLine(orDash(t.CreatedBy))),
		fmt.Sprintf("  session: %s  project: %s", ptrText(t.SessionID), ptrText(t.ProjectID)),
		fmt.Sprintf("  workspace: %s%s  branch: %s", sanitizeLine(orDash(t.WorkspaceKind)), optionalAt(t.WorkspacePath), ptrText(t.BranchName)),
		headerStyle.Render("Progress"),
		fmt.Sprintf("  status: %s  step: %s", sanitizeLine(orDash(t.Status)), ptrText(t.CurrentStepKey)),
	}
	if t.StartedAt != nil {
		lines = append(lines, "  started: "+formatTime(*t.StartedAt))
	}
	if t.CompletedAt != nil {
		lines = append(lines, "  completed: "+formatTime(*t.CompletedAt))
	}
	if t.Result != nil {
		lines = append(lines, "  result: "+sanitize(*t.Result))
	}
	if t.LastFailureError != nil {
		lines = append(lines, "  failure: "+sanitize(*t.LastFailureError))
	}
	if d.LatestSummary != nil {
		lines = append(lines, "  latest: "+sanitize(*d.LatestSummary))
	}
	lines = append(lines, headerStyle.Render("Chain"), "  "+renderChain(d.Parents, t.ID, d.Children))
	if t.Body != "" {
		lines = append(lines, headerStyle.Render("Body"), sanitize(t.Body))
	}
	if len(d.Comments) > 0 {
		lines = append(lines, headerStyle.Render(fmt.Sprintf("Comments (%d)", len(d.Comments))))
		for _, c := range d.Comments {
			lines = append(lines, fmt.Sprintf("  [%s] %s: %s", formatTime(c.CreatedAt), sanitizeLine(c.Author), sanitize(c.Body)))
		}
	}
	lines = append(lines, headerStyle.Render("Audit timeline"))
	type audit struct {
		at   int64
		text string
	}
	items := make([]audit, 0, len(d.Events)+len(d.Runs))
	for _, e := range d.Events {
		run := ""
		if e.RunID != nil {
			run = fmt.Sprintf(" [run #%d]", *e.RunID)
		}
		items = append(items, audit{e.CreatedAt, fmt.Sprintf("  [%s]%s event %s%s", formatTime(e.CreatedAt), run, sanitizeLine(e.Kind), renderMap(e.Payload))})
	}
	for _, r := range d.Runs {
		text := fmt.Sprintf("  [%s] run #%d %s %s", formatTime(r.StartedAt), r.ID, sanitizeLine(r.Profile), sanitizeLine(r.Status))
		if r.StepKey != nil {
			text += " step=" + sanitizeLine(*r.StepKey)
		}
		if r.Outcome != nil {
			text += " → " + sanitizeLine(*r.Outcome)
		}
		if r.EndedAt != nil {
			text += " ended=" + formatTime(*r.EndedAt)
		}
		if r.WorkerPID != nil {
			text += fmt.Sprintf(" pid=%d", *r.WorkerPID)
		}
		if r.Summary != nil {
			text += ": " + sanitize(*r.Summary)
		}
		if r.Error != nil {
			text += " error=" + sanitize(*r.Error)
		}
		text += renderMap(r.Metadata)
		items = append(items, audit{r.StartedAt, text})
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].at < items[j].at })
	for _, item := range items {
		lines = append(lines, item.text)
	}
	return strings.Split(strings.Join(lines, "\n"), "\n")
}

func ptrText(v *string) string {
	if v == nil || *v == "" {
		return "-"
	}
	return sanitizeLine(*v)
}
func orDash(v string) string {
	if v == "" {
		return "-"
	}
	return v
}
func optionalAt(v *string) string {
	if v == nil || *v == "" {
		return ""
	}
	return " @ " + sanitizeLine(*v)
}
func formatTime(v int64) string {
	if v == 0 {
		return "-"
	}
	return time.Unix(v, 0).Local().Format("2006-01-02 15:04:05")
}
func renderChain(parents []string, task string, children []string) string {
	left, right := "∅", "∅"
	if len(parents) > 0 {
		left = strings.Join(parents, ", ")
	}
	if len(children) > 0 {
		right = strings.Join(children, ", ")
	}
	return sanitizeLine(left) + " → " + sanitizeLine(orDash(task)) + " → " + sanitizeLine(right)
}
func renderMap(v map[string]any) string {
	if len(v) == 0 {
		return ""
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return " " + sanitizeLine(string(raw))
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
