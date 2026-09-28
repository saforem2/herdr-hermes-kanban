# `herdr-hermes-kanban`

A Herdr terminal board and quick-capture popup backed only by the `hermes kanban ...` CLI.

## Requirements

- Herdr `0.9.1` or newer
- Hermes Agent `0.21.5` or newer
- Go `1.24` or newer when building from source
- macOS or Linux

The plugin has no task database. Every read and write runs `hermes kanban ...`; it never opens `kanban.db`. Commands use argv execution with an 8-second timeout and 4 MiB output cap.

## Install

```sh
herdr plugin install saforem2/herdr-hermes-kanban
```

For local development:

```sh
./scripts/build.sh
herdr plugin link /path/to/herdr-hermes-kanban
```

## Open

```sh
herdr plugin action invoke hermes-kanban.open-board
herdr plugin action invoke hermes-kanban.quick-capture
```

The board is a regular Herdr tab. It therefore works in local Herdr, `herdr --remote mbph`, and Heeler terminal attach: the UI process and Hermes CLI run on the Herdr server host.

The capture action opens a modal popup. Enter creates an **unassigned `triage`** task on the selected board. It never assigns or dispatches work. Submissions use a stable idempotency key, so retrying after an ambiguous timeout does not duplicate the card. `Tab` selects another board; `Esc` cancels.

Optional bindings in `~/.config/herdr/config.toml`:

```toml
[[keys.command]]
key = "prefix+k"
type = "plugin_action"
command = "hermes-kanban.open-board"
description = "Open Hermes Kanban"

[[keys.command]]
key = "prefix+n"
type = "plugin_action"
command = "hermes-kanban.quick-capture"
description = "Capture thought to triage"
```

Reload Herdr after editing its configuration:

```sh
herdr server reload-config
```

## Board controls

| Key | Action |
|---|---|
| `h` / `l` | Previous / next status column |
| `j` / `k` | Next / previous card |
| `[` / `]` | Previous / next board |
| `Enter` | Load selected task details and comments |
| `PageUp` / `PageDown` | Scroll long task details |
| `n` | Create an unassigned `triage` task |
| `c` | Add a comment |
| `s` | Enter a safe target status shown by the prompt |
| `r` | Refresh |
| `q` | Quit |

Columns follow Hermes: `triage`, `todo`, `ready`, `running`, `blocked`, `scheduled`, `review`, `done`.

Status changes are mapped to official lifecycle commands: `promote`, `unblock`, `request-review`, and `complete`. The UI offers only source/target pairs accepted by Hermes `0.21.5`. It does not expose `block` or `schedule` because those CLI operations can clear a claim if a stale `ready` card starts running before the command executes. Claimed or `running` tasks have no manual status actions. The UI never invokes `claim`, `assign`, `reassign`, `reclaim`, `archive`, `gc`, or any `--force` operation.

Mutation input is locked while a command is pending. Task content, comments, board names, command errors, and identifiers are stripped of terminal control sequences before rendering. Narrow terminals show a horizontal window centered on the selected status; long columns and details keep the current selection/page visible.

## Development

```sh
go test -race ./...
go vet ./...
./scripts/build.sh
```

Tests run against a fake Hermes executable and assert exact argv, JSON decoding, timeouts, output limits, capture defaults, and transition policy.
