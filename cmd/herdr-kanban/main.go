package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/saforem2/herdr-hermes-kanban/internal/kanban"
	"github.com/saforem2/herdr-hermes-kanban/internal/ui"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "herdr-kanban:", err)
		os.Exit(1)
	}
}
func run(args []string) error {
	if len(args) == 0 {
		return usage()
	}
	ctx := context.Background()
	client := kanban.NewClient()
	switch args[0] {
	case "board":
		_, err := tea.NewProgram(ui.NewBoardModel(ctx, client), tea.WithAltScreen()).Run()
		return err
	case "capture":
		_, err := tea.NewProgram(ui.NewCaptureModel(ctx, client)).Run()
		return err
	case "open":
		if len(args) != 2 || (args[1] != "board" && args[1] != "board-overlay" && args[1] != "capture") {
			return usage()
		}
		herdr := os.Getenv("HERDR_BIN_PATH")
		if herdr == "" {
			herdr = "herdr"
		}
		openCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
		defer cancel()
		out, err := ui.OpenPane(openCtx, []string{herdr}, args[1])
		if err != nil {
			return fmt.Errorf("open %s pane: %w: %s", args[1], err, strings.TrimSpace(string(out)))
		}
		return nil
	case "help", "-h", "--help":
		return usageText()
	default:
		return usage()
	}
}
func usage() error {
	return fmt.Errorf("usage: herdr-kanban {board|capture|open board|open board-overlay|open capture}")
}
func usageText() error {
	fmt.Println("usage: herdr-kanban {board|capture|open board|open board-overlay|open capture}")
	return nil
}
