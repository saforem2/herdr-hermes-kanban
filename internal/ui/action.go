package ui

import (
	"context"
	"errors"
	"os"
	"os/exec"
)

func OpenPane(ctx context.Context, command []string, entrypoint string) ([]byte, error) {
	if len(command) == 0 {
		return nil, errors.New("herdr command is empty")
	}
	args := append(append([]string{}, command[1:]...), "plugin", "pane", "open", "--plugin", "hermes-kanban", "--entrypoint", entrypoint, "--focus")
	if entrypoint == "board-overlay" {
		args = append(append([]string{}, command[1:]...), "plugin", "pane", "open", "--plugin", "hermes-kanban", "--entrypoint", entrypoint, "--placement", "overlay", "--focus")
	}
	cmd := exec.CommandContext(ctx, command[0], args...)
	cmd.Env = os.Environ()
	return cmd.CombinedOutput()
}
