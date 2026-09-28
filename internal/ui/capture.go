package ui

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/saforem2/herdr-hermes-kanban/internal/kanban"
)

type CaptureModel struct {
	ctx     context.Context
	service Service
	boards  []kanban.Board
	board   int
	input   textinput.Model
	err     error
	pending tea.Cmd
	done    bool
}

func NewCaptureModel(ctx context.Context, s Service) CaptureModel {
	in := textinput.New()
	in.Prompt = "> "
	in.Placeholder = "Capture a thought"
	in.CharLimit = 512
	in.Focus()
	return CaptureModel{ctx: ctx, service: s, input: in}
}
func (m CaptureModel) Init() tea.Cmd {
	return func() tea.Msg {
		v, e := m.service.Boards(m.ctx)
		if e != nil {
			return failureMsg(e)
		}
		return boardsLoadedMsg(v)
	}
}
func (m CaptureModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch x := msg.(type) {
	case boardsLoadedMsg:
		m.boards = []kanban.Board(x)
		for i, b := range m.boards {
			if b.Current {
				m.board = i
			}
		}
	case failureMsg:
		m.err = error(x)
		m.pending = nil
	case changedMsg:
		m.done = true
		return m, tea.Quit
	case tea.KeyMsg:
		switch x.String() {
		case "esc", "ctrl+c":
			return m, tea.Quit
		case "tab":
			if len(m.boards) > 0 {
				m.board = (m.board + 1) % len(m.boards)
			}
			return m, nil
		case "shift+tab":
			if len(m.boards) > 0 {
				m.board = (m.board - 1 + len(m.boards)) % len(m.boards)
			}
			return m, nil
		case "enter":
			title := strings.TrimSpace(m.input.Value())
			if title == "" {
				return m, nil
			}
			board := m.boardSlug()
			m.pending = func() tea.Msg {
				_, e := m.service.CreateTriage(m.ctx, board, title, "")
				if e != nil {
					return failureMsg(e)
				}
				return changedMsg{}
			}
			return m, m.pending
		}
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}
func (m CaptureModel) boardSlug() string {
	if m.board >= 0 && m.board < len(m.boards) {
		return m.boards[m.board].Slug
	}
	return ""
}
func (m CaptureModel) View() string {
	if m.done {
		return "Captured."
	}
	err := ""
	if m.err != nil {
		err = "\n" + errorStyle.Render(m.err.Error())
	}
	return fmt.Sprintf("%s\n%s\n%s%s\n", headerStyle.Render("Quick capture → unassigned triage"), dimStyle.Render("board: "+m.boardSlug()+"  tab switches"), m.input.View(), err)
}
