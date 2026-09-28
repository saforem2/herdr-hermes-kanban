#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
mkdir -p bin
go build -trimpath -ldflags='-s -w' -o bin/herdr-kanban.tmp ./cmd/herdr-kanban
mv bin/herdr-kanban.tmp bin/herdr-kanban
