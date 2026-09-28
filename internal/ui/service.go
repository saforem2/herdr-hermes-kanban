package ui

import (
	"context"

	"github.com/saforem2/herdr-hermes-kanban/internal/kanban"
)

type Service interface {
	Boards(context.Context) ([]kanban.Board, error)
	List(context.Context, string) ([]kanban.Task, error)
	Show(context.Context, string, string) (kanban.Detail, error)
	CreateTriage(context.Context, string, string, string) ([]byte, error)
	Comment(context.Context, string, string, string) ([]byte, error)
	Transition(context.Context, string, string, string, string, string) ([]byte, error)
}

var Columns = kanban.StatusColumns

func SafeTransitions(from string) []string {
	switch from {
	case "triage", "todo":
		return []string{"ready", "blocked", "scheduled"}
	case "ready":
		return []string{"blocked", "scheduled", "done"}
	case "running":
		return []string{"blocked", "review", "done"}
	case "blocked", "scheduled":
		return []string{"ready", "done"}
	case "review":
		return []string{"todo", "done"}
	default:
		return nil
	}
}
