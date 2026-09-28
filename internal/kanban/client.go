package kanban

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"
)

var ErrOutputLimit = errors.New("hermes output exceeded limit")
var ErrUnsafeTransition = errors.New("unsafe or unsupported transition")

const defaultTimeout = 8 * time.Second
const defaultMaxOutput = 4 << 20

type CommandError struct {
	ExitCode int
	Stderr   string
}

func (e *CommandError) Error() string {
	return fmt.Sprintf("hermes exited %d: %s", e.ExitCode, strings.TrimSpace(e.Stderr))
}

type Client struct {
	Command   []string
	Timeout   time.Duration
	MaxOutput int64
	Env       []string
}

func NewClient() *Client {
	return &Client{Command: []string{"hermes"}, Timeout: defaultTimeout, MaxOutput: defaultMaxOutput}
}

func (c *Client) run(ctx context.Context, args ...string) ([]byte, error) {
	if len(c.Command) == 0 {
		return nil, errors.New("hermes command is empty")
	}
	timeout := c.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	limit := c.MaxOutput
	if limit <= 0 {
		limit = defaultMaxOutput
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	argv := append(append([]string{}, c.Command[1:]...), args...)
	cmd := exec.CommandContext(ctx, c.Command[0], argv...)
	cmd.Env = append(os.Environ(), c.Env...)
	var stdout, stderr limitedBuffer
	stdout.limit, stderr.limit = limit, limit
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if errors.Is(stdout.err, ErrOutputLimit) || errors.Is(stderr.err, ErrOutputLimit) || int64(stdout.Len()) >= limit || int64(stderr.Len()) >= limit {
		return nil, ErrOutputLimit
	}
	if err != nil {
		code := -1
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			code = exitErr.ExitCode()
		}
		return nil, &CommandError{ExitCode: code, Stderr: stderr.String()}
	}
	return stdout.Bytes(), nil
}

type limitedBuffer struct {
	bytes.Buffer
	limit int64
	err   error
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if b.err != nil {
		return len(p), b.err
	}
	remaining := b.limit - int64(b.Len())
	if remaining <= 0 {
		b.err = ErrOutputLimit
		return len(p), b.err
	}
	if int64(len(p)) > remaining {
		n, _ := b.Buffer.Write(p[:remaining])
		b.err = ErrOutputLimit
		return n, b.err
	}
	return b.Buffer.Write(p)
}

func decode[T any](data []byte, out *T) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(out); err != nil {
		return fmt.Errorf("invalid Hermes JSON: %w", err)
	}
	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("invalid Hermes JSON: trailing value")
		}
		return fmt.Errorf("invalid Hermes JSON: %w", err)
	}
	return nil
}

func boardArgs(board string, rest ...string) []string {
	return append([]string{"kanban", "--board", board}, rest...)
}
func (c *Client) Boards(ctx context.Context) ([]Board, error) {
	out, e := c.run(ctx, "kanban", "boards", "list", "--json")
	if e != nil {
		return nil, e
	}
	var v []Board
	e = decode(out, &v)
	return v, e
}
func (c *Client) List(ctx context.Context, board string) ([]Task, error) {
	out, e := c.run(ctx, boardArgs(board, "list", "--json")...)
	if e != nil {
		return nil, e
	}
	var v []Task
	e = decode(out, &v)
	return v, e
}
func (c *Client) Show(ctx context.Context, board, id string) (Detail, error) {
	out, e := c.run(ctx, boardArgs(board, "show", id, "--json")...)
	if e != nil {
		return Detail{}, e
	}
	var v Detail
	e = decode(out, &v)
	return v, e
}
func (c *Client) CreateTriage(ctx context.Context, board, title, body string) ([]byte, error) {
	args := boardArgs(board, "create", title, "--triage")
	if body != "" {
		args = append(args, "--body", body)
	}
	args = append(args, "--json")
	return c.run(ctx, args...)
}
func (c *Client) Comment(ctx context.Context, board, id, text string) ([]byte, error) {
	return c.run(ctx, boardArgs(board, "comment", id, text, "--author", "herdr-kanban")...)
}

func (c *Client) Transition(ctx context.Context, board, id, from, status, reason string) ([]byte, error) {
	var args []string
	switch status {
	case "ready":
		if from == "blocked" || from == "scheduled" {
			args = boardArgs(board, "unblock", id)
			if reason != "" {
				args = append(args, "--reason", reason)
			}
		} else {
			args = boardArgs(board, "promote", id)
			if reason != "" {
				args = append(args, reason)
			}
			args = append(args, "--json")
		}
	case "blocked":
		args = boardArgs(board, "block", id)
		if reason != "" {
			args = append(args, reason)
		}
	case "scheduled":
		args = boardArgs(board, "schedule", id)
		if reason != "" {
			args = append(args, reason)
		}
	case "review":
		args = boardArgs(board, "request-review", id)
		if reason != "" {
			args = append(args, "--summary", reason)
		}
	case "done":
		args = boardArgs(board, "complete", id)
		if reason != "" {
			args = append(args, "--result", reason)
		}
	case "todo":
		args = boardArgs(board, "reopen-review", id)
		if reason != "" {
			args = append(args, "--reason", reason)
		}
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnsafeTransition, status)
	}
	return c.run(ctx, args...)
}
