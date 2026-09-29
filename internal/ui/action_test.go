package ui

import (
	"context"
	"os"
	"strings"
	"testing"
)

func TestOpenPaneUsesHerdrArgv(t *testing.T) {
	old := os.Getenv("GO_WANT_HERDR_HELPER")
	t.Cleanup(func() { _ = os.Setenv("GO_WANT_HERDR_HELPER", old) })
	_ = os.Setenv("GO_WANT_HERDR_HELPER", "1")
	out, err := OpenPane(context.Background(), []string{os.Args[0], "-test.run=TestHerdrHelper", "--"}, "capture")
	if err != nil {
		t.Fatal(err)
	}
	want := "plugin\x1fpane\x1fopen\x1f--plugin\x1fhermes-kanban\x1f--entrypoint\x1fcapture\x1f--focus"
	if string(out) != want {
		t.Fatalf("got %q", out)
	}
}

func TestOpenOverlayPaneUsesDedicatedEntrypointAndPlacement(t *testing.T) {
	old := os.Getenv("GO_WANT_HERDR_HELPER")
	t.Cleanup(func() { _ = os.Setenv("GO_WANT_HERDR_HELPER", old) })
	_ = os.Setenv("GO_WANT_HERDR_HELPER", "1")
	out, err := OpenPane(context.Background(), []string{os.Args[0], "-test.run=TestHerdrHelper", "--"}, "board-overlay")
	if err != nil {
		t.Fatal(err)
	}
	want := "plugin\x1fpane\x1fopen\x1f--plugin\x1fhermes-kanban\x1f--entrypoint\x1fboard-overlay\x1f--placement\x1foverlay\x1f--focus"
	if string(out) != want {
		t.Fatalf("got %q", out)
	}
}

func TestHerdrHelper(t *testing.T) {
	if os.Getenv("GO_WANT_HERDR_HELPER") != "1" {
		return
	}
	for i, a := range os.Args {
		if a == "--" {
			_, _ = os.Stdout.WriteString(strings.Join(os.Args[i+1:], "\x1f"))
			os.Exit(0)
		}
	}
	os.Exit(2)
}
