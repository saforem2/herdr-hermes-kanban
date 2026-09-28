package ui

import (
	"context"

	"github.com/saforem2/herdr-hermes-kanban/internal/kanban"
)

type Service interface {
	Boards(context.Context) ([]kanban.Board, error)
	List(context.Context, string) ([]kanban.Task, error)
	Show(context.Context, string, string) (kanban.Detail, error)
	CreateTriage(context.Context, string, string, string, string) ([]byte, error)
	Comment(context.Context, string, string, string) ([]byte, error)
	Transition(context.Context, string, string, string, string, string) ([]byte, error)
}

var Columns = kanban.StatusColumns

func SafeTransitions(from string) []string {
	switch from {
	case "todo":
		return []string{"ready"}
	case "ready":
		return []string{"review", "done"}
	case "blocked":
		return []string{"ready", "done"}
	case "scheduled":
		return []string{"ready"}
	case "review":
		return []string{"done"}
	default:
		return nil
	}
}
